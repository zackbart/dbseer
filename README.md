# dbseer

A lightweight, browser-based Postgres GUI for dev environments.

Modeled on Prisma Studio — browse, filter, sort, and edit rows, with click-a-foreign-key-to-jump navigation — but ORM-agnostic. Works against any Postgres database by introspecting `information_schema` and `pg_catalog` directly. Single static Go binary with the frontend embedded; no runtime dependencies.

> **Status:** v0.1 scaffold. Core flows are wired end-to-end but there are known gaps (see "Known limitations" below).

## Row grid

- Click a row to select it; shift-click another row to select the full visible range.
- Cmd/Ctrl-click toggles individual rows in the current page selection.
- Use **Delete selected** or press Delete/Backspace to delete selected rows on editable tables.
- Double-click a cell to edit it. Arrow keys move the focused cell, and Cmd/Ctrl+C copies the focused cell value.

## Install

```sh
brew install zackbart/tap/dbseer
```

`go install` is **not supported** in v0.1 — the embedded frontend is built in CI, not at install time. If you want to build from source, clone and run `make build`.

## Usage

From inside any project directory:

```sh
cd ~/projects/my-app
dbseer
```

dbseer walks upward from the current directory looking for a Postgres connection in this priority order (first match wins):

1. `DATABASE_URL` / `POSTGRES_URL` / `POSTGRES_URI` / `PG_URL` in `.env`, `.env.local`, `.env.development`, `.env.development.local`
2. `prisma/schema.prisma` — reads the `datasource db { url = env("…") }` block and resolves the env var against the same `.env` chain
3. `drizzle.config.{ts,js,mjs}` — reads `url:` or `connectionString:` string literals and `process.env.*` references
4. `docker-compose.yml` / `compose.yaml` — detects a `postgres` service and builds a localhost URL from its environment + published port
5. `.dbseer.json` — explicit per-project config for multi-environment setups

When you run `dbseer` from a repo root that contains multiple nested database projects, it will scan downward too and open a small terminal picker so you can choose the right source without `cd`-ing around.

To override:

```sh
dbseer --url postgres://user:pass@localhost:5432/mydb
```

To see what dbseer discovered without actually connecting:

```sh
dbseer --which
```

## Flags

| Flag | Default | Purpose |
|---|---|---|
| `--url <dsn>` | — | Override discovery with a literal Postgres URL |
| `--host <addr>` | `127.0.0.1` | HTTP bind address |
| `--port <n>` | `4983` | HTTP bind port |
| `--allow-remote` | off | Allow non-localhost DB hosts |
| `--allow-prod` | off | Allow hostnames matching prod patterns (see below) |
| `--readonly` | off | Disable edit UI and set `default_transaction_read_only=on` on every session |
| `--which` / `--dry-run` | — | Print the discovered connection and exit |
| `--env <name>` | — | Pick a named environment from `.dbseer.json` |
| `--http-user <name>` | env / off | HTTP basic-auth username for non-local binds |
| `--http-password <value>` | env / off | HTTP basic-auth password for non-local binds |
| `--no-open` | off | Don't open a browser automatically |
| `--debug` / `--quiet` | — | slog level override (default Info) |
| `--version` / `-v` | — | Print version and exit |
| `--help` / `-h` | — | Print usage and exit |

## Subcommands

```sh
dbseer history [--limit N] [--since DURATION] [--table schema.name] [--json]
```

Prints the audit log (`.dbseer/history.jsonl` in the discovered project root, or `$XDG_STATE_HOME/dbseer/history.jsonl` when run outside a project). Every successful `INSERT`/`UPDATE`/`DELETE` is appended to this file.

## Safety rails

dbseer is **strictly a dev tool**. It refuses to connect to anything that looks like production by default:

- Binds to `127.0.0.1` only by default; non-local binds require `--http-user` and `--http-password`
- Refuses non-localhost DB hosts unless `--allow-remote` is passed
- Refuses hostnames matching prod patterns unless `--allow-prod` is passed:
  - Suffixes: `*.rds.amazonaws.com`, `*.supabase.co`, `*.supabase.com`, `*.neon.tech`, `*.neon.build`, `*.planetscale.com`, `*.cockroachlabs.cloud`
  - Any segment containing the word `prod` (word-boundary match — `productdb` does NOT match, `db-prod-1` does)
- Unscoped `UPDATE`/`DELETE` (affecting more than one row) trigger a confirmation modal requiring you to type the exact affected row count
- Every successful mutation is appended to `.dbseer/history.jsonl` with timestamp, SQL, and parameters

When a safety rail fires, the error message tells you exactly which flag would unblock it and suggests running `--which` to see what was discovered without connecting.

### Security hardening

The server includes several security measures:

