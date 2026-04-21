import { useParams, useSearchParams } from "react-router-dom";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useState, useCallback, useEffect } from "react";
import { api, queryKeys, ApiError } from "../lib/api";
import { decodeBrowseParams, encodeBrowseParams } from "../lib/url";
import type { BrowseResponse, Filter, Sort, Table, WireCell } from "../lib/types";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import DataGrid from "../components/DataGrid";
import ConfirmUnscoped from "../components/ConfirmUnscoped";
import InsertRowDialog from "../components/InsertRowDialog";
import { Database, KeyRound, RefreshCw, ShieldCheck } from "lucide-react";

interface UnscopedState {
  count: number;
  sql?: string;
  action: () => Promise<void>;
}

function cellKey(rowIndex: number, column: string) {
  return `${rowIndex}:${column}`;
}

function cellValuesEqual(a: WireCell | undefined, b: WireCell): boolean {
  if (!a) return false;
  if (a.v === b.v) return true;
  if (a.v === null || b.v === null) return false;
  if (typeof a.v === "object" && typeof b.v === "object") {
    return JSON.stringify(a.v) === JSON.stringify(b.v);
  }
  return false;
}

function editableKeyColumns(table: Table | undefined): string[] {
  if (!table) return [];
  return table.edit_key;
}

function rowMatchesWhere(
  row: WireCell[],
  columns: BrowseResponse["columns"],
  where: Record<string, WireCell>
): boolean {
  return Object.entries(where).every(([column, expected]) => {
    const colIdx = columns.findIndex((c) => c.name === column);
    return colIdx !== -1 && cellValuesEqual(row[colIdx], expected);
  });
}

function replaceCachedRow(
  data: BrowseResponse,
  where: Record<string, WireCell>,
  updatedRow: WireCell[]
): BrowseResponse | null {
  const rowIndex = data.rows.findIndex((row) => rowMatchesWhere(row, data.columns, where));
  if (rowIndex === -1) return null;

  const rows = data.rows.slice();
  rows[rowIndex] = updatedRow;
  return { ...data, rows };
}

function removeCachedRow(
  data: BrowseResponse,
  where: Record<string, WireCell>
): BrowseResponse | null {
  const rowIndex = data.rows.findIndex((row) => rowMatchesWhere(row, data.columns, where));
  if (rowIndex === -1) return null;

  const rows = data.rows.slice();
  rows.splice(rowIndex, 1);
  return {
    ...data,
    rows,
    page: {
      ...data.page,
      total: Math.max(0, data.page.total - 1),
    },
  };
}

function formatUpdatedAt(timestamp: number): string {
  return new Intl.DateTimeFormat(undefined, {
    hour: "numeric",
    minute: "2-digit",
    second: "2-digit",
  }).format(timestamp);
}

