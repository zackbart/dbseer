import type {
  Schema,
  DiscoverInfo,
  BrowseResponse,
  HistoryEntry,
  FKTarget,
  WireCell,
  Filter,
  Sort,
  ApiErrorBody,
} from "./types";
import { encodeBrowseParams } from "./url";

// Thrown when the server returns a non-2xx response with a JSON error envelope.
export class ApiError extends Error {
  status: number;
  code: string;
  detail?: unknown;

  constructor(status: number, body: ApiErrorBody) {
    super(body.message);
    this.name = "ApiError";
    this.status = status;
    this.code = body.code;
    this.detail = body.detail;
  }
}

// Low-level fetch with error envelope handling.
async function request<T>(
  path: string,
  init?: RequestInit & { confirmUnscoped?: number }
): Promise<T> {
  const { confirmUnscoped, ...fetchInit } = init ?? {};

  const headers = new Headers(fetchInit.headers);
  if (confirmUnscoped !== undefined) {
    headers.set("X-Dbseer-Confirm-Unscoped", String(confirmUnscoped));
  }

  const response = await fetch(path, { ...fetchInit, headers });

  if (!response.ok) {
    // Attempt to parse the error envelope {"error": {...}}
    let body: ApiErrorBody = {
      code: "internal",
      message: `HTTP ${response.status} ${response.statusText}`,
    };
    try {
      const json = (await response.json()) as { error?: ApiErrorBody };
      if (json.error) {
        body = json.error;
      }
    } catch {
      // Non-JSON error body — use the default above.
    }
    throw new ApiError(response.status, body);
  }

  // 204 No Content — return undefined cast to T
  if (response.status === 204) {
    return undefined as T;
  }

  return response.json() as Promise<T>;
}

export const api = {
  discover: () => request<DiscoverInfo>("/api/discover"),

  schema: (refresh = false) =>
    request<Schema>(`/api/schema${refresh ? "?refresh=1" : ""}`),

  browse: (
    schema: string,
    table: string,
    opts: { limit?: number; offset?: number; filters?: Filter[]; sorts?: Sort[] }
  ) =>
    request<BrowseResponse>(
      `/api/tables/${encodeURIComponent(schema)}/${encodeURIComponent(table)}/rows?${encodeBrowseParams(opts)}`
    ),

  insertRow: (
    schema: string,
    table: string,
    values: Record<string, WireCell>
  ) =>
    request<WireCell[]>(
      `/api/tables/${encodeURIComponent(schema)}/${encodeURIComponent(table)}/rows`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ values }),
      }
    ),

  updateRow: (
    schema: string,
    table: string,
    where: Record<string, WireCell>,
    values: Record<string, WireCell>,
    confirmUnscoped?: number
  ) =>
    request<WireCell[][]>(
      `/api/tables/${encodeURIComponent(schema)}/${encodeURIComponent(table)}/rows`,
      {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ where, values }),
        confirmUnscoped,
      }
    ),

  deleteRow: (
    schema: string,
    table: string,
    where: Record<string, WireCell>,
    confirmUnscoped?: number
  ) =>
    request<void>(
      `/api/tables/${encodeURIComponent(schema)}/${encodeURIComponent(table)}/rows`,
      {
        method: "DELETE",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ where }),
        confirmUnscoped,
      }
    ),

  fkTarget: (schema: string, table: string, col: string, val: WireCell) =>
    request<FKTarget>(
      `/api/tables/${encodeURIComponent(schema)}/${encodeURIComponent(table)}/fk-target?col=${encodeURIComponent(col)}&val=${encodeURIComponent(JSON.stringify(val.v))}`
    ),

  history: (
    opts: { limit?: number; since?: string; table?: string } = {}
  ) => {
    const params = new URLSearchParams();
    if (opts.limit !== undefined) params.set("limit", String(opts.limit));
    if (opts.since) params.set("since", opts.since);
    if (opts.table) params.set("table", opts.table);
    const qs = params.toString();
    return request<{ entries: HistoryEntry[] }>(
      `/api/history${qs ? "?" + qs : ""}`
    );
  },
};

// React Query key factory — consistent cache keys across the app.
export const queryKeys = {
  discover: ["discover"] as const,
  schema: ["schema"] as const,
  browse: (schema: string, table: string, opts: unknown) =>
    ["browse", schema, table, opts] as const,
  history: (opts: unknown) => ["history", opts] as const,
};
