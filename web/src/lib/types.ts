// Wire cell envelope — every row cell is wrapped in one of these.
export type TypeHint =
  | "text" | "int" | "float" | "numeric" | "bool"
  | "date" | "timestamp" | "timestamptz"
  | "uuid" | "jsonb" | "json" | "bytea" | "interval"
  | "enum" | "array"
  | "tsvector" | "xml" | "oid" | "bit" | "inet" | "cidr" | "macaddr"
  | "range" | "money" | "geometry" | "unknown"
  | "";

export interface WireCell {
  v: unknown;  // JSON-representable value, or null
  t: TypeHint;
}

// Schema shape from GET /api/schema
export interface Schema {
  tables: Table[];
  enums: EnumType[];
}

export interface Table {
  schema: string;
  name: string;
  kind: "r" | "v" | "m" | "p";
  editable: boolean;
  editable_reason: string | null;
  estimated_rows: number;
  columns: Column[];
  primary_key: string[];
  unique_constraints: string[][];
  foreign_keys: ForeignKey[];
}

export interface Column {
  name: string;
  ordinal: number;
  type: string;
  nullable: boolean;
  default: string | null;
  is_identity: boolean;
  is_generated: boolean;
  editor: TypeHint;
  enum_name: string | null;
}

export interface ForeignKey {
  name: string;
  columns: string[];
  references: {
    schema: string;
    table: string;
    columns: string[];
  };
  on_delete: string;
  on_update: string;
}

export interface EnumType {
  schema: string;
  name: string;
  values: string[];
}

// Discover response
export interface DiscoverInfo {
  source: "env" | "prisma" | "drizzle" | "compose" | "dbseer-config" | "flag" | "none";
  path: string;
  host: string;
  port: number;
  database: string;
  user: string;
  readonly: boolean;
  project_root: string;
}

// Filter + sort state
export type FilterOp =
  | "contains" | "equals" | "starts_with" | "ends_with"
  | "eq" | "ne" | "lt" | "lte" | "gt" | "gte"
  | "is_true" | "is_false" | "is_null" | "is_not_null"
  | "in";

export interface Filter {
  column: string;
  op: FilterOp;
  val?: string;
}

export interface Sort {
  column: string;
  desc: boolean;
}

// Browse response
export interface BrowseResponse {
  columns: ResultColumn[];
  rows: WireCell[][];
  page: { limit: number; offset: number; total: number; is_estimated?: boolean };
  sort: { column: string; dir: "asc" | "desc" }[];
  filters: { column: string; op: string; val: string }[];
}

export interface ResultColumn {
  name: string;
  type: string;
  editor: TypeHint;
}

// History entry
export interface HistoryEntry {
  ts: string;
  op: string;
  table: string;
  affected: number;
  sql: string;
  params?: unknown[];
}

// FK target response
export interface FKTarget {
  schema: string;
  table: string;
  pk: string[];
  filter: Record<string, { op: string; val: string }>;
}

// Error envelope
export interface ApiErrorBody {
  code: string;
  message: string;
  detail?: unknown;
}
