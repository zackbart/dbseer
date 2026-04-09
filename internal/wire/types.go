// Package wire provides marshalling from pgx native Go types to the JSON wire
// envelope {"v": <json_value>, "t": "<type_hint>"} used by the dbseer API.
package wire

// TypeHint is a sentinel string type that identifies the Postgres data type of
// a wire cell, allowing the browser to render the appropriate editor widget.
type TypeHint string

// Type hint constants corresponding to Postgres type families.
const (
	HintText        TypeHint = "text"
	HintInt         TypeHint = "int"
	HintFloat       TypeHint = "float"
	HintNumeric     TypeHint = "numeric"
	HintBool        TypeHint = "bool"
	HintDate        TypeHint = "date"
	HintTimestamp   TypeHint = "timestamp"
	HintTimestamptz TypeHint = "timestamptz"
	HintUUID        TypeHint = "uuid"
	HintJSONB       TypeHint = "jsonb"
	HintJSON        TypeHint = "json"
	HintBytea       TypeHint = "bytea"
	HintInterval    TypeHint = "interval"
	HintEnum        TypeHint = "enum"
	HintArray       TypeHint = "array"
	HintTsvector    TypeHint = "tsvector"
	HintXML         TypeHint = "xml"
	HintOID         TypeHint = "oid"
	HintBit         TypeHint = "bit"
	HintInet        TypeHint = "inet"
	HintCIDR        TypeHint = "cidr"
	HintMacaddr     TypeHint = "macaddr"
	HintRange       TypeHint = "range"
	HintMoney       TypeHint = "money"
	HintGeometry    TypeHint = "geometry"
	HintUnknown     TypeHint = "unknown"
)
