package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/zackbart/dbseer/internal/wire"
)

// UnscopedError is returned when an UPDATE or DELETE would affect more rows
// than expected (or more than 1 when no confirmation count is supplied).
type UnscopedError struct {
	Affected int64
}

// Error implements the error interface.
func (e *UnscopedError) Error() string {
	return fmt.Sprintf("unscoped mutation would affect %d rows", e.Affected)
}

// InsertRequest holds parameters for inserting a new row.
type InsertRequest struct {
	Schema string
	Table  string
	Values map[string]string // column → raw string value
}

// UpdateRequest holds parameters for updating row(s).
type UpdateRequest struct {
	Schema  string
	Table   string
	Where   map[string]string // column → value (must match PK or unique set)
	Values  map[string]string // columns to update
	Confirm int64             // 0 = no confirmation; > 0 = expected affected row count
}

// DeleteRequest holds parameters for deleting row(s).
type DeleteRequest struct {
	Schema  string
	Table   string
	Where   map[string]string
	Confirm int64
}

// Insert inserts a new row and returns the inserted row as wire cells.
// Returns ErrReadonly if the pool is read-only, ErrTableReadonly if not editable.
func Insert(ctx context.Context, pool *Pool, tableMeta Table, req InsertRequest) ([]wire.Cell, error) {
	if pool.Readonly() {
		return nil, ErrReadonly
	}
	if !tableMeta.Editable {
		return nil, ErrTableReadonly
	}

	sql, args, err := buildInsertSQL(req, tableMeta)
	if err != nil {
		return nil, err
	}

	rows, err := pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("insert: %w", err)
	}
	defer rows.Close()

	fds := rows.FieldDescriptions()
	colMap := map[string]Column{}
	for _, c := range tableMeta.Columns {
		colMap[c.Name] = c
	}

	var resultRow []wire.Cell
	if rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return nil, fmt.Errorf("scan insert result: %w", err)
		}
		resultRow = make([]wire.Cell, len(vals))
		for i, v := range vals {
			name := fds[i].Name
			if mc, ok := colMap[name]; ok {
				if mc.EnumName != nil {
					resultRow[i] = wire.MarshalEnum(v)
				} else {
					resultRow[i] = wire.MarshalWithOID(v, mc.TypeName)
				}
			} else {
				resultRow[i] = wire.Marshal(v)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("insert rows: %w", err)
	}
	return resultRow, nil
}

// Update updates rows matching the WHERE clause and returns the updated rows
// as wire cells. Enforces the unscoped mutation guard.
func Update(ctx context.Context, pool *Pool, tableMeta Table, req UpdateRequest) ([][]wire.Cell, error) {
	if pool.Readonly() {
		return nil, ErrReadonly
	}
	if !tableMeta.Editable {
		return nil, ErrTableReadonly
	}

	if err := validateWhereKeys(req.Where, tableMeta); err != nil {
		return nil, err
	}

	sql, args, err := buildUpdateSQL(req, tableMeta)
	if err != nil {
		return nil, err
	}

	return runMutationWithGuard(ctx, pool, tableMeta, sql, args, req.Confirm)
}

// Delete deletes rows matching the WHERE clause. Enforces the unscoped mutation guard.
func Delete(ctx context.Context, pool *Pool, tableMeta Table, req DeleteRequest) (int64, error) {
	if pool.Readonly() {
		return 0, ErrReadonly
	}
	if !tableMeta.Editable {
		return 0, ErrTableReadonly
	}

	if err := validateWhereKeys(req.Where, tableMeta); err != nil {
		return 0, err
	}

	sql, args, err := buildDeleteSQL(req, tableMeta)
	if err != nil {
		return 0, err
	}

	rows, err := runMutationWithGuard(ctx, pool, tableMeta, sql, args, req.Confirm)
	if err != nil {
		return 0, err
	}
	return int64(len(rows)), nil
}

