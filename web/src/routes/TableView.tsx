import { useParams, useSearchParams } from "react-router-dom";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useState, useCallback } from "react";
import { api, queryKeys, ApiError } from "../lib/api";
import { decodeBrowseParams, encodeBrowseParams } from "../lib/url";
import type { Filter, Sort, WireCell } from "../lib/types";
import DataGrid from "../components/DataGrid";
import ConfirmUnscoped from "../components/ConfirmUnscoped";

interface UnscopedState {
  count: number;
  sql?: string;
  action: () => Promise<void>;
}

export default function TableView() {
  const { schemaTable } = useParams<{ schemaTable: string }>();
  const [searchParams, setSearchParams] = useSearchParams();
  const queryClient = useQueryClient();

  const [unscopedState, setUnscopedState] = useState<UnscopedState | null>(null);

  // Parse schema/table from combined param
  const dotIdx = (schemaTable ?? "").indexOf(".");
  const schema = dotIdx !== -1 ? (schemaTable ?? "").slice(0, dotIdx) : "public";
  const tableName = dotIdx !== -1 ? (schemaTable ?? "").slice(dotIdx + 1) : (schemaTable ?? "");

  // Decode URL state
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

  // Schema query
  const schemaQuery = useQuery({
    queryKey: queryKeys.schema,
    queryFn: () => api.schema(),
  });

  const tableSchema = schemaQuery.data?.tables.find(
    (t) => t.schema === schema && t.name === tableName
  );

  // Browse query
  const browseOpts = { limit, offset, filters, sorts };
  const browseQuery = useQuery({
    queryKey: queryKeys.browse(schema, tableName, browseOpts),
    queryFn: () => api.browse(schema, tableName, browseOpts),
    enabled: !!tableName,
  });

  const invalidateBrowse = () => {
    void queryClient.invalidateQueries({ queryKey: ["browse", schema, tableName] });
  };

  // Build where clause from PK for a given row
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

  // Update mutation
  const updateMutation = useMutation({
    mutationFn: async ({
      where,
      values,
      confirm,
    }: {
      where: Record<string, WireCell>;
      values: Record<string, WireCell>;
      confirm?: number;
    }) => {
      return api.updateRow(schema, tableName, where, values, confirm);
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
            await api.updateRow(schema, tableName, variables.where, variables.values, count);
            invalidateBrowse();
          },
        });
      }
    },
  });

  // Delete mutation
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
      }
    },
  });

  const handleEditCell = useCallback(
    (rowIndex: number, column: string, newValue: WireCell) => {
      const where = buildPkWhere(rowIndex);
      if (!where) return;
      updateMutation.mutate({ where, values: { [column]: newValue } });
    },
    [buildPkWhere, updateMutation]
  );

  const handleDeleteRow = useCallback(
    (rowIndex: number) => {
      const where = buildPkWhere(rowIndex);
      if (!where) return;
      deleteMutation.mutate({ where });
    },
    [buildPkWhere, deleteMutation]
  );

  const handleAddRow = useCallback(async () => {
    // For v0.1, insert an empty row with defaults
    try {
      await api.insertRow(schema, tableName, {});
      invalidateBrowse();
    } catch {
      // Errors shown via alert for now — v0.1
      alert("Failed to add row. Fill in required columns.");
    }
  }, [schema, tableName]); // eslint-disable-line react-hooks/exhaustive-deps

  if (schemaQuery.isLoading) {
    return <div className="p-6 text-sm text-slate-400">Loading schema…</div>;
  }

  if (!tableSchema) {
    return (
      <div className="p-6 text-sm text-slate-500">
        Table <strong>{schemaTable}</strong> not found in schema.
      </div>
    );
  }

  if (browseQuery.isError) {
    return (
      <div className="p-6 text-sm text-red-500">
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

  return (
    <div className="flex flex-col h-full overflow-hidden">
      <div className="px-4 py-2 border-b border-slate-200 bg-white shrink-0 flex items-center gap-2">
        <h1 className="text-sm font-semibold text-slate-800">
          {schema}.{tableName}
        </h1>
        <span className="text-xs text-slate-400">
          ~{tableSchema.estimated_rows.toLocaleString()} rows estimated
        </span>
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
