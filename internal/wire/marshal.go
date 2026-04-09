package wire

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Cell is the JSON wire envelope for a single database cell value.
// V holds the raw JSON value and T holds the type hint for the browser editor.
type Cell struct {
	V json.RawMessage `json:"v"`
	T TypeHint        `json:"t"`
}

// Marshal converts a value returned by pgx rows.Values() into a wire Cell.
// The concrete Go type determines both the JSON encoding and the type hint.
// nil values produce {"v":null,"t":""}.
func Marshal(v any) Cell {
	if v == nil {
		return Cell{V: json.RawMessage("null"), T: ""}
	}

	switch val := v.(type) {
	case string:
		b, _ := json.Marshal(val)
		return Cell{V: b, T: HintText}

	case int16:
		b, _ := json.Marshal(val)
		return Cell{V: b, T: HintInt}

	case int32:
		b, _ := json.Marshal(val)
		return Cell{V: b, T: HintInt}

	case int64:
		b, _ := json.Marshal(val)
		return Cell{V: b, T: HintInt}

	case uint32:
		b, _ := json.Marshal(val)
		return Cell{V: b, T: HintInt}

	case float32:
		b, _ := json.Marshal(val)
		return Cell{V: b, T: HintFloat}

	case float64:
		b, _ := json.Marshal(val)
		return Cell{V: b, T: HintFloat}

	case bool:
		b, _ := json.Marshal(val)
		return Cell{V: b, T: HintBool}

	case time.Time:
		b, _ := json.Marshal(val.Format(time.RFC3339Nano))
		return Cell{V: b, T: HintTimestamptz}

	case pgtype.Numeric:
		s := marshalNumeric(val)
		b, _ := json.Marshal(s)
		return Cell{V: b, T: HintNumeric}

	case pgtype.Interval:
		s := marshalInterval(val)
		b, _ := json.Marshal(s)
		return Cell{V: b, T: HintInterval}

	case [16]byte:
		u := uuid.UUID(val)
		b, _ := json.Marshal(u.String())
		return Cell{V: b, T: HintUUID}

	case pgtype.UUID:
		u := uuid.UUID(val.Bytes)
		b, _ := json.Marshal(u.String())
		return Cell{V: b, T: HintUUID}

	case []byte:
		s := `\x` + hex.EncodeToString(val)
		b, _ := json.Marshal(s)
		return Cell{V: b, T: HintBytea}

	case map[string]any:
		b, err := json.Marshal(val)
		if err != nil {
			b = []byte("null")
		}
		return Cell{V: b, T: HintJSONB}

	case []any:
		cells := make([]Cell, len(val))
		for i, elem := range val {
			cells[i] = Marshal(elem)
		}
		b, err := json.Marshal(cells)
		if err != nil {
			b = []byte("null")
		}
		return Cell{V: b, T: HintArray}

	default:
		// Generic range handling via pgtype range types.
		if rc := tryMarshalRange(val); rc != nil {
			return *rc
		}
		// Fallback: attempt json.Marshal; if it fails, stringify.
		b, err := json.Marshal(val)
		if err != nil {
			b, _ = json.Marshal(fmt.Sprintf("%v", val))
		}
		return Cell{V: b, T: HintUnknown}
	}
}

// MarshalWithOID is like Marshal but uses the Postgres type name (from
// pg_type.typname at introspection time) to override the type hint when we
// know more than the Go type alone reveals.
//
// oidName should be the raw typname value, e.g. "int4", "uuid", "_text".
// Array types have a leading underscore; MarshalWithOID strips it and
// recurses element-by-element, returning hint "array".
func MarshalWithOID(v any, oidName string) Cell {
	// Array types: leading underscore prefix in pg_type.typname.
	if strings.HasPrefix(oidName, "_") {
		baseType := oidName[1:]
		// Marshal as array, applying the base type hint to each element.
		switch val := v.(type) {
		case []any:
			cells := make([]Cell, len(val))
			for i, elem := range val {
				cells[i] = MarshalWithOID(elem, baseType)
			}
			b, err := json.Marshal(cells)
			if err != nil {
				b = []byte("null")
			}
			return Cell{V: b, T: HintArray}
		default:
			// Not actually a slice — fall through to base marshal.
		}
	}

	cell := Marshal(v)
	hint := hintFromOIDName(oidName)
	if hint != "" {
		cell.T = hint
	}

	// money must be a string, never a float.
	if hint == HintMoney && v != nil {
		if _, ok := v.(string); !ok {
			b, _ := json.Marshal(fmt.Sprintf("%v", v))
			cell.V = b
		}
	}

	// geometry: encode raw bytes as \xWKB hex.
	if hint == HintGeometry {
		switch val := v.(type) {
		case []byte:
			s := `\x` + hex.EncodeToString(val)
			b, _ := json.Marshal(s)
			cell.V = b
		default:
			b, _ := json.Marshal(fmt.Sprintf("%v", v))
			cell.V = b
		}
	}

	return cell
}

