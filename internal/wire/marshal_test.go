package wire

import (
	"encoding/json"
	"math/big"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// cellJSON marshals a Cell to JSON bytes for comparison.
func cellJSON(t *testing.T, c Cell) string {
	t.Helper()
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("json.Marshal(Cell): %v", err)
	}
	return string(b)
}

func TestMarshal_Nil(t *testing.T) {
	c := Marshal(nil)
	got := cellJSON(t, c)
	want := `{"v":null,"t":""}`
	if got != want {
		t.Errorf("nil: got %s, want %s", got, want)
	}
}

func TestMarshal_String(t *testing.T) {
	c := Marshal("hello")
	got := cellJSON(t, c)
	want := `{"v":"hello","t":"text"}`
	if got != want {
		t.Errorf("string: got %s, want %s", got, want)
	}
}

func TestMarshal_Int64(t *testing.T) {
	c := Marshal(int64(42))
	got := cellJSON(t, c)
	want := `{"v":42,"t":"int"}`
	if got != want {
		t.Errorf("int64: got %s, want %s", got, want)
	}
}

func TestMarshal_Float64(t *testing.T) {
	c := Marshal(float64(3.14))
	got := cellJSON(t, c)
	want := `{"v":3.14,"t":"float"}`
	if got != want {
		t.Errorf("float64: got %s, want %s", got, want)
	}
}

func TestMarshal_Bool(t *testing.T) {
	c := Marshal(true)
	got := cellJSON(t, c)
	want := `{"v":true,"t":"bool"}`
	if got != want {
		t.Errorf("bool: got %s, want %s", got, want)
	}
}

func TestMarshal_TimeRoundtrip(t *testing.T) {
	ts := time.Date(2024, 3, 15, 10, 30, 45, 123456789, time.UTC)
	c := Marshal(ts)
	if c.T != HintTimestamptz {
		t.Errorf("time.Time: got hint %q, want %q", c.T, HintTimestamptz)
	}
	var s string
	if err := json.Unmarshal(c.V, &s); err != nil {
		t.Fatalf("unmarshal time value: %v", err)
	}
	parsed, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t.Fatalf("parse RFC3339Nano: %v", err)
	}
	if !parsed.Equal(ts) {
		t.Errorf("time round-trip: got %v, want %v", parsed, ts)
	}
}

func TestMarshal_NumericPrecision(t *testing.T) {
	// Build pgtype.Numeric for "123.4567890123456789"
	// = 1234567890123456789 * 10^-16
	i := new(big.Int)
	i.SetString("1234567890123456789", 10)
	n := pgtype.Numeric{Int: i, Exp: -16, Valid: true}

	c := Marshal(n)
	if c.T != HintNumeric {
		t.Errorf("numeric hint: got %q, want %q", c.T, HintNumeric)
	}
	var s string
	if err := json.Unmarshal(c.V, &s); err != nil {
		t.Fatalf("unmarshal numeric: %v", err)
	}
	want := "123.4567890123456789"
	if s != want {
		t.Errorf("numeric precision: got %q, want %q", s, want)
	}
}

func TestMarshal_Interval(t *testing.T) {
	// 14 months = 1Y2M, 3 days, 3723 seconds = 1H2M3S
	iv := pgtype.Interval{
		Months:       14,
		Days:         3,
		Microseconds: 3723 * 1_000_000,
		Valid:        true,
	}
	c := Marshal(iv)
	if c.T != HintInterval {
		t.Errorf("interval hint: got %q, want %q", c.T, HintInterval)
	}
	var s string
	if err := json.Unmarshal(c.V, &s); err != nil {
		t.Fatalf("unmarshal interval: %v", err)
	}
	want := "P1Y2M3DT1H2M3S"
	if s != want {
		t.Errorf("interval: got %q, want %q", s, want)
	}
}

