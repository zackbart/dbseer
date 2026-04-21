package db

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/zackbart/dbseer/internal/wire"
)

func textCell(v string) wire.Cell {
	b, _ := json.Marshal(v)
	return wire.Cell{V: b, T: wire.HintText}
}

func intCell(v int64) wire.Cell {
	b, _ := json.Marshal(v)
	return wire.Cell{V: b, T: wire.HintInt}
}

func nullCell() wire.Cell {
	return wire.Cell{V: json.RawMessage("null")}
}

func mutateTestTable() Table {
	return Table{
		Schema:     "public",
		Name:       "users",
		Kind:       "r",
		Editable:   true,
		PrimaryKey: []string{"id"},
		Columns: []Column{
			{Name: "id", Ordinal: 1, TypeName: "int4", Editor: "int", Nullable: false},
			{Name: "email", Ordinal: 2, TypeName: "text", Editor: "text", Nullable: true},
			{Name: "name", Ordinal: 3, TypeName: "text", Editor: "text", Nullable: true},
		},
	}
}

func multiPKTable() Table {
	return Table{
		Schema:     "public",
		Name:       "memberships",
		Kind:       "r",
		Editable:   true,
		PrimaryKey: []string{"user_id", "org_id"},
		Columns: []Column{
			{Name: "user_id", Ordinal: 1, TypeName: "int4", Editor: "int", Nullable: false},
			{Name: "org_id", Ordinal: 2, TypeName: "int4", Editor: "int", Nullable: false},
			{Name: "role", Ordinal: 3, TypeName: "text", Editor: "text", Nullable: true},
		},
	}
}

func uniqueOnlyTable() Table {
	return Table{
		Schema:            "public",
		Name:              "nopk",
		Kind:              "r",
		Editable:          true,
		PrimaryKey:        nil,
		UniqueConstraints: [][]string{{"email"}},
		Columns: []Column{
			{Name: "email", Ordinal: 1, TypeName: "text", Editor: "text", Nullable: false},
			{Name: "name", Ordinal: 2, TypeName: "text", Editor: "text", Nullable: true},
		},
	}
}

// --- INSERT SQL ---

func TestBuildInsertSQL_Basic(t *testing.T) {
	req := InsertRequest{
		Schema: "public",
		Table:  "users",
		Values: map[string]wire.Cell{
			"email": textCell("alice@example.com"),
			"name":  textCell("Alice"),
		},
	}
	sql, args, err := buildInsertSQL(req, mutateTestTable())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(sql, "INSERT INTO") {
		t.Errorf("SQL should start with INSERT INTO: %s", sql)
	}
	if !strings.Contains(sql, `"public"."users"`) {
		t.Errorf("SQL missing qualified table: %s", sql)
	}
	if !strings.Contains(sql, "RETURNING *") {
		t.Errorf("SQL missing RETURNING *: %s", sql)
	}
	if !strings.Contains(sql, `"email"`) {
		t.Errorf("SQL missing email column: %s", sql)
	}
	if !strings.Contains(sql, `"name"`) {
		t.Errorf("SQL missing name column: %s", sql)
	}
	// Should have placeholders for the 2 values.
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %d: %v", len(args), args)
	}
}

func TestBuildInsertSQL_DefaultValues(t *testing.T) {
	req := InsertRequest{
		Schema: "public",
		Table:  "users",
		Values: map[string]wire.Cell{},
	}
	sql, args, err := buildInsertSQL(req, mutateTestTable())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "DEFAULT VALUES") {
		t.Errorf("SQL should use DEFAULT VALUES when no columns given: %s", sql)
	}
	if len(args) != 0 {
		t.Errorf("expected 0 args, got %d", len(args))
	}
}

// --- UPDATE SQL ---

func TestBuildUpdateSQL_Basic(t *testing.T) {
	req := UpdateRequest{
		Schema:  "public",
		Table:   "users",
		Where:   map[string]wire.Cell{"id": intCell(42)},
		Values:  map[string]wire.Cell{"email": textCell("new@example.com")},
		Confirm: 0,
	}
	sql, args, err := buildUpdateSQL(req, mutateTestTable())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(sql, "UPDATE") {
		t.Errorf("SQL should start with UPDATE: %s", sql)
	}
	if !strings.Contains(sql, `"public"."users"`) {
		t.Errorf("SQL missing qualified table: %s", sql)
	}
	if !strings.Contains(sql, "SET") {
		t.Errorf("SQL missing SET clause: %s", sql)
	}
	if !strings.Contains(sql, "WHERE") {
		t.Errorf("SQL missing WHERE clause: %s", sql)
	}
	if !strings.Contains(sql, `"id"`) {
		t.Errorf("SQL missing PK column in WHERE: %s", sql)
	}
	if !strings.Contains(sql, "RETURNING *") {
		t.Errorf("SQL missing RETURNING *: %s", sql)
	}
	_ = args
}

func TestBuildUpdateSQL_PKInWhere(t *testing.T) {
	req := UpdateRequest{
		Schema: "public",
		Table:  "users",
		Where:  map[string]wire.Cell{"id": intCell(1)},
		Values: map[string]wire.Cell{"name": textCell("Bob")},
	}
	sql, _, err := buildUpdateSQL(req, mutateTestTable())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// WHERE clause must contain the id column.
	whereIdx := strings.Index(sql, "WHERE")
	if whereIdx < 0 {
		t.Fatalf("no WHERE in SQL: %s", sql)
	}
	whereClause := sql[whereIdx:]
	if !strings.Contains(whereClause, `"id"`) {
		t.Errorf("WHERE clause missing id column: %s", whereClause)
	}
}

