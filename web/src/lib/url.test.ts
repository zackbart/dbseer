/**
 * Manual sanity tests for encodeBrowseParams / decodeBrowseParams.
 *
 * Not wired to a test runner yet. Run with:
 *   npx tsx src/lib/url.test.ts
 *
 * Each assertion throws if the condition is false.
 */
import { encodeBrowseParams, decodeBrowseParams } from "./url";

function assert(cond: boolean, msg: string): void {
  if (!cond) throw new Error(`FAIL: ${msg}`);
}

function deepEqual(a: unknown, b: unknown): boolean {
  return JSON.stringify(a) === JSON.stringify(b);
}

// --- encodeBrowseParams ---

// Empty options produce empty string.
assert(encodeBrowseParams({}) === "", "empty opts → empty string");

// limit + offset
const basic = encodeBrowseParams({ limit: 10, offset: 20 });
assert(basic === "limit=10&offset=20", `basic pagination: got "${basic}"`);

// Single sort
const s1 = encodeBrowseParams({ sorts: [{ column: "name", desc: false }] });
assert(
  s1 === "sort%5Bname%5D=asc",
  `single sort asc: got "${s1}"`
);

// Multiple sorts preserve order
const s2 = encodeBrowseParams({
  sorts: [
    { column: "created_at", desc: true },
    { column: "name", desc: false },
  ],
});
const s2Parts = s2.split("&");
assert(s2Parts[0].startsWith("sort%5Bcreated_at%5D"), `first sort is created_at, got "${s2Parts[0]}"`);
assert(s2Parts[1].startsWith("sort%5Bname%5D"), `second sort is name, got "${s2Parts[1]}"`);

// Filter with val
const f1 = encodeBrowseParams({
  filters: [{ column: "email", op: "contains", val: "example.com" }],
});
assert(f1.includes("op%5Bemail%5D=contains"), `filter op: got "${f1}"`);
assert(f1.includes("val%5Bemail%5D=example.com"), `filter val: got "${f1}"`);

// Filter without val (e.g. is_null)
const f2 = encodeBrowseParams({
  filters: [{ column: "deleted_at", op: "is_null" }],
});
assert(f2.includes("op%5Bdeleted_at%5D=is_null"), `is_null op: got "${f2}"`);
assert(!f2.includes("val%5B"), `is_null should have no val: got "${f2}"`);

// --- round-trip ---

const original = {
  limit: 25,
  offset: 50,
  sorts: [
    { column: "created_at", desc: true },
    { column: "name", desc: false },
  ],
  filters: [
    { column: "email", op: "contains" as const, val: "example.com" },
    { column: "active", op: "is_true" as const },
  ],
};

const encoded = encodeBrowseParams(original);
const decoded = decodeBrowseParams(encoded);

assert(decoded.limit === 25, `round-trip limit: got ${decoded.limit}`);
assert(decoded.offset === 50, `round-trip offset: got ${decoded.offset}`);
assert(deepEqual(decoded.sorts, original.sorts), `round-trip sorts: got ${JSON.stringify(decoded.sorts)}`);
assert(
  decoded.filters.length === 2,
  `round-trip filter count: got ${decoded.filters.length}`
);

// Decode with leading "?"
const decoded2 = decodeBrowseParams("?" + encoded);
assert(decoded2.limit === 25, "decode with leading ?");

// Empty string returns defaults
const empty = decodeBrowseParams("");
assert(empty.limit === 50, "empty → default limit 50");
assert(empty.offset === 0, "empty → default offset 0");
assert(empty.filters.length === 0, "empty → no filters");
assert(empty.sorts.length === 0, "empty → no sorts");

console.log("All url.test.ts assertions passed.");
