package db

import (
	"strings"
	"testing"
)

// testCols returns a minimal column set for use in query tests.
func testCols() []Column {
	return []Column{
		{Name: "id", Ordinal: 1, TypeName: "int4", Editor: "int", Nullable: false},
		{Name: "email", Ordinal: 2, TypeName: "text", Editor: "text", Nullable: true},
		{Name: "created_at", Ordinal: 3, TypeName: "timestamptz", Editor: "timestamptz", Nullable: true},
		{Name: "role", Ordinal: 4, TypeName: "user_role", Editor: "enum", Nullable: true, EnumName: strPtr("user_role")},
	}
}

func strPtr(s string) *string { return &s }

func testTable() Table {
	return Table{
		Schema:     "public",
		Name:       "users",
		Kind:       "r",
		Editable:   true,
		PrimaryKey: []string{"id"},
		Columns:    testCols(),
	}
}

func TestBuildSQL_Simple(t *testing.T) {
	req := BrowseRequest{
		Schema: "public",
		Table:  "users",
		Limit:  50,
		Offset: 0,
	}
	sql, args, err := req.buildSQL(testCols())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args (limit, offset), got %d", len(args))
	}
	if args[0] != 50 || args[1] != 0 {
		t.Errorf("expected args [50, 0], got %v", args)
	}
	if !strings.Contains(sql, `"public"."users"`) {
		t.Errorf("SQL missing qualified table: %s", sql)
	}
	if !strings.Contains(sql, "LIMIT $1 OFFSET $2") {
		t.Errorf("SQL missing LIMIT/OFFSET placeholders: %s", sql)
	}
	if !strings.HasPrefix(sql, "SELECT * FROM") {
		t.Errorf("SQL should start with SELECT * FROM: %s", sql)
	}
}

func TestBuildSQL_FilterContains(t *testing.T) {
	req := BrowseRequest{
		Schema: "public",
		Table:  "users",
		Limit:  50,
		Filters: []Filter{
			{Column: "email", Op: OpContains, Value: "example"},
		},
	}
	sql, args, err := req.buildSQL(testCols())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(args) != 3 {
		t.Errorf("expected 3 args (filter, limit, offset), got %d: %v", len(args), args)
	}
	if args[0] != "example" {
		t.Errorf("expected arg[0] %q, got %v", "example", args[0])
	}
	if args[1] != 50 || args[2] != 0 {
		t.Errorf("expected args[1,2] [50, 0], got %v", args[1:])
	}
	if !strings.Contains(sql, `"email" ILIKE '%' || $1 || '%'`) {
		t.Errorf("SQL missing ILIKE pattern: %s", sql)
	}
	if !strings.Contains(sql, "LIMIT $2 OFFSET $3") {
		t.Errorf("SQL missing LIMIT/OFFSET placeholders: %s", sql)
	}
}

func TestBuildSQL_MultiSort(t *testing.T) {
	req := BrowseRequest{
		Schema: "public",
		Table:  "users",
		Limit:  50,
		Sorts: []Sort{
			{Column: "created_at", Desc: true},
			{Column: "email", Desc: false},
		},
	}
	sql, _, err := req.buildSQL(testCols())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, `"created_at" DESC`) {
		t.Errorf("SQL missing created_at DESC: %s", sql)
	}
	if !strings.Contains(sql, `"email" ASC`) {
		t.Errorf("SQL missing email ASC: %s", sql)
	}
	// created_at should appear before email in ORDER BY.
	idxCA := strings.Index(sql, `"created_at" DESC`)
	idxEmail := strings.Index(sql, `"email" ASC`)
	if idxCA > idxEmail {
		t.Errorf("expected created_at before email in ORDER BY: %s", sql)
	}
}

