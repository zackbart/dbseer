import type { Filter, Sort } from "./types";

/**
 * Encode filters + sorts into a query string.
 *
 * Format per docs/api.md:
 *   sort[<col>]=asc|desc       (order matters — built by hand, NOT URLSearchParams)
 *   op[<col>]=<operator>
 *   val[<col>]=<value>
 *   limit, offset
 *
 * URLSearchParams is deliberately avoided because the server parses sort[] entries
 * in the order they appear in the raw query string, and URLSearchParams is an
 * unordered multimap on the server side.
 */
export function encodeBrowseParams(opts: {
  limit?: number;
  offset?: number;
  filters?: Filter[];
  sorts?: Sort[];
}): string {
  const parts: string[] = [];

  if (opts.limit !== undefined) {
    parts.push(`limit=${encodeURIComponent(String(opts.limit))}`);
  }
  if (opts.offset !== undefined) {
    parts.push(`offset=${encodeURIComponent(String(opts.offset))}`);
  }

  for (const sort of opts.sorts ?? []) {
    const key = encodeURIComponent(`sort[${sort.column}]`);
    const val = encodeURIComponent(sort.desc ? "desc" : "asc");
    parts.push(`${key}=${val}`);
  }

  for (const filter of opts.filters ?? []) {
    const opKey = encodeURIComponent(`op[${filter.column}]`);
    const opVal = encodeURIComponent(filter.op);
    parts.push(`${opKey}=${opVal}`);

    if (filter.val !== undefined) {
      const valKey = encodeURIComponent(`val[${filter.column}]`);
      const valVal = encodeURIComponent(filter.val);
      parts.push(`${valKey}=${valVal}`);
    }
  }

  return parts.join("&");
}

export function decodeBrowseParams(search: string): {
  limit: number;
  offset: number;
  filters: Filter[];
  sorts: Sort[];
} {
  const raw = search.startsWith("?") ? search.slice(1) : search;

  let limit = 50;
  let offset = 0;
  const sorts: Sort[] = [];
  // Track order so we can reconstruct sorts in input order.
  const sortOrder: string[] = [];
  const sortMap: Record<string, boolean> = {};
  const opMap: Record<string, string> = {};
  const valMap: Record<string, string> = {};

  if (raw === "") {
    return { limit, offset, filters: [], sorts: [] };
  }

  for (const part of raw.split("&")) {
    const eqIdx = part.indexOf("=");
    if (eqIdx === -1) continue;

    const rawKey = decodeURIComponent(part.slice(0, eqIdx));
    const rawVal = decodeURIComponent(part.slice(eqIdx + 1));

    if (rawKey === "limit") {
      const parsed = parseInt(rawVal, 10);
      if (!isNaN(parsed)) limit = parsed;
    } else if (rawKey === "offset") {
      const parsed = parseInt(rawVal, 10);
      if (!isNaN(parsed)) offset = parsed;
    } else {
      const sortMatch = /^sort\[(.+)\]$/.exec(rawKey);
      const opMatch = /^op\[(.+)\]$/.exec(rawKey);
      const valMatch = /^val\[(.+)\]$/.exec(rawKey);

      if (sortMatch) {
        const col = sortMatch[1];
        sortOrder.push(col);
        sortMap[col] = rawVal === "desc";
      } else if (opMatch) {
        opMap[opMatch[1]] = rawVal;
      } else if (valMatch) {
        valMap[valMatch[1]] = rawVal;
      }
    }
  }

  for (const col of sortOrder) {
    sorts.push({ column: col, desc: sortMap[col] });
  }

  const filters: Filter[] = [];
  for (const col of Object.keys(opMap)) {
    const op = opMap[col] as Filter["op"];
    const val = valMap[col];
    filters.push(val !== undefined ? { column: col, op, val } : { column: col, op });
  }

  return { limit, offset, filters, sorts };
}
