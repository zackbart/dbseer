# dbseer HTTP API

**Version:** v0.1
**Base path:** `/api`
**Status:** Authoritative contract — source of truth for backend handlers and frontend client.

All responses are JSON. Timestamps are RFC 3339.

State-changing row endpoints (`POST`, `PATCH`, `DELETE` under `/api/tables/.../rows`) require `Content-Type: application/json`, reject unknown JSON fields, and reject cross-site browser requests unless the request origin matches the dbseer host. When dbseer is bound to a non-local address with HTTP auth enabled, those same endpoints also require a matching CSRF token via `X-Dbseer-CSRF`.

## Wire cell envelope

Every row cell is wrapped in an envelope:

```json
{ "v": <any JSON value or null>, "t": "<type-hint>" }
```

The `t` field tells the frontend editor which widget to render. Type hints:

| Hint | Go source | JSON shape of `v` | Notes |
|---|---|---|---|
| `text` | `string`, `varchar`, `bpchar`, `citext` | string | |
| `int` | `int2`, `int4`, `int8` | number | |
| `float` | `float4`, `float8` | number | |
| `numeric` | `numeric` | **string** | Stringified to preserve precision. |
| `bool` | `bool` | boolean | |
| `date` | `date` | string (`YYYY-MM-DD`) | |
| `timestamp` | `timestamp` | string (RFC3339, no zone) | |
| `timestamptz` | `timestamptz` | string (RFC3339) | |
| `uuid` | `uuid` | string | Canonical 8-4-4-4-12 form. |
| `jsonb` | `jsonb` | any (pass-through) | Rendered as JSON tree editor. |
| `json` | `json` | any | |
| `bytea` | `bytea` | string (`\xDEADBEEF`) | Postgres hex output. |
| `interval` | `interval` | string (ISO 8601 duration, `P1Y2M3DT4H5M6S`) | |
| `enum` | user-defined enum | string | Frontend shows dropdown using enum values from schema. |
| `array` | any `_<type>` | array of cell envelopes | Recursive. |
| `tsvector` | `tsvector` | string | Read-only in v0.1. |
| `xml` | `xml` | string | |
| `oid` | `oid`, `regclass`, `regtype` | number | |
| `bit` | `bit`, `varbit` | string (`"01010"`) | |
| `inet` | `inet` | string | |
| `cidr` | `cidr` | string | |
| `macaddr` | `macaddr`, `macaddr8` | string | |
| `range` | `int4range`, `tstzrange`, etc. | `{"lower":<any>,"upper":<any>,"lower_inc":bool,"upper_inc":bool,"empty":bool}` | |
| `money` | `money` | **string** | Never lossy-convert to number. |
| `geometry` | PostGIS `geometry`/`geography` | string (`\x<hex>`) | Placeholder rendering in v0.1. |
| `unknown` | anything else | any | Fallback; frontend renders as read-only text. |

A `null` value is always `{"v": null, "t": ""}` regardless of column type.

## Error envelope

All error responses have this shape:

```json
{
  "error": {
    "code": "<slug>",
    "message": "<human-readable>",
    "detail": <optional object>
  }
}
```

HTTP status aligns with the error class:
- `400` — validation errors (bad query string, malformed body)
- `401` — HTTP auth required for a protected server
- `403` — CSRF / same-origin rejection
- `404` — table/column/row not found
- `409` — `unscoped_mutation` (see rows endpoints)
- `500` — database or internal error

Error codes:
- `invalid_request` — bad params or body
- `invalid_request` — also used for auth / CSRF failures
- `not_found` — table or row missing
- `unscoped_mutation` — mutation would affect more than one row; requires confirmation header
- `table_readonly` — edit attempted on a view, materialized view, or no-PK table
- `server_readonly` — server started with `--readonly`, no mutations allowed
- `payload_too_large` — request body exceeds 2MB limit
- `db_error` — underlying Postgres error (detail.pg_error contains the message)
- `internal` — unhandled error

## Endpoints

### `GET /api/discover`

Returns metadata about the current connection. Called once on frontend load.

