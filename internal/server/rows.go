package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/zackbart/dbseer/internal/db"
	"github.com/zackbart/dbseer/internal/safety"
	"github.com/zackbart/dbseer/internal/wire"
)

// browseResponseJSON is the JSON shape for GET /api/tables/{schema}/{table}/rows.
type browseResponseJSON struct {
	Columns []browseColumnJSON `json:"columns"`
	Rows    [][]wire.Cell      `json:"rows"`
	Page    browsePageJSON     `json:"page"`
	Sort    []browseSortJSON   `json:"sort"`
	Filters []browseFilterJSON `json:"filters"`
}

type browseColumnJSON struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Editor string `json:"editor"`
}

type browsePageJSON struct {
	Limit       int   `json:"limit"`
	Offset      int   `json:"offset"`
	Total       int64 `json:"total"`
	IsEstimated bool  `json:"is_estimated"`
}

type browseSortJSON struct {
	Column string `json:"column"`
	Dir    string `json:"dir"`
}

type browseFilterJSON struct {
	Column string `json:"column"`
	Op     string `json:"op"`
	Val    string `json:"val"`
}

// handleBrowse handles GET /api/tables/{schema}/{table}/rows.
func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	schema := chi.URLParam(r, "schema")
	table := chi.URLParam(r, "table")

	limit, offset, err := parsePagination(r.URL.Query())
	if err != nil {
		writeError(w, 400, "invalid_request", err.Error(), nil)
		return
	}

	sorts := parseSortOrder(r.URL.RawQuery)
	filters := parseFilters(r.URL.Query())

	// Fetch table metadata from cache.
	sc, err := s.cfg.Cache.Get(r.Context(), s.cfg.Pool, false)
	if err != nil {
		writeError(w, 500, "db_error", "failed to load schema", map[string]string{"pg_error": err.Error()})
		return
	}

	tableMeta, found := findTable(sc, schema, table)
	if !found {
		writeError(w, 404, "not_found", fmt.Sprintf("table %s.%s not found", schema, table), nil)
		return
	}

	req := db.BrowseRequest{
		Schema:  schema,
		Table:   table,
		Limit:   limit,
		Offset:  offset,
		Filters: filters,
		Sorts:   sorts,
	}

	result, err := db.Browse(r.Context(), s.cfg.Pool, tableMeta, req)
	if err != nil {
		s.cfg.Logger.Error("browse failed", "schema", schema, "table", table, "err", err)
		writeError(w, 500, "db_error", "browse failed", map[string]string{"pg_error": err.Error()})
		return
	}

	cols := make([]browseColumnJSON, len(result.Columns))
	for i, c := range result.Columns {
		cols[i] = browseColumnJSON{Name: c.Name, Type: c.Type, Editor: c.Editor}
	}

	sortJSON := make([]browseSortJSON, len(sorts))
	for i, s := range sorts {
		dir := "asc"
		if s.Desc {
			dir = "desc"
		}
		sortJSON[i] = browseSortJSON{Column: s.Column, Dir: dir}
	}

	filterJSON := make([]browseFilterJSON, len(filters))
	for i, f := range filters {
		filterJSON[i] = browseFilterJSON{Column: f.Column, Op: string(f.Op), Val: f.Value}
	}

	rows := result.Rows
	if rows == nil {
		rows = [][]wire.Cell{}
	}

	writeJSON(w, 200, browseResponseJSON{
		Columns: cols,
		Rows:    rows,
		Page:    browsePageJSON{Limit: limit, Offset: offset, Total: result.Total, IsEstimated: result.IsEstimated},
		Sort:    sortJSON,
		Filters: filterJSON,
	})
}

// parsePagination extracts limit and offset from the query values.
func parsePagination(q url.Values) (limit, offset int, err error) {
	limit = 50
	offset = 0

	if v := q.Get("limit"); v != "" {
		limit, err = strconv.Atoi(v)
		if err != nil || limit < 1 {
			return 0, 0, fmt.Errorf("invalid limit: %q", v)
		}
		if limit > 1000 {
			limit = 1000
		}
	}

	if v := q.Get("offset"); v != "" {
		offset, err = strconv.Atoi(v)
		if err != nil || offset < 0 {
			return 0, 0, fmt.Errorf("invalid offset: %q", v)
		}
	}

	return limit, offset, nil
}

// parseSortOrder parses sort[<col>]=asc|desc from rawQuery preserving order.
// It walks the raw query string to maintain multi-column sort semantics.
func parseSortOrder(rawQuery string) []db.Sort {
	var sorts []db.Sort
	seen := map[string]bool{}

	pairs := strings.Split(rawQuery, "&")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key, err := url.QueryUnescape(kv[0])
		if err != nil {
			continue
		}
		val, err := url.QueryUnescape(kv[1])
		if err != nil {
			continue
		}

		if !strings.HasPrefix(key, "sort[") || !strings.HasSuffix(key, "]") {
			continue
		}
		col := key[5 : len(key)-1]
		if col == "" || seen[col] {
			continue
		}
		seen[col] = true
		sorts = append(sorts, db.Sort{
			Column: col,
			Desc:   strings.ToLower(val) == "desc",
		})
	}
	return sorts
}

