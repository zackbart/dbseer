// Package db provides Postgres connection pooling, schema introspection,
// query browsing, and row mutation for dbseer.
package db

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool wraps pgxpool.Pool with a readonly flag that gates write operations.
type Pool struct {
	*pgxpool.Pool
	readonly bool
}

// NewPool creates a new connection pool for the given DSN.
// If readonly is true, every connection starts with default_transaction_read_only=on.
func NewPool(ctx context.Context, dsn string, readonly bool) (*Pool, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}

	if readonly {
		// Belt and braces: every new connection starts with default_transaction_read_only=on.
		config.ConnConfig.RuntimeParams["default_transaction_read_only"] = "on"
	}

	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		return registerUserTypes(ctx, conn)
	}

	p, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("connect pool: %w", err)
	}
	if err := p.Ping(ctx); err != nil {
		p.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Pool{Pool: p, readonly: readonly}, nil
}

// Readonly returns true if this pool was opened in read-only mode.
func (p *Pool) Readonly() bool { return p.readonly }

// registerUserTypes loads all user-defined enum, composite, and domain types
// (and their array variants) into the connection's TypeMap. Individual type
// load errors are tolerated — exotic types that can't be loaded are skipped.
func registerUserTypes(ctx context.Context, conn *pgx.Conn) error {
	rows, err := conn.Query(ctx, `
		SELECT typname
		FROM pg_type
		WHERE typtype IN ('e', 'c', 'd')
		  AND typnamespace NOT IN (
		      SELECT oid FROM pg_namespace
		      WHERE nspname IN ('pg_catalog', 'information_schema', 'pg_toast')
		         OR nspname LIKE 'pg_temp_%'
		  )
	`)
	if err != nil {
		return fmt.Errorf("query user types: %w", err)
	}

	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			continue
		}
		names = append(names, n, "_"+n) // include array variant
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		// Non-fatal; proceed with whatever we got.
		slog.Warn("user type query error", "err", err)
	}

	for _, name := range names {
		dt, err := conn.LoadType(ctx, name)
		if err != nil {
			slog.Debug("could not load type", "type", name, "err", err)
			continue
		}
		conn.TypeMap().RegisterType(dt)
	}
	return nil
}