**Response:**
```json
{
  "source": "env",
  "path": "/Users/alice/projects/my-app/.env",
  "host": "127.0.0.1",
  "port": 5432,
  "database": "my_app_dev",
  "user": "postgres",
  "readonly": false,
  "project_root": "/Users/alice/projects/my-app"
}
```

`source` is one of `env`, `prisma`, `drizzle`, `compose`, `dbseer-config`, `flag`, `none`.

### `GET /api/schema`

Returns the full introspected schema. Cached server-side for 30 seconds; bypass with `?refresh=1`.

**Response:**
```json
{
  "tables": [
    {
      "schema": "public",
      "name": "users",
      "kind": "r",
      "editable": true,
      "editable_reason": null,
      "edit_key": ["id"],
      "estimated_rows": 1234,
      "columns": [
        {
          "name": "id",
          "ordinal": 1,
          "type": "uuid",
          "nullable": false,
          "default": "gen_random_uuid()",
          "is_identity": true,
          "is_generated": false,
          "editor": "uuid",
          "enum_name": null
        }
      ],
      "primary_key": ["id"],
      "unique_constraints": [["email"]],
      "foreign_keys": [
        {
          "name": "users_org_id_fkey",
          "columns": ["org_id"],
          "references": {
            "schema": "public",
            "table": "orgs",
            "columns": ["id"]
          },
          "on_delete": "CASCADE",
          "on_update": "NO ACTION"
        }
      ]
    }
  ],
  "enums": [
    { "schema": "public", "name": "role", "values": ["admin", "member", "viewer"] }
  ]
}
```

`kind` is `r` (ordinary table), `v` (view), `m` (materialized view), `p` (partitioned table).
`editable` is `false` for views, matviews, and tables with no primary key or unique constraint. `editable_reason` is a short string (e.g., `"no_primary_key"`, `"is_view"`) when `editable` is false.
`edit_key` is the canonical key dbseer uses for row edits and deletes: the primary key when present, otherwise the first unique constraint.
`editor` maps to a type hint (same vocabulary as the wire envelope) so the frontend can pre-pick an input widget without inspecting each cell.

### `GET /api/tables/{schema}/{table}/rows`

Browse rows with pagination, filter, and sort. All filter/sort/pagination is server-side.

**Query parameters:**
- `limit` (integer, default `50`, max `1000`)
- `offset` (integer, default `0`)
- `sort[<column>]` — one of `asc` or `desc`. Multiple allowed. Order matters (repeat the query param to specify multiple, and the backend preserves input order).
- `op[<column>]` — operator for a filter on `<column>`. Must be paired with a matching `val[<column>]` (except for operators that take no value).
- `val[<column>]` — filter value (string; the server parses to the column type).

**Supported operators by column type:**

| Type | Operators |
|---|---|
| text | `contains`, `equals`, `starts_with`, `ends_with`, `is_null`, `is_not_null` |
| int/float/numeric | `eq`, `ne`, `lt`, `lte`, `gt`, `gte`, `is_null`, `is_not_null` |
| date/timestamp/timestamptz | `eq`, `ne`, `lt`, `lte`, `gt`, `gte`, `is_null`, `is_not_null` |
| bool | `is_true`, `is_false`, `is_null` |
| enum | `in` (value is a comma-separated list), `is_null`, `is_not_null` |
| uuid | `eq`, `is_null`, `is_not_null` |
| jsonb/json/array/other | `is_null`, `is_not_null` only |

Filters combine with `AND`. `OR` and filter builders are v0.2.

**Response:**
```json
{
  "columns": [
    { "name": "id", "type": "uuid", "editor": "uuid" },
    { "name": "email", "type": "text", "editor": "text" }
  ],
  "rows": [
    [ {"v": "11111111-2222-3333-4444-555555555555", "t": "uuid"}, {"v": "alice@example.com", "t": "text"} ]
  ],
  "page": { "limit": 50, "offset": 0, "total": 12345 },
  "sort": [ {"column": "created_at", "dir": "desc"} ],
  "filters": [ {"column": "email", "op": "contains", "val": "example.com"} ]
}
```

