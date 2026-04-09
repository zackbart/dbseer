package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/zackbart/dbseer/internal/wire"
)

// Schema holds the full introspected database schema.
type Schema struct {
	Tables []Table
	Enums  []Enum
}

// Table represents a Postgres table, view, or materialized view.
type Table struct {
	Schema            string
	Name              string
	Kind              string // "r"=table, "v"=view, "m"=matview, "p"=partitioned
	Editable          bool
	EditableReason    string // "no_primary_key", "is_view", "is_matview" — empty when editable
	EstimatedRows     int64
	Columns           []Column
	PrimaryKey        []string
	UniqueConstraints [][]string
	ForeignKeys       []ForeignKey
}

// Column represents a single column of a Postgres table.
type Column struct {
	Name        string
	Ordinal     int
	Type        string  // pg_catalog.format_type output (e.g. "integer", "text")
	TypeOID     uint32
	TypeName    string  // pg_type.typname (e.g. "int4", "text", "uuid", "timestamptz")
	Nullable    bool
	Default     *string
	IsIdentity  bool
	IsGenerated bool
	Editor      string  // type hint from wire package
	EnumName    *string // set when TypeName matches a user-defined enum
}

// ForeignKey represents a foreign-key constraint.
type ForeignKey struct {
	Name       string
	Columns    []string
	RefSchema  string
	RefTable   string
	RefColumns []string
	OnDelete   string // "NO ACTION", "RESTRICT", "CASCADE", "SET NULL", "SET DEFAULT"
	OnUpdate   string
}

// Enum represents a user-defined Postgres enum type.
type Enum struct {
	Schema string
	Name   string
	Values []string
}

// EditorForTypeName maps a pg_type.typname to the wire type hint string used
// by the frontend editor. This mirrors the mapping in wire.MarshalWithOID.
func EditorForTypeName(name string) string {
	switch name {
	case "text", "varchar", "bpchar", "name", "citext":
		return string(wire.HintText)
	case "int2", "int4", "int8":
		return string(wire.HintInt)
	case "float4", "float8":
		return string(wire.HintFloat)
	case "numeric":
		return string(wire.HintNumeric)
	case "bool":
		return string(wire.HintBool)
	case "date":
		return string(wire.HintDate)
	case "timestamp":
		return string(wire.HintTimestamp)
	case "timestamptz":
		return string(wire.HintTimestamptz)
	case "time", "timetz":
		return string(wire.HintText)
	case "uuid":
		return string(wire.HintUUID)
	case "jsonb":
		return string(wire.HintJSONB)
	case "json":
		return string(wire.HintJSON)
	case "bytea":
		return string(wire.HintBytea)
	case "interval":
		return string(wire.HintInterval)
	case "tsvector", "tsquery":
		return string(wire.HintTsvector)
	case "xml":
		return string(wire.HintXML)
	case "oid", "regclass", "regtype":
		return string(wire.HintOID)
	case "bit", "varbit":
		return string(wire.HintBit)
	case "inet":
		return string(wire.HintInet)
	case "cidr":
		return string(wire.HintCIDR)
	case "macaddr", "macaddr8":
		return string(wire.HintMacaddr)
	case "int4range", "int8range", "numrange", "tsrange", "tstzrange", "daterange":
		return string(wire.HintRange)
	case "money":
		return string(wire.HintMoney)
	case "geometry", "geography":
		return string(wire.HintGeometry)
	default:
		return string(wire.HintUnknown)
	}
}

// fkActionName maps confdeltype/confupdtype single-char codes to action names.
func fkActionName(code string) string {
	switch code {
	case "a":
		return "NO ACTION"
	case "r":
		return "RESTRICT"
	case "c":
		return "CASCADE"
	case "n":
		return "SET NULL"
	case "d":
		return "SET DEFAULT"
	default:
		return "NO ACTION"
	}
}