func TestBuildSQL_TiebreakerFromTableMeta(t *testing.T) {
	tbl := testTable()
	req := BrowseRequest{
		Schema: "public",
		Table:  "users",
		Limit:  50,
		Sorts: []Sort{
			{Column: "created_at", Desc: true},
		},
	}
	sql, _, err := buildSQLWithMeta(req, tbl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// PK "id" should appear as tiebreaker after "created_at".
	idxCA := strings.Index(sql, `"created_at" DESC`)
	idxID := strings.Index(sql, `"id" ASC`)
	if idxID < 0 {
		t.Errorf("SQL missing PK tiebreaker 'id ASC': %s", sql)
	}
	if idxCA > idxID {
		t.Errorf("expected created_at before id in ORDER BY: %s", sql)
	}
}

func TestBuildSQL_TiebreakerNotDuplicated(t *testing.T) {
	tbl := testTable()
	req := BrowseRequest{
		Schema: "public",
		Table:  "users",
		Limit:  50,
		Sorts: []Sort{
			{Column: "id", Desc: true}, // PK already sorted explicitly
		},
	}
	sql, _, err := buildSQLWithMeta(req, tbl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "id" should appear only once in ORDER BY.
	orderByIdx := strings.Index(sql, "ORDER BY")
	if orderByIdx < 0 {
		t.Fatalf("no ORDER BY in SQL: %s", sql)
	}
	orderByClause := sql[orderByIdx:]
	count := strings.Count(orderByClause, `"id"`)
	if count != 1 {
		t.Errorf("expected 'id' to appear exactly once in ORDER BY, got %d: %s", count, sql)
	}
}

func TestBuildSQL_InOperator(t *testing.T) {
	req := BrowseRequest{
		Schema: "public",
		Table:  "users",
		Limit:  50,
		Filters: []Filter{
			{Column: "role", Op: OpIn, Value: "a,b,c"},
		},
	}
	sql, args, err := req.buildSQL(testCols())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, `"role" = ANY($1)`) {
		t.Errorf("SQL missing ANY operator: %s", sql)
	}
	if len(args) != 3 {
		t.Errorf("expected 3 args (filter, limit, offset), got %d", len(args))
	}
	parts, ok := args[0].([]string)
	if !ok {
		t.Fatalf("expected []string arg, got %T", args[0])
	}
	if len(parts) != 3 || parts[0] != "a" || parts[1] != "b" || parts[2] != "c" {
		t.Errorf("unexpected parts: %v", parts)
	}
}

func TestBuildSQL_IsNullConsumesNoParam(t *testing.T) {
	req := BrowseRequest{
		Schema: "public",
		Table:  "users",
		Limit:  50,
		Filters: []Filter{
			{Column: "email", Op: OpIsNull},
		},
	}
	sql, args, err := req.buildSQL(testCols())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(args) != 2 {
		t.Errorf("is_null filter should produce 2 args (limit, offset), got %d: %v", len(args), args)
	}
	if !strings.Contains(sql, `"email" IS NULL`) {
		t.Errorf("SQL missing IS NULL: %s", sql)
	}
}

func TestBuildSQL_StartsWith(t *testing.T) {
	req := BrowseRequest{
		Schema: "public",
		Table:  "users",
		Limit:  50,
		Filters: []Filter{
			{Column: "email", Op: OpStartsWith, Value: "alice"},
		},
	}
	sql, args, err := req.buildSQL(testCols())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, `"email" ILIKE $1 || '%'`) {
		t.Errorf("SQL missing starts_with pattern: %s", sql)
	}
	if len(args) != 3 || args[0] != "alice" {
		t.Errorf("unexpected args: %v", args)
	}
	if args[1] != 50 || args[2] != 0 {
		t.Errorf("expected args[1,2] [50, 0], got %v", args[1:])
	}
}

func TestBuildSQL_EndsWith(t *testing.T) {
	req := BrowseRequest{
		Schema: "public",
		Table:  "users",
		Limit:  50,
		Filters: []Filter{
			{Column: "email", Op: OpEndsWith, Value: ".com"},
		},
	}
	sql, args, err := req.buildSQL(testCols())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, `"email" ILIKE '%' || $1`) {
		t.Errorf("SQL missing ends_with pattern: %s", sql)
	}
	_ = args
}

func TestBuildSQL_UnknownColumn(t *testing.T) {
	req := BrowseRequest{
		Schema: "public",
		Table:  "users",
		Limit:  50,
		Filters: []Filter{
			{Column: "nonexistent", Op: OpEq, Value: "val"},
		},
	}
	_, _, err := req.buildSQL(testCols())
	if err == nil {
		t.Fatal("expected error for unknown column, got nil")
	}
}

func TestBuildSQL_UnknownOperator(t *testing.T) {
	req := BrowseRequest{
		Schema: "public",
		Table:  "users",
		Limit:  50,
		Filters: []Filter{
			{Column: "email", Op: "bogus_op", Value: "val"},
		},
	}
	_, _, err := req.buildSQL(testCols())
	if err == nil {
		t.Fatal("expected error for unknown operator, got nil")
	}
}

func TestBuildSQL_DefaultLimit(t *testing.T) {
	req := BrowseRequest{
		Schema: "public",
		Table:  "users",
		Limit:  0, // should default to 50
	}
	sql, _, err := req.buildSQL(testCols())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "LIMIT $1") {
		t.Errorf("expected default LIMIT $1 placeholder: %s", sql)
	}
}
