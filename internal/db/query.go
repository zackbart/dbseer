package db

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/zackbart/dbseer/internal/wire"
)

// FilterOp is the type for filter operators used in browse requests.
type FilterOp string

// Supported filter operators.
const (
	OpContains   FilterOp = "contains"
	OpEquals     FilterOp = "equals"
	OpStartsWith FilterOp = "starts_with"
	OpEndsWith   FilterOp = "ends_with"
	OpEq         FilterOp = "eq"
	OpNe         FilterOp = "ne"
	OpLt         FilterOp = "lt"
	OpLte        FilterOp = "lte"
	OpGt         FilterOp = "gt"
	OpGte        FilterOp = "gte"
	OpIsTrue     FilterOp = "is_true"
	OpIsFalse    FilterOp = "is_false"
	OpIsNull     FilterOp = "is_null"
	OpIsNotNull  FilterOp = "is_not_null"
	OpIn         FilterOp = "in"
)

// Filter specifies a column filter for a browse request.
type Filter struct {
	Column string
	Op     FilterOp
	Value  string // raw string, parsed per column type in the SQL builder
}

// Sort specifies a sort order for a browse request.
type Sort struct {
	Column string
	Desc   bool
}

// BrowseRequest holds parameters for a paginated, filtered, sorted browse.
type BrowseRequest struct {
	Schema  string
	Table   string
	Limit   int
	Offset  int
	Filters []Filter
	Sorts   []Sort
}

// BrowseResponse is the result of a browse query.
type BrowseResponse struct {
	Columns []ResultColumn
	Rows    [][]wire.Cell
	Total   int64
}

// ResultColumn describes a column in the browse response.
type ResultColumn struct {
	Name   string
	Type   string // pg typname
	Editor string // wire type hint
}

// Browse executes a paginated browse query against the given table.
// It returns a BrowseResponse with columns, rows as wire cells, and the total
// count matching the filter (separate COUNT(*) query).
func Browse(ctx context.Context, pool *Pool, tableMeta Table, req BrowseRequest) (*BrowseResponse, error) {
	sql, args, err := req.buildSQL(tableMeta.Columns)
	if err != nil {
		return nil, err
	}

	rows, err := pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("browse query: %w", err)
	}
	defer rows.Close()

	fds := rows.FieldDescriptions()

	// Build ResultColumn list from field descriptions.
	colMeta := make([]ResultColumn, len(fds))
	colMap := map[string]Column{}
	for _, c := range tableMeta.Columns {
		colMap[c.Name] = c
	}
	for i, fd := range fds {
		name := fd.Name
		colMeta[i].Name = name
		if mc, ok := colMap[name]; ok {
			colMeta[i].Type = mc.TypeName
			colMeta[i].Editor = mc.Editor
		}
	}

	// Collect rows.
	colsByName := colMap
	var resultRows [][]wire.Cell
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		cells := make([]wire.Cell, len(vals))
		for i, v := range vals {
			name := fds[i].Name
			if mc, ok := colsByName[name]; ok {
				if mc.EnumName != nil {
					cells[i] = wire.MarshalEnum(v)
				} else {
					cells[i] = wire.MarshalWithOID(v, mc.TypeName)
				}
			} else {
				cells[i] = wire.Marshal(v)
			}
		}
		resultRows = append(resultRows, cells)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration: %w", err)
	}

	// Count query.
	total, err := browseCount(ctx, pool, tableMeta.Columns, req)
	if err != nil {
		return nil, fmt.Errorf("count query: %w", err)
	}

	return &BrowseResponse{
		Columns: colMeta,
		Rows:    resultRows,
		Total:   total,
	}, nil
}