// validateWhereKeys checks that the WHERE map covers all PK columns (if a PK
// exists) or all columns of at least one unique constraint. Partial keys are
// rejected to prevent unintended multi-row mutations.
func validateWhereKeys(where map[string]string, tableMeta Table) error {
	if len(tableMeta.PrimaryKey) > 0 {
		for _, pk := range tableMeta.PrimaryKey {
			if _, ok := where[pk]; !ok {
				return fmt.Errorf("WHERE clause missing primary key column %q", pk)
			}
		}
		return nil
	}

	// No PK: check unique constraints.
	for _, uc := range tableMeta.UniqueConstraints {
		covered := true
		for _, col := range uc {
			if _, ok := where[col]; !ok {
				covered = false
				break
			}
		}
		if covered {
			return nil
		}
	}

	return fmt.Errorf("WHERE clause must cover all primary key or unique constraint columns")
}

// runMutationWithGuard executes a mutation SQL (UPDATE/DELETE RETURNING *)
// inside a transaction, applies the unscoped row-count guard, and returns the
// affected rows as wire cells.
func runMutationWithGuard(ctx context.Context, pool *Pool, tableMeta Table, sql string, args []any, confirm int64) ([][]wire.Cell, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}

	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		_ = tx.Rollback(ctx)
		return nil, fmt.Errorf("mutation query: %w", err)
	}

	fds := rows.FieldDescriptions()
	colMap := map[string]Column{}
	for _, c := range tableMeta.Columns {
		colMap[c.Name] = c
	}

	var resultRows [][]wire.Cell
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			rows.Close()
			_ = tx.Rollback(ctx)
			return nil, fmt.Errorf("scan mutation result: %w", err)
		}
		cells := make([]wire.Cell, len(vals))
		for i, v := range vals {
			name := fds[i].Name
			if mc, ok := colMap[name]; ok {
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
	rows.Close()
	if err := rows.Err(); err != nil {
		_ = tx.Rollback(ctx)
		return nil, fmt.Errorf("mutation rows: %w", err)
	}

	affected := int64(len(resultRows))

	// Unscoped guard: if more than 1 row would be affected and no confirmation
	// was provided, rollback and signal the caller.
	if affected > 1 && confirm == 0 {
		_ = tx.Rollback(ctx)
		return nil, &UnscopedError{Affected: affected}
	}

	// If confirmation was provided, verify the count still matches.
	if confirm > 0 && affected != confirm {
		_ = tx.Rollback(ctx)
		return nil, &UnscopedError{Affected: affected}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return resultRows, nil
}

// buildInsertSQL constructs an INSERT ... RETURNING * statement.
func buildInsertSQL(req InsertRequest, tableMeta Table) (string, []any, error) {
	qtable, err := QualifiedTable(req.Schema, req.Table)
	if err != nil {
		return "", nil, err
	}

	colMap := map[string]Column{}
	for _, c := range tableMeta.Columns {
		colMap[c.Name] = c
	}

	// Collect columns to insert (only non-identity, non-generated that were provided).
	var colNames []string
	var placeholders []string
	var args []any
	argN := 1

	// Iterate in a stable order based on table column ordering.
	for _, col := range tableMeta.Columns {
		rawVal, provided := req.Values[col.Name]
		if !provided {
			continue
		}
		if col.IsGenerated {
			continue // Cannot insert into generated columns.
		}
		v, err := parseMutateValue(rawVal, col)
		if err != nil {
			return "", nil, err
		}
		qcol, err := Quote(col.Name)
		if err != nil {
			return "", nil, err
		}
		colNames = append(colNames, qcol)
		placeholders = append(placeholders, fmt.Sprintf("$%d", argN))
		args = append(args, v)
		argN++
	}

	if len(colNames) == 0 {
		// Default-only insert.
		return fmt.Sprintf("INSERT INTO %s DEFAULT VALUES RETURNING *", qtable), nil, nil
	}

	sql := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING *",
		qtable,
		strings.Join(colNames, ", "),
		strings.Join(placeholders, ", "),
	)
	return sql, args, nil
}

// buildUpdateSQL constructs an UPDATE ... SET ... WHERE ... RETURNING * statement.
func buildUpdateSQL(req UpdateRequest, tableMeta Table) (string, []any, error) {
	qtable, err := QualifiedTable(req.Schema, req.Table)
	if err != nil {
		return "", nil, err
	}

	colMap := map[string]Column{}
	for _, c := range tableMeta.Columns {
		colMap[c.Name] = c
	}

	var setParts []string
	var args []any
	argN := 1

	// SET clause — iterate in column order for stability.
	for _, col := range tableMeta.Columns {
		rawVal, provided := req.Values[col.Name]
		if !provided {
			continue
		}
		if col.IsGenerated {
			continue
		}
		v, err := parseMutateValue(rawVal, col)
		if err != nil {
			return "", nil, err
		}
		qcol, err := Quote(col.Name)
		if err != nil {
			return "", nil, err
		}
		setParts = append(setParts, fmt.Sprintf("%s = $%d", qcol, argN))
		args = append(args, v)
		argN++
	}
	if len(setParts) == 0 {
		return "", nil, fmt.Errorf("no columns to update")
	}

	// WHERE clause.
	var whereParts []string
	for _, col := range tableMeta.Columns {
		rawVal, provided := req.Where[col.Name]
		if !provided {
			continue
		}
		v, err := parseMutateValue(rawVal, col)
		if err != nil {
			return "", nil, err
		}
		qcol, err := Quote(col.Name)
		if err != nil {
			return "", nil, err
		}
		whereParts = append(whereParts, fmt.Sprintf("%s = $%d", qcol, argN))
		args = append(args, v)
		argN++
	}
	if len(whereParts) == 0 {
		return "", nil, fmt.Errorf("UPDATE requires a WHERE clause")
	}

	sql := fmt.Sprintf("UPDATE %s SET %s WHERE %s RETURNING *",
		qtable,
		strings.Join(setParts, ", "),
		strings.Join(whereParts, " AND "),
	)
	return sql, args, nil
}

// buildDeleteSQL constructs a DELETE ... WHERE ... RETURNING * statement.
func buildDeleteSQL(req DeleteRequest, tableMeta Table) (string, []any, error) {
	qtable, err := QualifiedTable(req.Schema, req.Table)
	if err != nil {
		return "", nil, err
	}

	colMap := map[string]Column{}
	for _, c := range tableMeta.Columns {
		colMap[c.Name] = c
	}

	var whereParts []string
	var args []any
	argN := 1

	for _, col := range tableMeta.Columns {
		rawVal, provided := req.Where[col.Name]
		if !provided {
			continue
		}
		v, err := parseMutateValue(rawVal, col)
		if err != nil {
			return "", nil, err
		}
		qcol, err := Quote(col.Name)
		if err != nil {
			return "", nil, err
		}
		whereParts = append(whereParts, fmt.Sprintf("%s = $%d", qcol, argN))
		args = append(args, v)
		argN++
	}
	if len(whereParts) == 0 {
		return "", nil, fmt.Errorf("DELETE requires a WHERE clause")
	}

	_ = colMap
	sql := fmt.Sprintf("DELETE FROM %s WHERE %s RETURNING *",
		qtable,
		strings.Join(whereParts, " AND "),
	)
	return sql, args, nil
}

// parseMutateValue coerces a raw string mutation value to the column's type.
// For now it passes the string directly to pgx which handles text casting.
// Structured types (uuid, timestamps) pass through as strings too; Postgres
// will cast via the parameterized query mechanism.
func parseMutateValue(raw string, col Column) (any, error) {
	// Pass null sentinel.
	if raw == "" || raw == "null" {
		if col.Nullable {
			return nil, nil
		}
		// Non-nullable: pass empty string and let Postgres error if invalid.
		if raw == "null" {
			return nil, nil
		}
	}
	// For all types, pass the raw string — pgx will let Postgres cast it.
	// This matches what the wire layer expects (raw string values from the client).
	return raw, nil
}