func TestBuildUpdateSQL_NoColumns(t *testing.T) {
	req := UpdateRequest{
		Schema: "public",
		Table:  "users",
		Where:  map[string]wire.Cell{"id": intCell(1)},
		Values: map[string]wire.Cell{},
	}
	_, _, err := buildUpdateSQL(req, mutateTestTable())
	if err == nil {
		t.Fatal("expected error for empty values, got nil")
	}
}

// --- DELETE SQL ---

func TestBuildDeleteSQL_Basic(t *testing.T) {
	req := DeleteRequest{
		Schema: "public",
		Table:  "users",
		Where:  map[string]wire.Cell{"id": intCell(42)},
	}
	sql, args, err := buildDeleteSQL(req, mutateTestTable())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(sql, "DELETE FROM") {
		t.Errorf("SQL should start with DELETE FROM: %s", sql)
	}
	if !strings.Contains(sql, `"public"."users"`) {
		t.Errorf("SQL missing qualified table: %s", sql)
	}
	if !strings.Contains(sql, `"id" = $1`) {
		t.Errorf("SQL missing WHERE id clause: %s", sql)
	}
	if !strings.Contains(sql, "RETURNING *") {
		t.Errorf("SQL missing RETURNING *: %s", sql)
	}
	if len(args) != 1 {
		t.Errorf("expected 1 arg, got %d: %v", len(args), args)
	}
}

func TestBuildDeleteSQL_NoWhere(t *testing.T) {
	req := DeleteRequest{
		Schema: "public",
		Table:  "users",
		Where:  map[string]wire.Cell{},
	}
	_, _, err := buildDeleteSQL(req, mutateTestTable())
	if err == nil {
		t.Fatal("expected error for empty WHERE, got nil")
	}
}

// --- validateWhereKeys ---

func TestValidateWhereKeys_PKFull(t *testing.T) {
	tbl := mutateTestTable()
	err := validateWhereKeys(map[string]wire.Cell{"id": intCell(1)}, tbl)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateWhereKeys_PKPartial(t *testing.T) {
	tbl := multiPKTable()
	// Only user_id provided, missing org_id.
	err := validateWhereKeys(map[string]wire.Cell{"user_id": intCell(1)}, tbl)
	if err == nil {
		t.Fatal("expected error for partial PK WHERE, got nil")
	}
}

func TestValidateWhereKeys_PKFull_Multi(t *testing.T) {
	tbl := multiPKTable()
	err := validateWhereKeys(map[string]wire.Cell{"user_id": intCell(1), "org_id": intCell(2)}, tbl)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateWhereKeys_UniqueConstraint(t *testing.T) {
	tbl := uniqueOnlyTable()
	err := validateWhereKeys(map[string]wire.Cell{"email": textCell("alice@example.com")}, tbl)
	if err != nil {
		t.Errorf("expected no error when full unique constraint provided, got: %v", err)
	}
}

func TestValidateWhereKeys_NoPKAndNoUC(t *testing.T) {
	tbl := Table{
		Schema:         "public",
		Name:           "nopk",
		Kind:           "r",
		Editable:       false,
		EditableReason: "no_primary_key",
	}
	err := validateWhereKeys(map[string]wire.Cell{"col": textCell("val")}, tbl)
	if err == nil {
		t.Fatal("expected error for table with no PK and no UC, got nil")
	}
}

func TestBuildUpdateSQL_NullAssignmentUsesNULLLiteral(t *testing.T) {
	req := UpdateRequest{
		Schema: "public",
		Table:  "users",
		Where:  map[string]wire.Cell{"id": intCell(1)},
		Values: map[string]wire.Cell{"name": nullCell()},
	}

	sql, args, err := buildUpdateSQL(req, mutateTestTable())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, `"name" = NULL`) {
		t.Fatalf("expected NULL assignment in SQL, got %s", sql)
	}
	if len(args) != 1 {
		t.Fatalf("expected only WHERE arg, got %d", len(args))
	}
}

func TestBuildDeleteSQL_NullWhereUsesIsNull(t *testing.T) {
	tbl := uniqueOnlyTable()
	req := DeleteRequest{
		Schema: "public",
		Table:  "nopk",
		Where:  map[string]wire.Cell{"name": nullCell()},
	}

	sql, _, err := buildDeleteSQL(req, tbl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, `"name" IS NULL`) {
		t.Fatalf("expected IS NULL predicate, got %s", sql)
	}
}

func TestParseMutateValue_PreservesEmptyString(t *testing.T) {
	v, err := parseMutateValue(textCell(""), mutateTestTable().Columns[1])
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, ok := v.(string); !ok || got != "" {
		t.Fatalf("expected empty string, got %#v", v)
	}
}

// --- UnscopedError ---

func TestUnscopedError_Message(t *testing.T) {
	err := &UnscopedError{Affected: 42}
	want := "unscoped mutation would affect 42 rows"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}