export default function TableView() {
  const { schemaTable } = useParams<{ schemaTable: string }>();
  const [searchParams, setSearchParams] = useSearchParams();
  const queryClient = useQueryClient();

  const [unscopedState, setUnscopedState] = useState<UnscopedState | null>(null);
  const [insertOpen, setInsertOpen] = useState(false);
  const [insertSaving, setInsertSaving] = useState(false);

  const [savingCells, setSavingCells] = useState<Set<string>>(new Set());
  const [cellErrors, setCellErrors] = useState<Map<string, string>>(new Map());

  const dotIdx = (schemaTable ?? "").indexOf(".");
  const schema = dotIdx !== -1 ? (schemaTable ?? "").slice(0, dotIdx) : "public";
  const tableName = dotIdx !== -1 ? (schemaTable ?? "").slice(dotIdx + 1) : (schemaTable ?? "");

  const { limit, offset, filters, sorts } = decodeBrowseParams(searchParams.toString());

  const setUrlState = useCallback(
    (updates: { filters?: Filter[]; sorts?: Sort[]; limit?: number; offset?: number }) => {
      const newFilters = updates.filters ?? filters;
      const newSorts = updates.sorts ?? sorts;
      const newLimit = updates.limit ?? limit;
      const newOffset = updates.offset ?? offset;

      const qs = encodeBrowseParams({
        limit: newLimit,
        offset: newOffset,
        filters: newFilters,
        sorts: newSorts,
      });
      setSearchParams(qs ? Object.fromEntries(new URLSearchParams(qs)) : {}, { replace: true });
    },
    [filters, sorts, limit, offset, setSearchParams]
  );

  const discoverQuery = useQuery({
    queryKey: queryKeys.discover,
    queryFn: () => api.discover(),
    staleTime: 30_000,
  });
  const colSizingKey = discoverQuery.data
    ? `dbseer:colWidths:${discoverQuery.data.host}:${discoverQuery.data.port}:${discoverQuery.data.database}:${schema}.${tableName}`
    : undefined;

  const schemaQuery = useQuery({
    queryKey: queryKeys.schema,
    queryFn: () => api.schema(),
    staleTime: 30_000,
  });

  const tableSchema = schemaQuery.data?.tables.find(
    (t) => t.schema === schema && t.name === tableName
  );

  const browseOpts = { limit, offset, filters, sorts };
  const browseQuery = useQuery({
    queryKey: queryKeys.browse(schema, tableName, browseOpts),
    queryFn: () => api.browse(schema, tableName, browseOpts),
    enabled: !!tableName,
    placeholderData: (previousData) => previousData,
  });

  useEffect(() => {
    setCellErrors(new Map());
    setSavingCells(new Set());
  }, [browseQuery.data]);

  const invalidateBrowse = useCallback(() => {
    void queryClient.invalidateQueries({ queryKey: ["browse", schema, tableName] });
  }, [queryClient, schema, tableName]);

  const patchBrowseCaches = useCallback(
    (updater: (data: BrowseResponse) => BrowseResponse | null): boolean => {
      let changed = false;
      const entries = queryClient.getQueriesData<BrowseResponse>({
        queryKey: ["browse", schema, tableName],
      });

      for (const [key, value] of entries) {
        if (!value) continue;
        const next = updater(value);
        if (!next) continue;
        changed = true;
        queryClient.setQueryData(key, next);
      }

      return changed;
    },
    [queryClient, schema, tableName]
  );

  const buildRowWhere = useCallback(
    (rowIndex: number): Record<string, WireCell> | null => {
      if (!tableSchema || !browseQuery.data) return null;
      const keyColumns = editableKeyColumns(tableSchema);
      if (keyColumns.length === 0) return null;
      const row = browseQuery.data.rows[rowIndex];
      if (!row) return null;
      const where: Record<string, WireCell> = {};
      for (const keyColumn of keyColumns) {
        const colIdx = browseQuery.data.columns.findIndex((c) => c.name === keyColumn);
        if (colIdx === -1) return null;
        where[keyColumn] = row[colIdx];
      }
      return where;
    },
    [tableSchema, browseQuery.data]
  );

  const handleEditCell = useCallback(
    async (rowIndex: number, column: string, newValue: WireCell) => {
      const where = buildRowWhere(rowIndex);
      if (!where) return;

      const colIdx = browseQuery.data?.columns.findIndex((c) => c.name === column);
      const currentCell =
        colIdx !== undefined && colIdx >= 0
          ? browseQuery.data?.rows[rowIndex]?.[colIdx]
          : undefined;
      if (cellValuesEqual(currentCell, newValue)) return;

      const key = cellKey(rowIndex, column);
      setSavingCells((prev) => new Set(prev).add(key));
      setCellErrors((prev) => {
        const next = new Map(prev);
        next.delete(key);
        return next;
      });

      try {
        const rows = await api.updateRow(schema, tableName, where, { [column]: newValue });
        const updatedRow = rows[0];
        const patched =
          updatedRow !== undefined &&
          patchBrowseCaches((data) => replaceCachedRow(data, where, updatedRow));
        if (!patched) {
          invalidateBrowse();
        }
      } catch (err) {
        const msg = err instanceof ApiError ? err.message : "Update failed";
        setCellErrors((prev) => new Map(prev).set(key, msg));
        toast.error(`${column}: ${msg}`, { duration: Infinity });

        if (err instanceof ApiError && err.code === "unscoped_mutation") {
          const detail = err.detail as { affected?: number; sql?: string } | undefined;
          const count = detail?.affected ?? 0;
          setUnscopedState({
            count,
            sql: detail?.sql,
            action: async () => {
              const rows = await api.updateRow(
                schema,
                tableName,
                where,
                { [column]: newValue },
                count
              );
              const updatedRow = rows[0];
              const patched =
                updatedRow !== undefined &&
                patchBrowseCaches((data) => replaceCachedRow(data, where, updatedRow));
              if (!patched) {
                invalidateBrowse();
              }
            },
          });
        }
      } finally {
        setSavingCells((prev) => {
          const next = new Set(prev);
          next.delete(key);
          return next;
        });
      }
    },
    [buildRowWhere, schema, tableName, invalidateBrowse, browseQuery.data, patchBrowseCaches]
  );

  const deleteMutation = useMutation({
    mutationFn: async ({
      where,
      confirm,
    }: {
      where: Record<string, WireCell>;
      confirm?: number;
    }) => {
      return api.deleteRow(schema, tableName, where, confirm);
    },
    onSuccess: (_, variables) => {
      const patched = patchBrowseCaches((data) => removeCachedRow(data, variables.where));
      if (!patched) {
        invalidateBrowse();
      }
    },
    onError: (err, variables) => {
      if (err instanceof ApiError && err.code === "unscoped_mutation") {
        const detail = err.detail as { affected?: number; sql?: string } | undefined;
        const count = detail?.affected ?? 0;
        setUnscopedState({
          count,
          sql: detail?.sql,
          action: async () => {
            await api.deleteRow(schema, tableName, variables.where, count);
            const patched = patchBrowseCaches((data) => removeCachedRow(data, variables.where));
            if (!patched) {
              invalidateBrowse();
            }
          },
        });
      } else {
        toast.error(err instanceof ApiError ? err.message : "Delete failed", {
          duration: Infinity,
        });
      }
    },
  });

  const handleDeleteRow = useCallback(
    (rowIndex: number) => {
      const where = buildRowWhere(rowIndex);
      if (!where) return;
      deleteMutation.mutate({ where });
    },
    [buildRowWhere, deleteMutation]
  );

  const handleAddRow = useCallback(() => {
    setInsertOpen(true);
  }, []);

  const handleInsertRow = useCallback(
    async (values: Record<string, WireCell>) => {
      setInsertSaving(true);
      try {
        await api.insertRow(schema, tableName, values);
        invalidateBrowse();
        toast.success("Row added");
      } catch (err) {
        toast.error(
          err instanceof ApiError ? err.message : "Failed to add row. Fill in required columns.",
          { duration: Infinity }
        );
        throw err;
      } finally {
        setInsertSaving(false);
      }
    },
    [schema, tableName, invalidateBrowse]
  );

  const handleRefresh = useCallback(() => {
    void queryClient.invalidateQueries({ queryKey: queryKeys.schema });
    void queryClient.invalidateQueries({ queryKey: queryKeys.discover });
    invalidateBrowse();
  }, [queryClient, invalidateBrowse]);

  if (schemaQuery.isLoading) {
    return <div className="p-6 text-sm text-muted-foreground">Loading schema...</div>;
  }

  if (!tableSchema) {
    return (
      <div className="p-6 text-sm text-muted-foreground">
        Table <strong className="text-foreground">{schemaTable}</strong> not found in schema.
      </div>
    );
  }

  if (browseQuery.isError) {
    return (
      <div className="p-6 text-sm text-destructive">
        Failed to load rows:{" "}
        {browseQuery.error instanceof ApiError ? browseQuery.error.message : "Unknown error"}
      </div>
    );
  }

  const emptyBrowse = {
    columns: [],
    rows: [],
    page: { limit, offset, total: 0, is_estimated: false },
    sort: [],
    filters: [],
  };

  const browseData = browseQuery.data ?? emptyBrowse;
  const keyModeLabel =
    tableSchema.edit_key.length === 0
      ? "Read-only"
      : tableSchema.primary_key.length > 0
        ? "Primary key edits"
        : "Unique-key edits";
  const rowCountLabel =
    tableSchema.estimated_rows < 0
      ? "Unknown size"
      : `~${tableSchema.estimated_rows.toLocaleString()} rows`;
  const activeStatus = [
    filters.length > 0 ? `${filters.length} filter${filters.length === 1 ? "" : "s"}` : null,
    sorts.length > 0 ? `${sorts.length} sort${sorts.length === 1 ? "" : "s"}` : null,
    browseQuery.isFetching ? "Refreshing" : null,
  ]
    .filter(Boolean)
    .join(" • ");
  const lastUpdatedLabel =
    browseQuery.dataUpdatedAt > 0 ? formatUpdatedAt(browseQuery.dataUpdatedAt) : null;

  return (
    <div className="dbseer-shell flex h-full flex-col overflow-hidden">
      <div className="dbseer-hero shrink-0 border-b border-border px-4 py-4">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
          <div className="space-y-3">
            <div className="flex flex-wrap items-center gap-2">
              <Badge
                variant="outline"
                className="gap-1 border-border/80 bg-background/70 text-[11px] text-muted-foreground"
              >
                <Database className="size-3.5" />
                {discoverQuery.data?.database ?? "database"}
              </Badge>
              <Badge
                variant={tableSchema.editable ? "secondary" : "outline"}
                className="gap-1 text-[11px]"
              >
                <ShieldCheck className="size-3.5" />
                {tableSchema.editable ? "Writable" : "Read-only"}
              </Badge>
              <Badge variant="outline" className="gap-1 text-[11px]">
                <KeyRound className="size-3.5" />
                {keyModeLabel}
              </Badge>
            </div>

            <div>
              <h1 className="text-xl font-semibold tracking-tight text-foreground">
                {schema}.{tableName}
              </h1>
              <p className="mt-1 max-w-3xl text-sm text-muted-foreground">
                {discoverQuery.data
                  ? `${discoverQuery.data.host}:${discoverQuery.data.port} • ${rowCountLabel}`
                  : rowCountLabel}
                {lastUpdatedLabel ? ` • synced ${lastUpdatedLabel}` : ""}
              </p>
            </div>

            {(activeStatus || !tableSchema.editable) && (
              <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                {activeStatus && (
                  <span className="rounded-full border border-border/80 bg-background/80 px-2.5 py-1">
                    {activeStatus}
                  </span>
                )}
                {!tableSchema.editable && (
                  <span className="rounded-full border border-border/80 bg-background/80 px-2.5 py-1">
                    {tableSchema.editable_reason === "no_primary_key"
                      ? "Needs a primary key or unique constraint to edit rows"
                      : "Structure is read-only in dbseer"}
                  </span>
                )}
              </div>
            )}
          </div>

          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={handleRefresh}
              className="gap-1.5 bg-background/80"
              title="Refresh schema and rows"
            >
              <RefreshCw className={`size-3.5 ${browseQuery.isFetching ? "animate-spin" : ""}`} />
              Refresh
            </Button>
          </div>
        </div>
      </div>

      <div className="flex-1 overflow-hidden">
        <DataGrid
          table={tableSchema}
          data={browseData}
          loading={browseQuery.isFetching}
          filters={filters}
          sorts={sorts}
          page={{ limit, offset }}
          enums={schemaQuery.data?.enums ?? []}
          savingCells={savingCells}
          cellErrors={cellErrors}
          colSizingKey={colSizingKey}
          onFiltersChange={(f) => setUrlState({ filters: f, offset: 0 })}
          onSortsChange={(s) => setUrlState({ sorts: s })}
          onPageChange={(p) => setUrlState({ limit: p.limit, offset: p.offset })}
          onEditCell={handleEditCell}
          onDeleteRow={handleDeleteRow}
          onAddRow={handleAddRow}
        />
      </div>

      {unscopedState && (
        <ConfirmUnscoped
          affectedCount={unscopedState.count}
          sql={unscopedState.sql}
          onConfirm={async () => {
            await unscopedState.action();
            setUnscopedState(null);
          }}
          onCancel={() => setUnscopedState(null)}
        />
      )}

      <InsertRowDialog
        open={insertOpen}
        table={tableSchema}
        enums={schemaQuery.data?.enums ?? []}
        saving={insertSaving}
        onClose={() => {
          if (!insertSaving) {
            setInsertOpen(false);
          }
        }}
        onSubmit={handleInsertRow}
      />
    </div>
  );
}
