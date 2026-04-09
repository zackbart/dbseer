package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// fkTargetResponse is the JSON shape for GET /api/tables/{schema}/{table}/fk-target.
type fkTargetResponse struct {
	Schema string                        `json:"schema"`
	Table  string                        `json:"table"`
	PK     []string                      `json:"pk"`
	Filter map[string]fkTargetFilterItem `json:"filter"`
}

// fkTargetFilterItem holds a single filter op+val for the FK target.
type fkTargetFilterItem struct {
	Op  string `json:"op"`
	Val string `json:"val"`
}

// handleFKTarget handles GET /api/tables/{schema}/{table}/fk-target.
func (s *Server) handleFKTarget(w http.ResponseWriter, r *http.Request) {
	schema := chi.URLParam(r, "schema")
	table := chi.URLParam(r, "table")
	col := r.URL.Query().Get("col")
	val := r.URL.Query().Get("val")

	if col == "" {
		writeError(w, 400, "invalid_request", "col parameter is required", nil)
		return
	}

	sc, err := s.cfg.Cache.Get(r.Context(), s.cfg.Pool, false)
	if err != nil {
		writeError(w, 500, "db_error", "failed to load schema", map[string]string{"pg_error": err.Error()})
		return
	}

	tableMeta, found := findTable(sc, schema, table)
	if !found {
		writeError(w, 404, "not_found", "table not found", nil)
		return
	}

	// Find the first FK that includes the given column.
	for _, fk := range tableMeta.ForeignKeys {
		for i, fkCol := range fk.Columns {
			if fkCol != col {
				continue
			}
			// Found the FK. Build the response.
			refCol := fk.RefColumns[i]

			// Look up the PK of the referenced table.
			var pk []string
			refMeta, refFound := findTable(sc, fk.RefSchema, fk.RefTable)
			if refFound {
				pk = refMeta.PrimaryKey
			}
			if pk == nil {
				pk = []string{}
			}

			filter := map[string]fkTargetFilterItem{
				refCol: {Op: "eq", Val: val},
			}

			writeJSON(w, 200, fkTargetResponse{
				Schema: fk.RefSchema,
				Table:  fk.RefTable,
				PK:     pk,
				Filter: filter,
			})
			return
		}
	}

	writeError(w, 404, "not_found", "no foreign key found for column", nil)
}