### `POST /api/tables/{schema}/{table}/rows`

Insert a new row.

**Body:**
```json
{ "values": { "email": {"v": "bob@example.com", "t": "text"}, "name": {"v": "Bob", "t": "text"} } }
```

Omitted columns take their database default. The frontend must not send identity/generated columns.

The request must be same-origin in the browser and use `Content-Type: application/json`.

**Response:** `201 Created`, body is the inserted row in the same shape as a row element in `GET .../rows`.

### `PATCH /api/tables/{schema}/{table}/rows`

Update row(s) matching a `where` clause. The `where` clause MUST name every primary-key column (or unique-constraint columns) of the table; partial keys are rejected with `invalid_request`.

**Body:**
```json
{
  "where": { "id": {"v": "...", "t": "uuid"} },
  "values": { "email": {"v": "new@example.com", "t": "text"} }
}
```

The request must be same-origin in the browser and use `Content-Type: application/json`.

**Unscoped guard:** The handler runs the update inside a transaction with a row-count check. If the number of affected rows is `> 1`, the transaction is rolled back and the response is:

```json
HTTP/1.1 409 Conflict
{
  "error": {
    "code": "unscoped_mutation",
    "message": "this update would affect 42 rows",
    "detail": { "affected": 42 }
  }
}
```

To commit anyway, the client must retry with header `X-Dbseer-Confirm-Unscoped: 42` (exact count echo). The server re-verifies the count on retry and aborts if it no longer matches.

**Response (success):** `200 OK`, body is the updated row(s) as an array.

### `DELETE /api/tables/{schema}/{table}/rows`

Delete row(s) matching a `where` clause. Same unscoped-mutation guard as PATCH.

**Body:**
```json
{ "where": { "id": {"v": "...", "t": "uuid"} } }
```

The request must be same-origin in the browser and use `Content-Type: application/json`.

**Response (success):** `204 No Content`.

### `GET /api/tables/{schema}/{table}/fk-target`

Resolve the target table for a foreign-key click. Given a column name and a value, returns which table the frontend should navigate to, and the filter to pre-seed.

**Query parameters:**
- `col` — the column in this table
- `val` — the cell's raw wire-format value (URL-encoded JSON of the `v` field)

**Response:**
```json
{
  "schema": "public",
  "table": "orgs",
  "pk": ["id"],
  "filter": { "id": { "op": "eq", "val": "..." } }
}
```

If the column has no FK, returns `404 not_found`.
If the column has multiple FKs (uncommon), returns the first one; a v0.2 picker UI may expose the choice.

### `GET /api/history`

Read the audit log (`.dbseer/history.jsonl`) with optional filters.

**Query parameters:**
- `limit` (default `50`, max `1000`)
- `since` (RFC 3339 timestamp)
- `table` (exact `schema.name` match)

**Response:**
```json
{
  "entries": [
    {
      "ts": "2026-04-09T12:34:56Z",
      "op": "UPDATE",
      "table": "public.users",
      "affected": 1,
      "sql": "UPDATE ...",
      "params": [...]
    }
  ]
}
```

## Dev mode

When `dbseer` is built with `-tags dev` and run with `--dev`, the backend reverse-proxies every non-`/api/*` request to the Vite dev server on `http://localhost:5173`. The user visits `http://localhost:4983` regardless. In production builds, the same non-API paths are served from the embedded frontend filesystem at `internal/ui/dist`.

All responses include security headers: `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `X-XSS-Protection: 1; mode=block`, `Referrer-Policy: strict-origin-when-cross-origin`, and `Permissions-Policy` to disable browser features not needed by the app.

## Notes for clients

- **Idempotency:** POST/PATCH/DELETE are not idempotent in v0.1. Do not retry blindly.
- **Pagination:** `total` is an exact `COUNT(*)`. Large-table estimation is v0.2.
- **Schema cache:** The frontend should cache `/api/schema` with `staleTime: 30s`. Invalidate after any successful mutation on any table in that schema.
- **Transport:** HTTP only. No websocket, no SSE in v0.1.