func TestMarshal_UUID16Byte(t *testing.T) {
	// A known UUID in byte form.
	var b [16]byte
	// 550e8400-e29b-41d4-a716-446655440000
	b[0] = 0x55
	b[1] = 0x0e
	b[2] = 0x84
	b[3] = 0x00
	b[4] = 0xe2
	b[5] = 0x9b
	b[6] = 0x41
	b[7] = 0xd4
	b[8] = 0xa7
	b[9] = 0x16
	b[10] = 0x44
	b[11] = 0x66
	b[12] = 0x55
	b[13] = 0x44
	b[14] = 0x00
	b[15] = 0x00

	c := Marshal(b)
	if c.T != HintUUID {
		t.Errorf("uuid hint: got %q, want %q", c.T, HintUUID)
	}
	var s string
	if err := json.Unmarshal(c.V, &s); err != nil {
		t.Fatalf("unmarshal uuid: %v", err)
	}
	want := "550e8400-e29b-41d4-a716-446655440000"
	if s != want {
		t.Errorf("uuid: got %q, want %q", s, want)
	}
}

func TestMarshal_Bytea(t *testing.T) {
	data := []byte{0xde, 0xad, 0xbe, 0xef}
	c := Marshal(data)
	if c.T != HintBytea {
		t.Errorf("bytea hint: got %q, want %q", c.T, HintBytea)
	}
	var s string
	if err := json.Unmarshal(c.V, &s); err != nil {
		t.Fatalf("unmarshal bytea: %v", err)
	}
	want := `\xdeadbeef`
	if s != want {
		t.Errorf("bytea: got %q, want %q", s, want)
	}
}

func TestMarshal_JSONBMap(t *testing.T) {
	m := map[string]any{"foo": "bar", "n": float64(42)}
	c := Marshal(m)
	if c.T != HintJSONB {
		t.Errorf("jsonb hint: got %q, want %q", c.T, HintJSONB)
	}
	// Re-decode and check key presence.
	var out map[string]any
	if err := json.Unmarshal(c.V, &out); err != nil {
		t.Fatalf("unmarshal jsonb map: %v", err)
	}
	if out["foo"] != "bar" {
		t.Errorf("jsonb key foo: got %v", out["foo"])
	}
}

func TestMarshal_NestedArray(t *testing.T) {
	arr := []any{int64(1), "two", nil}
	c := Marshal(arr)
	if c.T != HintArray {
		t.Errorf("array hint: got %q, want %q", c.T, HintArray)
	}
	var cells []Cell
	if err := json.Unmarshal(c.V, &cells); err != nil {
		t.Fatalf("unmarshal array cells: %v", err)
	}
	if len(cells) != 3 {
		t.Fatalf("array length: got %d, want 3", len(cells))
	}
	if cells[0].T != HintInt {
		t.Errorf("array[0] hint: got %q, want %q", cells[0].T, HintInt)
	}
	if cells[1].T != HintText {
		t.Errorf("array[1] hint: got %q, want %q", cells[1].T, HintText)
	}
	if cells[2].T != TypeHint("") {
		t.Errorf("array[2] hint (nil): got %q, want empty", cells[2].T)
	}
}

type unknownStruct struct {
	X int
	Y string
}

func TestMarshal_UnknownStruct(t *testing.T) {
	c := Marshal(unknownStruct{X: 1, Y: "z"})
	if c.T != HintUnknown {
		t.Errorf("unknown struct hint: got %q, want %q", c.T, HintUnknown)
	}
	// Value should be non-null JSON.
	if string(c.V) == "null" {
		t.Errorf("unknown struct value should not be null")
	}
}

func TestMarshalWithOID_Int4(t *testing.T) {
	c := MarshalWithOID(int64(42), "int4")
	if c.T != HintInt {
		t.Errorf("int4 oid hint: got %q, want %q", c.T, HintInt)
	}
	var n int64
	if err := json.Unmarshal(c.V, &n); err != nil {
		t.Fatalf("unmarshal int4: %v", err)
	}
	if n != 42 {
		t.Errorf("int4 value: got %d, want 42", n)
	}
}

