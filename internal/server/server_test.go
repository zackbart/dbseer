package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zackbart/dbseer/internal/db"
	"github.com/zackbart/dbseer/internal/discover"
)

// --- parseSortOrder tests ---

func TestParseSortOrder_SingleColumn(t *testing.T) {
	sorts := parseSortOrder("sort[created_at]=desc")
	if len(sorts) != 1 {
		t.Fatalf("expected 1 sort, got %d", len(sorts))
	}
	if sorts[0].Column != "created_at" {
		t.Errorf("expected column created_at, got %s", sorts[0].Column)
	}
	if !sorts[0].Desc {
		t.Errorf("expected Desc=true")
	}
}

func TestParseSortOrder_MultiColumnOrder(t *testing.T) {
	// Verifies that multi-column sort order is preserved from raw query string.
	sorts := parseSortOrder("sort[b]=asc&sort[a]=desc&sort[c]=asc")
	if len(sorts) != 3 {
		t.Fatalf("expected 3 sorts, got %d", len(sorts))
	}
	cols := []string{sorts[0].Column, sorts[1].Column, sorts[2].Column}
	if cols[0] != "b" || cols[1] != "a" || cols[2] != "c" {
		t.Errorf("sort order not preserved: %v", cols)
	}
}

func TestParseSortOrder_Empty(t *testing.T) {
	sorts := parseSortOrder("limit=50&offset=0")
	if len(sorts) != 0 {
		t.Errorf("expected no sorts, got %d", len(sorts))
	}
}

func TestParseSortOrder_Deduplication(t *testing.T) {
	// Duplicate column: first occurrence wins.
	sorts := parseSortOrder("sort[id]=asc&sort[id]=desc")
	if len(sorts) != 1 {
		t.Fatalf("expected 1 sort after dedup, got %d", len(sorts))
	}
	if sorts[0].Desc {
		t.Errorf("expected first occurrence (asc), got desc")
	}
}

// --- parseFilters tests ---

func TestParseFilters_ValidPair(t *testing.T) {
	q := mustParseQuery("op[email]=contains&val[email]=example.com")
	filters := parseFilters(q)
	if len(filters) != 1 {
		t.Fatalf("expected 1 filter, got %d", len(filters))
	}
	if filters[0].Column != "email" {
		t.Errorf("expected column email, got %s", filters[0].Column)
	}
	if filters[0].Op != "contains" {
		t.Errorf("expected op contains, got %s", filters[0].Op)
	}
	if filters[0].Value != "example.com" {
		t.Errorf("expected val example.com, got %s", filters[0].Value)
	}
}

func TestParseFilters_NoValueOp(t *testing.T) {
	q := mustParseQuery("op[email]=is_null")
	filters := parseFilters(q)
	if len(filters) != 1 {
		t.Fatalf("expected 1 filter for is_null, got %d", len(filters))
	}
	if filters[0].Value != "" {
		t.Errorf("expected empty value for is_null, got %q", filters[0].Value)
	}
}

func TestParseFilters_MissingVal_Skipped(t *testing.T) {
	// op without val for a value-requiring op should be skipped.
	q := mustParseQuery("op[name]=contains")
	filters := parseFilters(q)
	if len(filters) != 0 {
		t.Errorf("expected 0 filters when val is missing, got %d", len(filters))
	}
}

// --- writeError / writeJSON tests ---

func TestWriteError_Envelope(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, 400, "invalid_request", "bad params", nil)

	if w.Code != 400 {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var env errorEnvelope
	if err := json.NewDecoder(w.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Error.Code != "invalid_request" {
		t.Errorf("expected code invalid_request, got %s", env.Error.Code)
	}
	if env.Error.Message != "bad params" {
		t.Errorf("expected message 'bad params', got %s", env.Error.Message)
	}
}

func TestWriteJSON_ContentType(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, 200, map[string]string{"hello": "world"})

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}
	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// --- handleDiscover with nil pool ---

func TestHandleDiscover_NilPool(t *testing.T) {
	s := &Server{
		cfg: Config{
			Pool:   nil,
			Source: discover.Source{Kind: discover.SourceEnv, Path: "/app/.env"},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/discover", nil)
	w := httptest.NewRecorder()
	s.handleDiscover(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp discoverResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "env" {
		t.Errorf("expected source env, got %s", resp.Source)
	}
	if resp.Path != "/app/.env" {
		t.Errorf("expected path /app/.env, got %s", resp.Path)
	}
}

// --- readonly guard middleware ---

func TestReadonlyGuard_BlocksMutation(t *testing.T) {
	handler := readonlyGuard(true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/tables/public/users/rows", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestReadonlyGuard_AllowsGet(t *testing.T) {
	handler := readonlyGuard(true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/tables/public/users/rows", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestReadonlyGuard_AllowsMutationWhenNotReadonly(t *testing.T) {
	handler := readonlyGuard(false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest(http.MethodDelete, "/api/tables/public/users/rows", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// --- cellValueToString tests ---

func TestCellValueToString_String(t *testing.T) {
	b, _ := json.Marshal("hello")
	if got := cellValueToString(b); got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}
}

func TestCellValueToString_Int(t *testing.T) {
	b, _ := json.Marshal(42.0)
	if got := cellValueToString(b); got != "42" {
		t.Errorf("expected '42', got %q", got)
	}
}

func TestCellValueToString_Null(t *testing.T) {
	b, _ := json.Marshal(nil)
	if got := cellValueToString(b); got != "null" {
		t.Errorf("expected 'null', got %q", got)
	}
}

// --- findTable tests ---

func TestFindTable_Found(t *testing.T) {
	sc := &db.Schema{
		Tables: []db.Table{
			{Schema: "public", Name: "users"},
			{Schema: "public", Name: "posts"},
		},
	}
	tbl, found := findTable(sc, "public", "posts")
	if !found {
		t.Fatal("expected to find table posts")
	}
	if tbl.Name != "posts" {
		t.Errorf("expected posts, got %s", tbl.Name)
	}
}

func TestFindTable_NotFound(t *testing.T) {
	sc := &db.Schema{Tables: []db.Table{{Schema: "public", Name: "users"}}}
	_, found := findTable(sc, "public", "missing")
	if found {
		t.Error("expected not found")
	}
}

// helpers

func mustParseQuery(raw string) map[string][]string {
	// Build a url.Values from a raw query string.
	req := httptest.NewRequest(http.MethodGet, "/?"+raw, nil)
	return req.URL.Query()
}