// MarshalEnum marshals a value with type hint "enum". It delegates to Marshal
// for the value encoding but overrides the hint.
func MarshalEnum(v any) Cell {
	c := Marshal(v)
	c.T = HintEnum
	return c
}

// hintFromOIDName maps a Postgres typname to a TypeHint.
// Returns "" if the typname is not recognized (caller keeps Marshal's default).
func hintFromOIDName(typname string) TypeHint {
	switch typname {
	case "text", "varchar", "bpchar", "name", "citext":
		return HintText
	case "int2":
		return HintInt
	case "int4":
		return HintInt
	case "int8":
		return HintInt
	case "float4", "float8":
		return HintFloat
	case "numeric":
		return HintNumeric
	case "bool":
		return HintBool
	case "date":
		return HintDate
	case "timestamp":
		return HintTimestamp
	case "timestamptz":
		return HintTimestamptz
	case "time", "timetz":
		return HintText
	case "uuid":
		return HintUUID
	case "jsonb":
		return HintJSONB
	case "json":
		return HintJSON
	case "bytea":
		return HintBytea
	case "interval":
		return HintInterval
	case "tsvector", "tsquery":
		return HintTsvector
	case "xml":
		return HintXML
	case "oid", "regclass", "regtype":
		return HintOID
	case "bit", "varbit":
		return HintBit
	case "inet":
		return HintInet
	case "cidr":
		return HintCIDR
	case "macaddr", "macaddr8":
		return HintMacaddr
	case "int4range", "int8range", "numrange", "tsrange", "tstzrange", "daterange":
		return HintRange
	case "money":
		return HintMoney
	case "geometry", "geography":
		return HintGeometry
	default:
		return ""
	}
}

// marshalNumeric serializes a pgtype.Numeric as a decimal string to avoid
// any floating-point precision loss.
func marshalNumeric(n pgtype.Numeric) string {
	if !n.Valid {
		return "null"
	}
	// Use the big.Int representation: value = Int * 10^Exp
	if n.Int == nil {
		return "0"
	}
	intStr := n.Int.String()
	exp := int(n.Exp)

	if exp == 0 {
		return intStr
	}

	negative := false
	digits := intStr
	if strings.HasPrefix(digits, "-") {
		negative = true
		digits = digits[1:]
	}

	if exp > 0 {
		// Multiply: append zeros.
		return intStr + strings.Repeat("0", exp)
	}

	// exp < 0: insert decimal point.
	absExp := -exp
	if absExp >= len(digits) {
		// Need leading zeros after decimal point.
		frac := strings.Repeat("0", absExp-len(digits)) + digits
		result := "0." + frac
		if negative {
			result = "-" + result
		}
		return result
	}
	insertAt := len(digits) - absExp
	result := digits[:insertAt] + "." + digits[insertAt:]
	if negative {
		result = "-" + result
	}
	return result
}

// marshalInterval serializes a pgtype.Interval as an ISO 8601 duration string,
// e.g. "P1Y2M3DT1H2M3S".
func marshalInterval(iv pgtype.Interval) string {
	if !iv.Valid {
		return ""
	}

	months := iv.Months
	years := months / 12
	months = months % 12
	days := iv.Days

	totalMicros := iv.Microseconds
	negative := totalMicros < 0
	if negative {
		totalMicros = -totalMicros
	}
	hours := totalMicros / (3600 * 1_000_000)
	totalMicros -= hours * 3600 * 1_000_000
	minutes := totalMicros / (60 * 1_000_000)
	totalMicros -= minutes * 60 * 1_000_000
	seconds := totalMicros / 1_000_000
	micros := totalMicros % 1_000_000

	if negative {
		hours = -hours
		minutes = -minutes
		seconds = -seconds
		micros = -micros
	}

	var sb strings.Builder
	sb.WriteString("P")
	if years != 0 {
		fmt.Fprintf(&sb, "%dY", years)
	}
	if months != 0 {
		fmt.Fprintf(&sb, "%dM", months)
	}
	if days != 0 {
		fmt.Fprintf(&sb, "%dD", days)
	}
	// Time component.
	if hours != 0 || minutes != 0 || seconds != 0 || micros != 0 {
		sb.WriteString("T")
		if hours != 0 {
			fmt.Fprintf(&sb, "%dH", hours)
		}
		if minutes != 0 {
			fmt.Fprintf(&sb, "%dM", minutes)
		}
		if seconds != 0 || micros != 0 {
			if micros == 0 {
				fmt.Fprintf(&sb, "%dS", seconds)
			} else {
				// Format fractional seconds.
				fracStr := strings.TrimRight(fmt.Sprintf("%06d", abs64(micros)), "0")
				fmt.Fprintf(&sb, "%d.%sS", seconds, fracStr)
			}
		}
	}

	result := sb.String()
	if result == "P" {
		result = "P0D"
	}
	return result
}

func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

// rangeShape is the JSON representation of a Postgres range value.
type rangeShape struct {
	Lower    json.RawMessage `json:"lower"`
	Upper    json.RawMessage `json:"upper"`
	LowerInc bool            `json:"lower_inc"`
	UpperInc bool            `json:"upper_inc"`
	Empty    bool            `json:"empty"`
}