// browseCount runs a COUNT(*) with the same WHERE clause as the browse query.
func browseCount(ctx context.Context, pool *Pool, cols []Column, req BrowseRequest) (int64, error) {
	qtable, err := QualifiedTable(req.Schema, req.Table)
	if err != nil {
		return 0, err
	}

	colMap := map[string]Column{}
	for _, c := range cols {
		colMap[c.Name] = c
	}

	var whereParts []string
	var args []any
	for _, f := range req.Filters {
		mc, ok := colMap[f.Column]
		if !ok {
			return 0, fmt.Errorf("unknown filter column: %q", f.Column)
		}
		qcol, err := Quote(f.Column)
		if err != nil {
			return 0, err
		}
		snippet, newArgs, err := buildFilterSnippet(qcol, f, mc, len(args)+1)
		if err != nil {
			return 0, err
		}
		whereParts = append(whereParts, snippet)
		args = append(args, newArgs...)
	}

	var sql string
	if len(whereParts) > 0 {
		sql = "SELECT count(*) FROM " + qtable + " WHERE " + strings.Join(whereParts, " AND ")
	} else {
		sql = "SELECT count(*) FROM " + qtable
	}

	var total int64
	if err := pool.QueryRow(ctx, sql, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

// buildSQL constructs the SELECT statement for a BrowseRequest.
// It is exported as an unexported method so it can be unit-tested without a DB.
func (req BrowseRequest) buildSQL(cols []Column) (string, []any, error) {
	qtable, err := QualifiedTable(req.Schema, req.Table)
	if err != nil {
		return "", nil, err
	}

	colMap := map[string]Column{}
	for _, c := range cols {
		colMap[c.Name] = c
	}

	var args []any
	var whereParts []string

	for _, f := range req.Filters {
		mc, ok := colMap[f.Column]
		if !ok {
			return "", nil, fmt.Errorf("unknown filter column: %q", f.Column)
		}
		qcol, err := Quote(f.Column)
		if err != nil {
			return "", nil, err
		}
		snippet, newArgs, err := buildFilterSnippet(qcol, f, mc, len(args)+1)
		if err != nil {
			return "", nil, err
		}
		whereParts = append(whereParts, snippet)
		args = append(args, newArgs...)
	}

	// ORDER BY.
	var orderParts []string
	sortedCols := map[string]bool{}
	for _, s := range req.Sorts {
		qcol, err := Quote(s.Column)
		if err != nil {
			return "", nil, err
		}
		dir := "ASC"
		if s.Desc {
			dir = "DESC"
		}
		orderParts = append(orderParts, qcol+" "+dir)
		sortedCols[s.Column] = true
	}
	// Note: tiebreaker is not appended in buildSQL (no tableMeta PK available).
	// Use buildSQLWithMeta for PK tiebreaker support.

	var sb strings.Builder
	sb.WriteString("SELECT * FROM ")
	sb.WriteString(qtable)
	if len(whereParts) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(whereParts, " AND "))
	}
	if len(orderParts) > 0 {
		sb.WriteString(" ORDER BY ")
		sb.WriteString(strings.Join(orderParts, ", "))
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	sb.WriteString(fmt.Sprintf(" LIMIT %d OFFSET %d", limit, req.Offset))

	return sb.String(), args, nil
}

// buildSQLWithMeta constructs the SELECT statement using actual PK from tableMeta.
func buildSQLWithMeta(req BrowseRequest, tableMeta Table) (string, []any, error) {
	qtable, err := QualifiedTable(req.Schema, req.Table)
	if err != nil {
		return "", nil, err
	}

	colMap := map[string]Column{}
	for _, c := range tableMeta.Columns {
		colMap[c.Name] = c
	}

	var args []any
	var whereParts []string

	for _, f := range req.Filters {
		mc, ok := colMap[f.Column]
		if !ok {
			return "", nil, fmt.Errorf("unknown filter column: %q", f.Column)
		}
		qcol, err := Quote(f.Column)
		if err != nil {
			return "", nil, err
		}
		snippet, newArgs, err := buildFilterSnippet(qcol, f, mc, len(args)+1)
		if err != nil {
			return "", nil, err
		}
		whereParts = append(whereParts, snippet)
		args = append(args, newArgs...)
	}

	// ORDER BY.
	sortedCols := map[string]bool{}
	var orderParts []string
	for _, s := range req.Sorts {
		qcol, err := Quote(s.Column)
		if err != nil {
			return "", nil, err
		}
		dir := "ASC"
		if s.Desc {
			dir = "DESC"
		}
		orderParts = append(orderParts, qcol+" "+dir)
		sortedCols[s.Column] = true
	}
	// Stable tiebreaker: append PK columns not already in sort.
	for _, pk := range tableMeta.PrimaryKey {
		if !sortedCols[pk] {
			qcol, err := Quote(pk)
			if err != nil {
				return "", nil, err
			}
			orderParts = append(orderParts, qcol+" ASC")
		}
	}

	var sb strings.Builder
	sb.WriteString("SELECT * FROM ")
	sb.WriteString(qtable)
	if len(whereParts) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(whereParts, " AND "))
	}
	if len(orderParts) > 0 {
		sb.WriteString(" ORDER BY ")
		sb.WriteString(strings.Join(orderParts, ", "))
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	sb.WriteString(fmt.Sprintf(" LIMIT %d OFFSET %d", limit, req.Offset))

	return sb.String(), args, nil
}