- **HTTP security headers:** `X-Content-Type-Options`, `X-Frame-Options`, `Content-Security-Policy`, `Cross-Origin-Opener-Policy`, `Referrer-Policy`, `Permissions-Policy`
- **Same-origin write protection:** `POST`/`PATCH`/`DELETE` row mutations reject cross-site browser requests by validating `Origin` / `Referer` / `Sec-Fetch-Site`
- **Authenticated remote access:** when binding to a non-local address, dbseer requires HTTP basic auth and a CSRF token on every state-changing request
- **Request timeouts:** Read timeout 10s, write timeout 30s, idle timeout 120s
- **Request body limit:** Max 2MB per request
- **Strict JSON decoding:** mutation endpoints require `Content-Type: application/json`, reject unknown fields, and reject multiple JSON payloads in one body
- **Localhost-only by default:** Binds to `127.0.0.1` only

## Filter operators

| Column type | Supported operators |
|---|---|
| text | `contains`, `equals`, `starts_with`, `ends_with`, `is_null`, `is_not_null` |
| int, float, numeric, money | `eq`, `ne`, `lt`, `lte`, `gt`, `gte`, `is_null`, `is_not_null` |
| date, timestamp, timestamptz | `eq`, `ne`, `lt`, `lte`, `gt`, `gte`, `is_null`, `is_not_null` |
| bool | `is_true`, `is_false`, `is_null` |
| enum | `in` (comma-separated), `is_null`, `is_not_null` |
| uuid | `eq`, `is_null`, `is_not_null` |
| jsonb, json, array, other | `is_null`, `is_not_null` only in v0.1 |

Filters combine with `AND`. `OR` and JSON path filters are v0.2.

## Architecture

- **Backend:** Go 1.22+, [pgx v5](https://github.com/jackc/pgx), [chi v5](https://github.com/go-chi/chi), stdlib `log/slog`. Single binary with the frontend embedded via `//go:embed all:dist`.
- **Frontend:** React 18, TypeScript strict, [TanStack Query v5](https://tanstack.com/query), [TanStack Table v8](https://tanstack.com/table), [React Router v6](https://reactrouter.com), Tailwind v3. Filter/sort/page state lives in the URL — back/forward and deep-linking Just Work.
- **Dev loop:** `make dev` runs `air` (Go hot-reload) and `vite` (HMR) in parallel. Go on `:4983`, Vite on `:5173`; Go (when built with `-tags dev`) reverse-proxies non-`/api/*` requests to Vite. You visit `http://localhost:4983` only.
- **Distribution:** `goreleaser` builds binaries for `darwin/linux × amd64/arm64` on each `v*` tag and publishes them to GitHub Releases. The Homebrew formula in `zackbart/homebrew-tap` is auto-updated via goreleaser's `brews:` section.

## Developing

```sh
git clone https://github.com/zackbart/dbseer
cd dbseer
make dev                      # air + vite, hot reload on both sides
```

Other targets:

```sh
make build                    # full production build (pnpm build + go build + embed)
make test                     # go test ./...
make test-integration         # run real Postgres mutation tests with DBSEER_TEST_POSTGRES_DSN=...
make lint                     # golangci-lint + eslint
make fmt                      # gofmt + prettier
make clean                    # rm binary + dist + node_modules
```

Project layout:

```
cmd/dbseer/                   CLI entry point, flag parsing, subcommands
internal/
  discover/                   auto-discovery chain (.env, schema.prisma, drizzle, compose, .dbseer.json)
  db/                         pgxpool + introspection + query builder + row-count-guarded mutations
  wire/                       pgtype → JSON cell envelope (every Postgres type variant)
  safety/                     host classification, prod-pattern detection, audit log, actionable errors
  server/                     chi router + handlers, SPA fallback
  ui/                         embed.FS for web/dist, dev vs prod build tags
web/                          Vite + React + Tailwind frontend
  src/lib/                    types + API client + URL codec
  src/routes/                 TableView, HistoryView
  src/components/             DataGrid, CellEditor, FilterBar, FkLink, ConfirmUnscoped, etc.
docs/api.md                   authoritative HTTP API contract
```

Tests include unit coverage plus opt-in real Postgres mutation integration tests in `internal/db`. Run them with `DBSEER_TEST_POSTGRES_DSN=postgres://... make test-integration`.

## Known limitations (v0.1)

- **`go install` is unsupported** — install via Homebrew, or build from source with `make build`. The embedded frontend must be built before the Go compile, which `go install` can't arrange.
- **Tables without a primary key or unique constraint are read-only in the UI.** Tables with a unique constraint but no primary key are editable using the server-provided `edit_key`.
- **Views and materialized views are read-only** (expected — there's no sane "edit a view" story).
- **PostGIS geometry columns display as `\xHEX`** placeholders. Rich map rendering is v0.2.
- **JSON path filters are not supported** — only `is_null`/`is_not_null` on jsonb/json columns in v0.1.
- **Row virtualization is not implemented** — page-based pagination (25/50/100/250) only. Tables with millions of rows will work; the grid just renders one page at a time.
- **No query editor, no schema editing, no non-Postgres databases** — all v0.2+.
- **Not a multi-user admin tool** — remote access now requires explicit HTTP auth, but dbseer is still designed for trusted dev environments rather than shared production use.

## License

MIT — see [LICENSE](./LICENSE).