func TestMarshalWithOID_Tsvector(t *testing.T) {
	c := MarshalWithOID("'foo':1 'bar':2", "tsvector")
	if c.T != HintTsvector {
		t.Errorf("tsvector hint: got %q, want %q", c.T, HintTsvector)
	}
}

func TestMarshalEnum(t *testing.T) {
	c := MarshalEnum("admin")
	if c.T != HintEnum {
		t.Errorf("enum hint: got %q, want %q", c.T, HintEnum)
	}
	var s string
	if err := json.Unmarshal(c.V, &s); err != nil {
		t.Fatalf("unmarshal enum: %v", err)
	}
	if s != "admin" {
		t.Errorf("enum value: got %q, want %q", s, "admin")
	}
}

func TestMarshalWithOID_ArrayPrefix(t *testing.T) {
	arr := []any{int64(1), int64(2), int64(3)}
	c := MarshalWithOID(arr, "_int4")
	if c.T != HintArray {
		t.Errorf("_int4 array hint: got %q, want %q", c.T, HintArray)
	}
	var cells []Cell
	if err := json.Unmarshal(c.V, &cells); err != nil {
		t.Fatalf("unmarshal _int4 cells: %v", err)
	}
	for i, cell := range cells {
		if cell.T != HintInt {
			t.Errorf("_int4 cells[%d] hint: got %q, want %q", i, cell.T, HintInt)
		}
	}
}

func TestMarshal_Int16(t *testing.T) {
	c := Marshal(int16(7))
	if c.T != HintInt {
		t.Errorf("int16 hint: got %q", c.T)
	}
}

func TestMarshal_Int32(t *testing.T) {
	c := Marshal(int32(100))
	if c.T != HintInt {
		t.Errorf("int32 hint: got %q", c.T)
	}
}

func TestMarshal_Uint32(t *testing.T) {
	c := Marshal(uint32(99))
	if c.T != HintInt {
		t.Errorf("uint32 hint: got %q", c.T)
	}
}

func TestMarshal_Float32(t *testing.T) {
	c := Marshal(float32(1.5))
	if c.T != HintFloat {
		t.Errorf("float32 hint: got %q", c.T)
	}
}

func TestMarshal_PgtypeUUID(t *testing.T) {
	var b [16]byte
	b[0] = 0x55
	b[15] = 0xFF
	u := pgtype.UUID{Bytes: b, Valid: true}
	c := Marshal(u)
	if c.T != HintUUID {
		t.Errorf("pgtype.UUID hint: got %q", c.T)
	}
	var s string
	if err := json.Unmarshal(c.V, &s); err != nil {
		t.Fatalf("unmarshal pgtype.UUID: %v", err)
	}
	if len(s) != 36 {
		t.Errorf("uuid string length: got %d", len(s))
	}
}

func TestMarshalWithOID_Money(t *testing.T) {
	c := MarshalWithOID("$1,234.56", "money")
	if c.T != HintMoney {
		t.Errorf("money hint: got %q, want %q", c.T, HintMoney)
	}
	var s string
	if err := json.Unmarshal(c.V, &s); err != nil {
		t.Fatalf("unmarshal money: %v", err)
	}
	if s != "$1,234.56" {
		t.Errorf("money value: got %q", s)
	}
}

func TestMarshalWithOID_Date(t *testing.T) {
	// time.Time from pgx for date columns.
	ts := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	c := MarshalWithOID(ts, "date")
	if c.T != HintDate {
		t.Errorf("date hint: got %q, want %q", c.T, HintDate)
	}
}

func TestMarshalWithOID_Inet(t *testing.T) {
	c := MarshalWithOID("192.168.1.1/32", "inet")
	if c.T != HintInet {
		t.Errorf("inet hint: got %q, want %q", c.T, HintInet)
	}
}