// buildFilterSnippet builds a single SQL filter expression for a given filter
// operation on a quoted column identifier. It returns the SQL snippet and any
// new argument values. argN is the next $N index to use.
func buildFilterSnippet(qcol string, f Filter, col Column, argN int) (string, []any, error) {
	placeholder := fmt.Sprintf("$%d", argN)

	switch f.Op {
	case OpContains:
		v, err := parseFilterValue(f.Value, col)
		if err != nil {
			return "", nil, err
		}
		return fmt.Sprintf("%s ILIKE '%%' || %s || '%%'", qcol, placeholder), []any{v}, nil

	case OpEquals, OpEq:
		v, err := parseFilterValue(f.Value, col)
		if err != nil {
			return "", nil, err
		}
		return fmt.Sprintf("%s = %s", qcol, placeholder), []any{v}, nil

	case OpStartsWith:
		v, err := parseFilterValue(f.Value, col)
		if err != nil {
			return "", nil, err
		}
		return fmt.Sprintf("%s ILIKE %s || '%%'", qcol, placeholder), []any{v}, nil

	case OpEndsWith:
		v, err := parseFilterValue(f.Value, col)
		if err != nil {
			return "", nil, err
		}
		return fmt.Sprintf("%s ILIKE '%%' || %s", qcol, placeholder), []any{v}, nil

	case OpNe:
		v, err := parseFilterValue(f.Value, col)
		if err != nil {
			return "", nil, err
		}
		return fmt.Sprintf("%s <> %s", qcol, placeholder), []any{v}, nil

	case OpLt:
		v, err := parseFilterValue(f.Value, col)
		if err != nil {
			return "", nil, err
		}
		return fmt.Sprintf("%s < %s", qcol, placeholder), []any{v}, nil

	case OpLte:
		v, err := parseFilterValue(f.Value, col)
		if err != nil {
			return "", nil, err
		}
		return fmt.Sprintf("%s <= %s", qcol, placeholder), []any{v}, nil

	case OpGt:
		v, err := parseFilterValue(f.Value, col)
		if err != nil {
			return "", nil, err
		}
		return fmt.Sprintf("%s > %s", qcol, placeholder), []any{v}, nil

	case OpGte:
		v, err := parseFilterValue(f.Value, col)
		if err != nil {
			return "", nil, err
		}
		return fmt.Sprintf("%s >= %s", qcol, placeholder), []any{v}, nil

	case OpIsNull:
		return fmt.Sprintf("%s IS NULL", qcol), nil, nil

	case OpIsNotNull:
		return fmt.Sprintf("%s IS NOT NULL", qcol), nil, nil

	case OpIsTrue:
		return fmt.Sprintf("%s IS TRUE", qcol), nil, nil

	case OpIsFalse:
		return fmt.Sprintf("%s IS FALSE", qcol), nil, nil

	case OpIn:
		// Split by comma, pass as []string — pgx handles array encoding.
		var parts []string
		for _, p := range strings.Split(f.Value, ",") {
			parts = append(parts, strings.TrimSpace(p))
		}
		return fmt.Sprintf("%s = ANY(%s)", qcol, placeholder), []any{parts}, nil

	default:
		return "", nil, fmt.Errorf("unknown filter operator: %q", f.Op)
	}
}

// parseFilterValue coerces a raw string filter value to the appropriate Go
// type for the column's Postgres type. Text stays as string; numbers are
// parsed; bools are parsed; timestamps are parsed.
func parseFilterValue(raw string, col Column) (any, error) {
	switch col.Editor {
	case string(wire.HintInt):
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid integer filter value %q for column %q: %w", raw, col.Name, err)
		}
		return v, nil

	case string(wire.HintFloat):
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid float filter value %q for column %q: %w", raw, col.Name, err)
		}
		return v, nil

	case string(wire.HintBool):
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid boolean filter value %q for column %q: %w", raw, col.Name, err)
		}
		return v, nil

	case string(wire.HintDate):
		t, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return nil, fmt.Errorf("invalid date filter value %q for column %q (expected YYYY-MM-DD): %w", raw, col.Name, err)
		}
		return t, nil

	case string(wire.HintTimestamp), string(wire.HintTimestamptz):
		formats := []string{
			time.RFC3339,
			time.RFC3339Nano,
			"2006-01-02T15:04:05",
			"2006-01-02 15:04:05",
			"2006-01-02",
		}
		for _, f := range formats {
			if t, err := time.Parse(f, raw); err == nil {
				return t, nil
			}
		}
		return nil, fmt.Errorf("invalid timestamp filter value %q for column %q", raw, col.Name)

	default:
		// text, uuid, enum, etc. — pass as string.
		return raw, nil
	}
}
