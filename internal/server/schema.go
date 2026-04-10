package server

import (
	"net/http"

	"github.com/zackbart/dbseer/internal/db"
)

// schemaTableJSON is the JSON shape for a table in GET /api/schema.
type schemaTableJSON struct {
	Schema            string              `json:"schema"`
	Name              string              `json:"name"`
	Kind              string              `json:"kind"`
	Editable          bool                `json:"editable"`
	EditableReason    *string             `json:"editable_reason"`
	EstimatedRows     int64               `json:"estimated_rows"`
	Columns           []schemaColumnJSON  `json:"columns"`
	PrimaryKey        []string            `json:"primary_key"`
	UniqueConstraints [][]string          `json:"unique_constraints"`
	ForeignKeys       []schemaFKJSON      `json:"foreign_keys"`
}

// schemaColumnJSON is the JSON shape for a column in the schema response.
type schemaColumnJSON struct {
	Name        string  `json:"name"`
	Ordinal     int     `json:"ordinal"`
	Type        string  `json:"type"`
	Nullable    bool    `json:"nullable"`
	Default     *string `json:"default"`
	IsIdentity  bool    `json:"is_identity"`
	IsGenerated bool    `json:"is_generated"`
	Editor      string  `json:"editor"`
	EnumName    *string `json:"enum_name"`
}

// schemaFKJSON is the JSON shape for a foreign key in the schema response.
type schemaFKJSON struct {
	Name       string          `json:"name"`
	Columns    []string        `json:"columns"`
	References schemaFKRefJSON `json:"references"`
	OnDelete   string          `json:"on_delete"`
	OnUpdate   string          `json:"on_update"`
}

// schemaFKRefJSON is the JSON shape for a foreign key reference.
type schemaFKRefJSON struct {
	Schema  string   `json:"schema"`
	Table   string   `json:"table"`
	Columns []string `json:"columns"`
}

// schemaEnumJSON is the JSON shape for an enum in the schema response.
type schemaEnumJSON struct {
	Schema string   `json:"schema"`
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

// schemaResponse is the full JSON shape for GET /api/schema.
type schemaResponse struct {
	Tables []schemaTableJSON `json:"tables"`
	Enums  []schemaEnumJSON  `json:"enums"`
}

// handleSchema handles GET /api/schema.
func (s *Server) handleSchema(w http.ResponseWriter, r *http.Request) {
	refresh := r.URL.Query().Get("refresh") == "1"

	schema, err := s.cfg.Cache.Get(r.Context(), s.cfg.Pool, refresh)
	if err != nil {
		s.cfg.Logger.Error("schema introspection failed", "err", err)
		writeError(w, 500, "db_error", "failed to introspect schema", map[string]string{"pg_error": err.Error()})
		return
	}

	writeJSON(w, 200, buildSchemaResponse(schema))
}

// buildSchemaResponse converts a db.Schema into the JSON wire shape.
func buildSchemaResponse(schema *db.Schema) schemaResponse {
	tables := make([]schemaTableJSON, len(schema.Tables))
	for i, t := range schema.Tables {
		tables[i] = buildTableJSON(t)
	}

	enums := make([]schemaEnumJSON, len(schema.Enums))
	for i, e := range schema.Enums {
		enums[i] = schemaEnumJSON{
			Schema: e.Schema,
			Name:   e.Name,
			Values: e.Values,
		}
	}

	return schemaResponse{Tables: tables, Enums: enums}
}

// buildTableJSON converts a db.Table into the JSON wire shape.
func buildTableJSON(t db.Table) schemaTableJSON {
	cols := make([]schemaColumnJSON, len(t.Columns))
	for i, c := range t.Columns {
		cols[i] = schemaColumnJSON{
			Name:        c.Name,
			Ordinal:     c.Ordinal,
			Type:        c.TypeName,
			Nullable:    c.Nullable,
			Default:     c.Default,
			IsIdentity:  c.IsIdentity,
			IsGenerated: c.IsGenerated,
			Editor:      c.Editor,
			EnumName:    c.EnumName,
		}
	}

	fks := make([]schemaFKJSON, len(t.ForeignKeys))
	for i, fk := range t.ForeignKeys {
		fks[i] = schemaFKJSON{
			Name:    fk.Name,
			Columns: fk.Columns,
			References: schemaFKRefJSON{
				Schema:  fk.RefSchema,
				Table:   fk.RefTable,
				Columns: fk.RefColumns,
			},
			OnDelete: fk.OnDelete,
			OnUpdate: fk.OnUpdate,
		}
	}

	var editableReason *string
	if !t.Editable && t.EditableReason != "" {
		r := t.EditableReason
		editableReason = &r
	}

	pk := t.PrimaryKey
	if pk == nil {
		pk = []string{}
	}
	uc := t.UniqueConstraints
	if uc == nil {
		uc = [][]string{}
	}

	return schemaTableJSON{
		Schema:            t.Schema,
		Name:              t.Name,
		Kind:              t.Kind,
		Editable:          t.Editable,
		EditableReason:    editableReason,
		EstimatedRows:     t.EstimatedRows,
		Columns:           cols,
		PrimaryKey:        pk,
		UniqueConstraints: uc,
		ForeignKeys:       fks,
	}
}
