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

	// Prisma-style DSNs include ?schema=public, which isn't a real Postgres
	// runtime parameter — Postgres rejects it with FATAL "unrecognized
	// configuration parameter". Translate it into search_path (unless one
	// is already set explicitly).
	if schema, ok := config.ConnConfig.RuntimeParams["schema"]; ok {
		delete(config.ConnConfig.RuntimeParams, "schema")
		if schema != "" {
			if _, hasSearchPath := config.ConnConfig.RuntimeParams["search_path"]; !hasSearchPath {
				config.ConnConfig.RuntimeParams["search_path"] = schema
			}
		}
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
// (and their array variants) into the connection's TypeMap.
//
// Uses pgx 5.5+'s Conn.LoadTypes (plural) to fetch every type in a single
// round-trip. The previous implementation called Conn.LoadType per type
// inside a loop, which over a high-latency proxy (e.g. Railway) cost
// N×RTT per new connection — multi-second AfterConnect stalls that
// manifested as random timeouts on the first browse against any cold
// connection from the pool.
//
// On any error from LoadTypes, we log and continue with whatever pgx was
// able to register. Type loading is best-effort: exotic types that fail
// to load just won't get binary decoding, falling back to text.
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

	if len(names) == 0 {
		return nil
	}

	// Single round-trip batch load via pgx 5.5+ LoadTypes (plural).
	// LoadTypes returns the loaded types but does NOT register them — we
	// hand the slice to TypeMap.RegisterTypes to complete the process.
	// Type loading is best-effort: any error means we fall back to text
	// decoding for the missing types but the connection stays usable.
	loaded, err := conn.LoadTypes(ctx, names)
	if err != nil {
		slog.Debug("could not batch load user types", "count", len(names), "err", err)
		return nil
	}
	conn.TypeMap().RegisterTypes(loaded)
	return nil
}
