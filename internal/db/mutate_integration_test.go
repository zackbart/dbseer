package db

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/zackbart/dbseer/internal/wire"
)

func TestMutationIntegration_InsertUpdateDelete(t *testing.T) {
	dsn := os.Getenv("DBSEER_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set DBSEER_TEST_POSTGRES_DSN to run real Postgres mutation integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := NewPool(ctx, dsn, false)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	defer pool.Close()

	schema := fmt.Sprintf("dbseer_it_%d", time.Now().UnixNano())
	qschema, _ := Quote(schema)
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+qschema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA "+qschema+" CASCADE")
	}()

	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %s.widgets (
			code text UNIQUE NOT NULL,
			name text NOT NULL,
			note text,
			payload jsonb NOT NULL DEFAULT '{}'::jsonb
		)
	`, qschema)); err != nil {
		t.Fatalf("create table: %v", err)
	}

	tableMeta := mustFindIntegrationTable(t, ctx, pool, schema, "widgets")

	inserted, err := Insert(ctx, pool, tableMeta, InsertRequest{
		Schema: schema,
		Table:  "widgets",
		Values: map[string]wire.Cell{
			"code":    textWireCell("alpha"),
			"name":    textWireCell("Alpha"),
			"note":    nullWireCell(),
			"payload": jsonWireCell(map[string]any{"enabled": true}),
		},
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if len(inserted) != len(tableMeta.Columns) {
		t.Fatalf("expected %d inserted columns, got %d", len(tableMeta.Columns), len(inserted))
	}

	updated, err := Update(ctx, pool, tableMeta, UpdateRequest{
		Schema: schema,
		Table:  "widgets",
		Where: map[string]wire.Cell{
			"code": textWireCell("alpha"),
		},
		Values: map[string]wire.Cell{
			"name":    textWireCell("Renamed"),
			"note":    textWireCell("created by integration test"),
			"payload": jsonWireCell(map[string]any{"enabled": false, "version": 2}),
		},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(updated) != 1 {
		t.Fatalf("expected 1 updated row, got %d", len(updated))
	}

	var (
		name string
		note string
	)
	if err := pool.QueryRow(ctx, "SELECT name, note FROM "+qschema+`.widgets WHERE code = 'alpha'`).Scan(&name, &note); err != nil {
		t.Fatalf("verify update: %v", err)
	}
	if name != "Renamed" || note != "created by integration test" {
		t.Fatalf("unexpected updated values: name=%q note=%q", name, note)
	}

	deleted, err := Delete(ctx, pool, tableMeta, DeleteRequest{
		Schema: schema,
		Table:  "widgets",
		Where: map[string]wire.Cell{
			"code": textWireCell("alpha"),
		},
	})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 deleted row, got %d", deleted)
	}
}

func mustFindIntegrationTable(t *testing.T, ctx context.Context, pool *Pool, schema, table string) Table {
	t.Helper()

	introspected, err := Introspect(ctx, pool)
	if err != nil {
		t.Fatalf("introspect: %v", err)
	}
	for _, candidate := range introspected.Tables {
		if candidate.Schema == schema && candidate.Name == table {
			return candidate
		}
	}
	t.Fatalf("table %s.%s not found", schema, table)
	return Table{}
}

func textWireCell(value string) wire.Cell {
	b, _ := json.Marshal(value)
	return wire.Cell{V: b, T: wire.HintText}
}

func jsonWireCell(value any) wire.Cell {
	b, _ := json.Marshal(value)
	return wire.Cell{V: b, T: wire.HintJSONB}
}

func nullWireCell() wire.Cell {
	return wire.Cell{V: json.RawMessage("null")}
}