// parseFilters extracts op[<col>]+val[<col>] pairs from query values.
func parseFilters(q url.Values) []db.Filter {
	// No-value operators that don't require a val[col].
	noValueOps := map[string]bool{
		"is_null":     true,
		"is_not_null": true,
		"is_true":     true,
		"is_false":    true,
	}

	var filters []db.Filter
	for key, vals := range q {
		if !strings.HasPrefix(key, "op[") || !strings.HasSuffix(key, "]") {
			continue
		}
		col := key[3 : len(key)-1]
		if col == "" || len(vals) == 0 {
			continue
		}
		op := vals[0]
		if op == "" {
			continue
		}

		var filterVal string
		if !noValueOps[op] {
			filterVal = q.Get("val[" + col + "]")
			// If no val and op requires one, skip.
			if filterVal == "" && !noValueOps[op] {
				// Allow empty string values for some ops (e.g. equals "").
				// But if val key is entirely absent, skip.
				if _, hasVal := q["val["+col+"]"]; !hasVal {
					continue
				}
			}
		}

		filters = append(filters, db.Filter{
			Column: col,
			Op:     db.FilterOp(op),
			Value:  filterVal,
		})
	}
	return filters
}

// insertBody is the JSON body for POST .../rows.
type insertBody struct {
	Values map[string]wire.Cell `json:"values"`
}

// handleInsert handles POST /api/tables/{schema}/{table}/rows.
func (s *Server) handleInsert(w http.ResponseWriter, r *http.Request) {
	schema := chi.URLParam(r, "schema")
	table := chi.URLParam(r, "table")

	var body insertBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid_request", "invalid JSON body", nil)
		return
	}

	sc, err := s.cfg.Cache.Get(r.Context(), s.cfg.Pool, false)
	if err != nil {
		writeError(w, 500, "db_error", "failed to load schema", map[string]string{"pg_error": err.Error()})
		return
	}

	tableMeta, found := findTable(sc, schema, table)
	if !found {
		writeError(w, 404, "not_found", fmt.Sprintf("table %s.%s not found", schema, table), nil)
		return
	}

	values := cellMapToStringMap(body.Values)

	req := db.InsertRequest{
		Schema: schema,
		Table:  table,
		Values: values,
	}

	row, err := db.Insert(r.Context(), s.cfg.Pool, tableMeta, req)
	if err != nil {
		if errors.Is(err, db.ErrTableReadonly) {
			writeError(w, 403, "table_readonly", "table is read-only", nil)
			return
		}
		if errors.Is(err, db.ErrReadonly) {
			writeError(w, 403, "server_readonly", "server is in read-only mode", nil)
			return
		}
		writeError(w, 500, "db_error", "insert failed", map[string]string{"pg_error": err.Error()})
		return
	}

	s.cfg.Cache.Invalidate()
	appendAuditLog(s.cfg.AuditLog, "INSERT", schema+"."+table, 1, req)

	writeJSON(w, 201, row)
}

// updateBody is the JSON body for PATCH .../rows.
type updateBody struct {
	Where  map[string]wire.Cell `json:"where"`
	Values map[string]wire.Cell `json:"values"`
}

// handleUpdate handles PATCH /api/tables/{schema}/{table}/rows.
func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	schema := chi.URLParam(r, "schema")
	table := chi.URLParam(r, "table")

	var body updateBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid_request", "invalid JSON body", nil)
		return
	}

	// Parse confirmation header.
	var confirm int64
	if hdr := r.Header.Get("X-Dbseer-Confirm-Unscoped"); hdr != "" {
		var err error
		confirm, err = strconv.ParseInt(hdr, 10, 64)
		if err != nil {
			writeError(w, 400, "invalid_request", "invalid X-Dbseer-Confirm-Unscoped header", nil)
			return
		}
	}

	sc, err := s.cfg.Cache.Get(r.Context(), s.cfg.Pool, false)
	if err != nil {
		writeError(w, 500, "db_error", "failed to load schema", map[string]string{"pg_error": err.Error()})
		return
	}

	tableMeta, found := findTable(sc, schema, table)
	if !found {
		writeError(w, 404, "not_found", fmt.Sprintf("table %s.%s not found", schema, table), nil)
		return
	}

	req := db.UpdateRequest{
		Schema:  schema,
		Table:   table,
		Where:   cellMapToStringMap(body.Where),
		Values:  cellMapToStringMap(body.Values),
		Confirm: confirm,
	}

	rows, err := db.Update(r.Context(), s.cfg.Pool, tableMeta, req)
	if err != nil {
		var unscopedErr *db.UnscopedError
		if errors.As(err, &unscopedErr) {
			writeError(w, 409, "unscoped_mutation",
				fmt.Sprintf("this update would affect %d rows", unscopedErr.Affected),
				map[string]int64{"affected": unscopedErr.Affected})
			return
		}
		if errors.Is(err, db.ErrTableReadonly) {
			writeError(w, 403, "table_readonly", "table is read-only", nil)
			return
		}
		if errors.Is(err, db.ErrReadonly) {
			writeError(w, 403, "server_readonly", "server is in read-only mode", nil)
			return
		}
		writeError(w, 500, "db_error", "update failed", map[string]string{"pg_error": err.Error()})
		return
	}

	s.cfg.Cache.Invalidate()
	appendAuditLog(s.cfg.AuditLog, "UPDATE", schema+"."+table, int64(len(rows)), req)

	writeJSON(w, 200, rows)
}