// tryMarshalRange attempts to handle the known pgtype range types.
// Returns nil if v is not a recognized range type.
func tryMarshalRange(v any) *Cell {
	var shape *rangeShape

	switch r := v.(type) {
	case pgtype.Range[pgtype.Int4]:
		shape = marshalTypedRange(
			r.LowerType, r.UpperType, r.Valid,
			func() json.RawMessage {
				if r.Lower.Valid {
					b, _ := json.Marshal(r.Lower.Int32)
					return b
				}
				return json.RawMessage("null")
			},
			func() json.RawMessage {
				if r.Upper.Valid {
					b, _ := json.Marshal(r.Upper.Int32)
					return b
				}
				return json.RawMessage("null")
			},
		)

	case pgtype.Range[pgtype.Int8]:
		shape = marshalTypedRange(
			r.LowerType, r.UpperType, r.Valid,
			func() json.RawMessage {
				if r.Lower.Valid {
					b, _ := json.Marshal(r.Lower.Int64)
					return b
				}
				return json.RawMessage("null")
			},
			func() json.RawMessage {
				if r.Upper.Valid {
					b, _ := json.Marshal(r.Upper.Int64)
					return b
				}
				return json.RawMessage("null")
			},
		)

	case pgtype.Range[pgtype.Numeric]:
		shape = marshalTypedRange(
			r.LowerType, r.UpperType, r.Valid,
			func() json.RawMessage {
				if r.Lower.Valid {
					b, _ := json.Marshal(marshalNumeric(r.Lower))
					return b
				}
				return json.RawMessage("null")
			},
			func() json.RawMessage {
				if r.Upper.Valid {
					b, _ := json.Marshal(marshalNumeric(r.Upper))
					return b
				}
				return json.RawMessage("null")
			},
		)

	case pgtype.Range[pgtype.Timestamp]:
		shape = marshalTypedRange(
			r.LowerType, r.UpperType, r.Valid,
			func() json.RawMessage {
				if r.Lower.Valid {
					b, _ := json.Marshal(r.Lower.Time.Format(time.RFC3339Nano))
					return b
				}
				return json.RawMessage("null")
			},
			func() json.RawMessage {
				if r.Upper.Valid {
					b, _ := json.Marshal(r.Upper.Time.Format(time.RFC3339Nano))
					return b
				}
				return json.RawMessage("null")
			},
		)

	case pgtype.Range[pgtype.Timestamptz]:
		shape = marshalTypedRange(
			r.LowerType, r.UpperType, r.Valid,
			func() json.RawMessage {
				if r.Lower.Valid {
					b, _ := json.Marshal(r.Lower.Time.Format(time.RFC3339Nano))
					return b
				}
				return json.RawMessage("null")
			},
			func() json.RawMessage {
				if r.Upper.Valid {
					b, _ := json.Marshal(r.Upper.Time.Format(time.RFC3339Nano))
					return b
				}
				return json.RawMessage("null")
			},
		)

	case pgtype.Range[pgtype.Date]:
		shape = marshalTypedRange(
			r.LowerType, r.UpperType, r.Valid,
			func() json.RawMessage {
				if r.Lower.Valid {
					b, _ := json.Marshal(r.Lower.Time.Format("2006-01-02"))
					return b
				}
				return json.RawMessage("null")
			},
			func() json.RawMessage {
				if r.Upper.Valid {
					b, _ := json.Marshal(r.Upper.Time.Format("2006-01-02"))
					return b
				}
				return json.RawMessage("null")
			},
		)

	default:
		return nil
	}

	b, err := json.Marshal(shape)
	if err != nil {
		b = []byte("null")
	}
	return &Cell{V: b, T: HintRange}
}

// marshalTypedRange builds a rangeShape from type flags and lazy value getters.
func marshalTypedRange(
	lowerType, upperType pgtype.BoundType,
	valid bool,
	lowerFn, upperFn func() json.RawMessage,
) *rangeShape {
	if !valid {
		return &rangeShape{
			Lower:    json.RawMessage("null"),
			Upper:    json.RawMessage("null"),
			LowerInc: false,
			UpperInc: false,
			Empty:    true,
		}
	}

	empty := lowerType == pgtype.Empty || upperType == pgtype.Empty
	if empty {
		return &rangeShape{
			Lower:    json.RawMessage("null"),
			Upper:    json.RawMessage("null"),
			LowerInc: false,
			UpperInc: false,
			Empty:    true,
		}
	}

	lower := lowerFn()
	upper := upperFn()

	if lowerType == pgtype.Unbounded {
		lower = json.RawMessage("null")
	}
	if upperType == pgtype.Unbounded {
		upper = json.RawMessage("null")
	}

	return &rangeShape{
		Lower:    lower,
		Upper:    upper,
		LowerInc: lowerType == pgtype.Inclusive,
		UpperInc: upperType == pgtype.Inclusive,
		Empty:    false,
	}
}