// Introspect runs the five introspection queries and assembles a Schema.
func Introspect(ctx context.Context, pool *Pool) (*Schema, error) {
	// --- 1. Enums ---
	enums, enumSet, err := introspectEnums(ctx, pool)
	if err != nil {
		return nil, fmt.Errorf("introspect enums: %w", err)
	}

	// --- 2. Tables ---
	tableRows, err := pool.Query(ctx, `
		SELECT
		  n.nspname AS schema,
		  c.relname AS name,
		  c.reltuples::bigint AS estimated_rows,
		  c.relkind AS kind
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind IN ('r','v','m','p')
		  AND n.nspname NOT IN ('pg_catalog','information_schema','pg_toast')
		  AND n.nspname NOT LIKE 'pg_temp_%'
		ORDER BY n.nspname, c.relname
	`)
	if err != nil {
		return nil, fmt.Errorf("query tables: %w", err)
	}
	defer tableRows.Close()

	type tableKey struct{ schema, name string }
	var tables []Table
	tableIdx := map[tableKey]int{}

	for tableRows.Next() {
		var t Table
		var estRows int64
		if err := tableRows.Scan(&t.Schema, &t.Name, &estRows, &t.Kind); err != nil {
			return nil, fmt.Errorf("scan table row: %w", err)
		}
		t.EstimatedRows = estRows
		tableIdx[tableKey{t.Schema, t.Name}] = len(tables)
		tables = append(tables, t)
	}
	if err := tableRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tables: %w", err)
	}

	// --- 3. Columns (per table) ---
	for i := range tables {
		qualName := tables[i].Schema + "." + tables[i].Name
		cols, err := introspectColumns(ctx, pool, qualName, enumSet)
		if err != nil {
			return nil, fmt.Errorf("introspect columns for %s: %w", qualName, err)
		}
		tables[i].Columns = cols
	}

	// --- 4. Primary Keys (per table) ---
	for i := range tables {
		qualName := tables[i].Schema + "." + tables[i].Name
		pk, err := introspectPrimaryKey(ctx, pool, qualName)
		if err != nil {
			return nil, fmt.Errorf("introspect pk for %s: %w", qualName, err)
		}
		tables[i].PrimaryKey = pk
	}

	// --- 5. Unique Constraints (per table) ---
	for i := range tables {
		qualName := tables[i].Schema + "." + tables[i].Name
		ucs, err := introspectUniqueConstraints(ctx, pool, qualName)
		if err != nil {
			return nil, fmt.Errorf("introspect unique constraints for %s: %w", qualName, err)
		}
		tables[i].UniqueConstraints = ucs
	}

	// --- 6. Foreign Keys (per table) ---
	for i := range tables {
		qualName := tables[i].Schema + "." + tables[i].Name
		fks, err := introspectForeignKeys(ctx, pool, qualName)
		if err != nil {
			return nil, fmt.Errorf("introspect fks for %s: %w", qualName, err)
		}
		tables[i].ForeignKeys = fks
	}

	// --- Classify editability ---
	for i := range tables {
		t := &tables[i]
		switch t.Kind {
		case "v":
			t.Editable = false
			t.EditableReason = "is_view"
		case "m":
			t.Editable = false
			t.EditableReason = "is_matview"
		default:
			if len(t.PrimaryKey) == 0 && len(t.UniqueConstraints) == 0 {
				t.Editable = false
				t.EditableReason = "no_primary_key"
			} else {
				t.Editable = true
			}
		}
	}

	_ = tableIdx // used implicitly above
	return &Schema{Tables: tables, Enums: enums}, nil
}