// deleteBody is the JSON body for DELETE .../rows.
type deleteBody struct {
	Where map[string]wire.Cell `json:"where"`
}

// handleDelete handles DELETE /api/tables/{schema}/{table}/rows.
func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	schema := chi.URLParam(r, "schema")
	table := chi.URLParam(r, "table")

	var body deleteBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid_request", "invalid JSON body", nil)
		return
	}

	// Parse confirmation header.
	var confirm int64
	if hdr := r.Header.Get("X-Dbseer-Confirm-Unscoped"); hdr != "" {
		var err error
		confirm, err = strconv.ParseInt(hdr, 10, 64)
		if err != nil {
			writeError(w, 400, "invalid_request", "invalid X-Dbseer-Confirm-Unscoped header", nil)
			return
		}
	}

	sc, err := s.cfg.Cache.Get(r.Context(), s.cfg.Pool, false)
	if err != nil {
		writeError(w, 500, "db_error", "failed to load schema", map[string]string{"pg_error": err.Error()})
		return
	}

	tableMeta, found := findTable(sc, schema, table)
	if !found {
		writeError(w, 404, "not_found", fmt.Sprintf("table %s.%s not found", schema, table), nil)
		return
	}

	req := db.DeleteRequest{
		Schema:  schema,
		Table:   table,
		Where:   cellMapToStringMap(body.Where),
		Confirm: confirm,
	}

	_, err = db.Delete(r.Context(), s.cfg.Pool, tableMeta, req)
	if err != nil {
		var unscopedErr *db.UnscopedError
		if errors.As(err, &unscopedErr) {
			writeError(w, 409, "unscoped_mutation",
				fmt.Sprintf("this delete would affect %d rows", unscopedErr.Affected),
				map[string]int64{"affected": unscopedErr.Affected})
			return
		}
		if errors.Is(err, db.ErrTableReadonly) {
			writeError(w, 403, "table_readonly", "table is read-only", nil)
			return
		}
		if errors.Is(err, db.ErrReadonly) {
			writeError(w, 403, "server_readonly", "server is in read-only mode", nil)
			return
		}
		writeError(w, 500, "db_error", "delete failed", map[string]string{"pg_error": err.Error()})
		return
	}

	s.cfg.Cache.Invalidate()
	appendAuditLog(s.cfg.AuditLog, "DELETE", schema+"."+table, int64(len(body.Where)), req)

	w.WriteHeader(204)
}

// cellMapToStringMap converts a map[string]wire.Cell to map[string]string
// by stringifying the V field of each cell.
func cellMapToStringMap(cells map[string]wire.Cell) map[string]string {
	result := make(map[string]string, len(cells))
	for k, cell := range cells {
		result[k] = cellValueToString(cell.V)
	}
	return result
}

// cellValueToString converts a wire.Cell V (json.RawMessage) to a string.
func cellValueToString(v json.RawMessage) string {
	if len(v) == 0 || string(v) == "null" {
		return "null"
	}
	// Unmarshal to any, then stringify.
	var raw any
	if err := json.Unmarshal(v, &raw); err != nil {
		return string(v)
	}
	switch val := raw.(type) {
	case string:
		return val
	case float64:
		// Avoid scientific notation for integers.
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// findTable looks up a table by schema+name in the cached schema.
func findTable(sc *db.Schema, schema, table string) (db.Table, bool) {
	for _, t := range sc.Tables {
		if t.Schema == schema && t.Name == table {
			return t, true
		}
	}
	return db.Table{}, false
}

// appendAuditLog writes an entry to the audit log if it is non-nil.
func appendAuditLog(logger *safety.Logger, op, table string, affected int64, req any) {
	if logger == nil {
		return
	}
	e := safety.Entry{
		TS:       time.Now().UTC(),
		Op:       op,
		Table:    table,
		Affected: affected,
	}
	// Best-effort: ignore errors.
	_ = logger.Append(e)
}
