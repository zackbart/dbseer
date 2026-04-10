import { useParams, useSearchParams } from "react-router-dom";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useState, useCallback, useEffect } from "react";
import { api, queryKeys, ApiError } from "../lib/api";
import { decodeBrowseParams, encodeBrowseParams } from "../lib/url";
import type { Filter, Sort, WireCell } from "../lib/types";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import DataGrid from "../components/DataGrid";
import ConfirmUnscoped from "../components/ConfirmUnscoped";

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

export default function TableView() {
  const { schemaTable } = useParams<{ schemaTable: string }>();
  const [searchParams, setSearchParams] = useSearchParams();
  const queryClient = useQueryClient();

  const [unscopedState, setUnscopedState] = useState<UnscopedState | null>(null);

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
  });
  const colSizingKey = discoverQuery.data
    ? `dbseer:colWidths:${discoverQuery.data.host}:${discoverQuery.data.port}:${discoverQuery.data.database}:${schema}.${tableName}`
    : undefined;

  const schemaQuery = useQuery({
    queryKey: queryKeys.schema,
    queryFn: () => api.schema(),
  });

  const tableSchema = schemaQuery.data?.tables.find(
    (t) => t.schema === schema && t.name === tableName
  );

  const browseOpts = { limit, offset, filters, sorts };
  const browseQuery = useQuery({
    queryKey: queryKeys.browse(schema, tableName, browseOpts),
    queryFn: () => api.browse(schema, tableName, browseOpts),
    enabled: !!tableName,
  });

  useEffect(() => {
    setCellErrors(new Map());
    setSavingCells(new Set());
  }, [browseQuery.data]);

  const invalidateBrowse = useCallback(() => {
    void queryClient.invalidateQueries({ queryKey: ["browse", schema, tableName] });
  }, [queryClient, schema, tableName]);

  const buildPkWhere = useCallback(
    (rowIndex: number): Record<string, WireCell> | null => {
      if (!tableSchema || !browseQuery.data) return null;
      const pk = tableSchema.primary_key;
      if (pk.length === 0) return null;
      const row = browseQuery.data.rows[rowIndex];
      if (!row) return null;
      const where: Record<string, WireCell> = {};
      for (const pkCol of pk) {
        const colIdx = browseQuery.data.columns.findIndex((c) => c.name === pkCol);
        if (colIdx === -1) return null;
        where[pkCol] = row[colIdx];
      }
      return where;
    },
    [tableSchema, browseQuery.data]
  );

  const handleEditCell = useCallback(
    async (rowIndex: number, column: string, newValue: WireCell) => {
      const where = buildPkWhere(rowIndex);
      if (!where) return;

      const colIdx = browseQuery.data?.columns.findIndex((c) => c.name === column);
      const currentCell = colIdx !== undefined && colIdx >= 0 ? browseQuery.data?.rows[rowIndex]?.[colIdx] : undefined;
      if (cellValuesEqual(currentCell, newValue)) return;

      const key = cellKey(rowIndex, column);
      setSavingCells((prev) => new Set(prev).add(key));
      setCellErrors((prev) => {
        const next = new Map(prev);
        next.delete(key);
        return next;
      });

      try {
        await api.updateRow(schema, tableName, where, { [column]: newValue });
        invalidateBrowse();
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
              await api.updateRow(schema, tableName, where, { [column]: newValue }, count);
              invalidateBrowse();
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
    [buildPkWhere, schema, tableName, invalidateBrowse, browseQuery.data]
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
    onSuccess: () => invalidateBrowse(),
    onError: (err, variables) => {
      if (err instanceof ApiError && err.code === "unscoped_mutation") {
        const detail = err.detail as { affected?: number; sql?: string } | undefined;
        const count = detail?.affected ?? 0;
        setUnscopedState({
          count,
          sql: detail?.sql,
          action: async () => {
            await api.deleteRow(schema, tableName, variables.where, count);
            invalidateBrowse();
          },
        });
      } else {
        toast.error(err instanceof ApiError ? err.message : "Delete failed", { duration: Infinity });
      }
    },
  });

  const handleDeleteRow = useCallback(
    (rowIndex: number) => {
      const where = buildPkWhere(rowIndex);
      if (!where) return;
      deleteMutation.mutate({ where });
    },
    [buildPkWhere, deleteMutation]
  );

  const handleAddRow = useCallback(async () => {
    try {
      await api.insertRow(schema, tableName, {});
      invalidateBrowse();
      toast.success("Row added");
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to add row. Fill in required columns.", { duration: Infinity });
    }
  }, [schema, tableName, invalidateBrowse]);

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
        {browseQuery.error instanceof ApiError
          ? browseQuery.error.message
          : "Unknown error"}
      </div>
    );
  }

  const emptyBrowse = {
    columns: [],
    rows: [],
    page: { limit, offset, total: 0 },
    sort: [],
    filters: [],
  };

  const rowCountLabel =
    tableSchema.estimated_rows < 0
      ? "rows: unknown"
      : `~${tableSchema.estimated_rows.toLocaleString()} rows`;

  return (
    <div className="flex flex-col h-full overflow-hidden">
      <div className="px-4 py-2 border-b border-border bg-card shrink-0 flex items-center gap-2">
        <h1 className="text-sm font-semibold text-foreground">
          {schema}.{tableName}
        </h1>
        <span className="text-xs text-muted-foreground">{rowCountLabel}</span>
        <Button
          variant="outline"
          size="sm"
          onClick={handleRefresh}
          className="ml-auto"
          title="Refresh schema and rows"
        >
          Refresh
        </Button>
      </div>

      <div className="flex-1 overflow-hidden">
        <DataGrid
          table={tableSchema}
          data={browseQuery.data ?? emptyBrowse}
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
    </div>
  );
}