func introspectEnums(ctx context.Context, pool *Pool) ([]Enum, map[string]bool, error) {
	rows, err := pool.Query(ctx, `
		SELECT
		  n.nspname AS schema,
		  t.typname AS enum_name,
		  array_agg(e.enumlabel ORDER BY e.enumsortorder) AS values
		FROM pg_type t
		JOIN pg_namespace n ON n.oid = t.typnamespace
		JOIN pg_enum e ON e.enumtypid = t.oid
		WHERE t.typtype = 'e'
		GROUP BY n.nspname, t.typname
		ORDER BY n.nspname, t.typname
	`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var enums []Enum
	enumSet := map[string]bool{}

	for rows.Next() {
		var e Enum
		var vals []string
		if err := rows.Scan(&e.Schema, &e.Name, &vals); err != nil {
			return nil, nil, err
		}
		e.Values = vals
		enums = append(enums, e)
		enumSet[e.Name] = true
	}
	return enums, enumSet, rows.Err()
}

func introspectColumns(ctx context.Context, pool *Pool, qualTable string, enumSet map[string]bool) ([]Column, error) {
	rows, err := pool.Query(ctx, `
		SELECT
		  a.attname AS column_name,
		  a.attnum AS ordinal,
		  a.atttypid AS type_oid,
		  pg_catalog.format_type(a.atttypid, a.atttypmod) AS data_type,
		  t.typname AS type_name,
		  NOT a.attnotnull AS is_nullable,
		  pg_get_expr(d.adbin, d.adrelid) AS column_default,
		  a.attidentity IN ('a','d') AS is_identity,
		  a.attgenerated = 's' AS is_generated
		FROM pg_attribute a
		JOIN pg_type t ON t.oid = a.atttypid
		LEFT JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
		WHERE a.attrelid = $1::regclass
		  AND a.attnum > 0
		  AND NOT a.attisdropped
		ORDER BY a.attnum
	`, qualTable)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []Column
	for rows.Next() {
		var c Column
		var typeOID uint32
		if err := rows.Scan(
			&c.Name, &c.Ordinal, &typeOID,
			&c.Type, &c.TypeName,
			&c.Nullable, &c.Default,
			&c.IsIdentity, &c.IsGenerated,
		); err != nil {
			return nil, err
		}
		c.TypeOID = typeOID
		if enumSet[c.TypeName] {
			c.Editor = string(wire.HintEnum)
			name := c.TypeName
			c.EnumName = &name
		} else {
			c.Editor = EditorForTypeName(c.TypeName)
		}
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

func introspectPrimaryKey(ctx context.Context, pool *Pool, qualTable string) ([]string, error) {
	rows, err := pool.Query(ctx, `
		SELECT a.attname AS column_name
		FROM pg_constraint c
		JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = ANY(c.conkey)
		WHERE c.conrelid = $1::regclass AND c.contype = 'p'
		ORDER BY a.attnum
	`, qualTable)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pk []string
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			return nil, err
		}
		pk = append(pk, col)
	}
	return pk, rows.Err()
}

func introspectUniqueConstraints(ctx context.Context, pool *Pool, qualTable string) ([][]string, error) {
	rows, err := pool.Query(ctx, `
		SELECT array_agg(a.attname ORDER BY a.attnum) AS columns
		FROM pg_constraint c
		JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = ANY(c.conkey)
		WHERE c.conrelid = $1::regclass AND c.contype = 'u'
		GROUP BY c.conname
		ORDER BY c.conname
	`, qualTable)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ucs [][]string
	for rows.Next() {
		var cols []string
		if err := rows.Scan(&cols); err != nil {
			return nil, err
		}
		ucs = append(ucs, cols)
	}
	return ucs, rows.Err()
}

func introspectForeignKeys(ctx context.Context, pool *Pool, qualTable string) ([]ForeignKey, error) {
	// Corrected FK query from plan revision B7 — uses WITH ORDINALITY to
	// preserve declared column position in multi-column FKs.
	rows, err := pool.Query(ctx, `
		SELECT
		  c.conname AS constraint_name,
		  c.conrelid::regclass::text AS local_table,
		  (SELECT array_agg(la.attname ORDER BY k.ord)
		     FROM unnest(c.conkey) WITH ORDINALITY AS k(attnum, ord)
		     JOIN pg_attribute la ON la.attrelid = c.conrelid AND la.attnum = k.attnum) AS local_columns,
		  c.confrelid::regclass::text AS foreign_table,
		  (SELECT array_agg(fa.attname ORDER BY k.ord)
		     FROM unnest(c.confkey) WITH ORDINALITY AS k(attnum, ord)
		     JOIN pg_attribute fa ON fa.attrelid = c.confrelid AND fa.attnum = k.attnum) AS foreign_columns,
		  c.confdeltype AS on_delete,
		  c.confupdtype AS on_update
		FROM pg_constraint c
		WHERE c.contype = 'f' AND c.conrelid = $1::regclass
	`, qualTable)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fks []ForeignKey
	for rows.Next() {
		var fk ForeignKey
		var localTable, foreignTable string
		var localCols, foreignCols []string
		var onDelete, onUpdate string
		if err := rows.Scan(
			&fk.Name,
			&localTable, &localCols,
			&foreignTable, &foreignCols,
			&onDelete, &onUpdate,
		); err != nil {
			return nil, err
		}
		fk.Columns = localCols

		// foreignTable is "schema.table" from ::regclass::text — split it.
		parts := strings.SplitN(foreignTable, ".", 2)
		if len(parts) == 2 {
			fk.RefSchema = strings.Trim(parts[0], `"`)
			fk.RefTable = strings.Trim(parts[1], `"`)
		} else {
			fk.RefSchema = "public"
			fk.RefTable = strings.Trim(foreignTable, `"`)
		}
		fk.RefColumns = foreignCols
		fk.OnDelete = fkActionName(onDelete)
		fk.OnUpdate = fkActionName(onUpdate)
		fks = append(fks, fk)
	}
	return fks, rows.Err()
}
