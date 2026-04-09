import { useState, useCallback } from "react";
import {
  useReactTable,
  getCoreRowModel,
  flexRender,
  createColumnHelper,
} from "@tanstack/react-table";
import type { ColumnDef, SortingState } from "@tanstack/react-table";
import type {
  Table as TableSchema,
  BrowseResponse,
  Filter,
  Sort,
  WireCell,
  TypeHint,
  EnumType,
} from "../lib/types";
import CellEditor from "./CellEditor";
import FkLink from "./FkLink";
import SortChips from "./SortChips";
import FilterBar from "./FilterBar";

export interface DataGridProps {
  table: TableSchema;
  data: BrowseResponse;
  loading: boolean;
  filters: Filter[];
  sorts: Sort[];
  page: { limit: number; offset: number };
  enums: EnumType[];
  onFiltersChange: (f: Filter[]) => void;
  onSortsChange: (s: Sort[]) => void;
  onPageChange: (p: { limit: number; offset: number }) => void;
  onEditCell: (rowIndex: number, column: string, newValue: WireCell) => void;
  onDeleteRow: (rowIndex: number) => void;
  onAddRow: () => void;
}

interface CellErrorState {
  rowIndex: number;
  column: string;
  message: string;
}

interface CellSavingState {
  rowIndex: number;
  column: string;
}

function formatCellDisplay(cell: WireCell): string {
  if (cell.v === null) return "";
  const v = cell.v;
  if (typeof v === "string") return v.length > 80 ? v.slice(0, 80) + "…" : v;
  if (typeof v === "number" || typeof v === "boolean") return String(v);
  return JSON.stringify(v);
}

function isRightAligned(t: TypeHint): boolean {
  return t === "int" || t === "float" || t === "numeric" || t === "money";
}

interface CellDisplayProps {
  cell: WireCell;
  colName: string;
  tableSchema: TableSchema;
  schema: string;
  table: string;
  hasError: boolean;
  errorMsg?: string;
  isSaving: boolean;
  onDoubleClick: () => void;
}

function CellDisplay({
  cell,
  colName,
  tableSchema,
  schema,
  table,
  hasError,
  errorMsg,
  isSaving,
  onDoubleClick,
}: CellDisplayProps) {
  // Check if this column is an FK column
  const fk = tableSchema.foreign_keys.find((fk) => fk.columns.includes(colName));

  if (isSaving) {
    return (
      <span className="text-slate-400 text-xs">
        <span className="animate-spin inline-block">⟳</span>
      </span>
    );
  }

  if (cell.v === null) {
    return (
      <span
        className={`italic text-slate-400 text-xs cursor-pointer select-none ${hasError ? "border border-red-400 rounded" : ""}`}
        title={errorMsg}
        onDoubleClick={onDoubleClick}
      >
        NULL
      </span>
    );
  }

  const display = formatCellDisplay(cell);

  if (fk && cell.v !== null) {
    return (
      <span
        className={`cursor-pointer select-none ${hasError ? "border border-red-400 rounded" : ""}`}
        onDoubleClick={onDoubleClick}
        title={errorMsg}
      >
        <FkLink
          schema={schema}
          table={table}
          col={colName}
          val={cell}
          display={display}
          tableSchema={tableSchema}
        />
      </span>
    );
  }

  const t = cell.t;

  if (t === "bool") {
    return (
      <span
        className={`cursor-pointer select-none ${hasError ? "border border-red-400 rounded px-0.5" : ""}`}
        title={errorMsg}
        onDoubleClick={onDoubleClick}
      >
        {cell.v ? "✓" : "✗"}
      </span>
    );
  }

  if (t === "jsonb" || t === "json") {
    return (
      <JsonCell cell={cell} hasError={hasError} errorMsg={errorMsg} onDoubleClick={onDoubleClick} />
    );
  }

  if (t === "bytea") {
    const byteStr = typeof cell.v === "string" ? cell.v : "";
    const byteLen = byteStr.startsWith("\\x") ? (byteStr.length - 2) / 2 : byteStr.length;
    return (
      <span
        className="text-slate-400 italic text-xs cursor-pointer"
        title={`${byteLen} bytes`}
        onDoubleClick={onDoubleClick}
      >
        {"<binary>"}
      </span>
    );
  }

  const rightAlign = isRightAligned(t);
  const mono = t === "numeric" || t === "money";

  return (
    <span
      className={`cursor-pointer select-none truncate block ${rightAlign ? "text-right" : ""} ${mono ? "font-mono" : ""} ${hasError ? "border border-red-400 rounded px-0.5" : ""}`}
      title={errorMsg ?? (display.length > 80 ? formatCellDisplay(cell) : undefined)}
      onDoubleClick={onDoubleClick}
    >
      {display}
    </span>
  );
}

function JsonCell({
  cell,
  hasError,
  errorMsg,
  onDoubleClick,
}: {
  cell: WireCell;
  hasError: boolean;
  errorMsg?: string;
  onDoubleClick: () => void;
}) {
  const [open, setOpen] = useState(false);
  return (
    <span className="relative">
      <button
        className={`text-xs text-slate-500 hover:text-slate-700 cursor-pointer ${hasError ? "border border-red-400 rounded px-0.5" : ""}`}
        title={errorMsg}
        onClick={() => setOpen((o) => !o)}
        onDoubleClick={onDoubleClick}
      >
        {"{…}"}
      </button>
      {open && (
        <div className="absolute left-0 top-full z-10 bg-white border border-slate-200 rounded shadow-lg p-2 w-64 max-h-48 overflow-auto">
          <pre className="text-[10px] font-mono text-slate-700 whitespace-pre-wrap">
            {JSON.stringify(cell.v, null, 2)}
          </pre>
          <button
            onClick={() => setOpen(false)}
            className="absolute top-1 right-1 text-slate-400 hover:text-slate-700 text-xs"
          >
            ×
          </button>
        </div>
      )}
    </span>
  );
}

const PAGE_SIZES = [25, 50, 100, 250];

type RowData = WireCell[];

export default function DataGrid({
  table,
  data,
  loading,
  filters,
  sorts,
  page,
  enums,
  onFiltersChange,
  onSortsChange,
  onPageChange,
  onEditCell,
  onDeleteRow,
  onAddRow,
}: DataGridProps) {
  const [editingCell, setEditingCell] = useState<{ rowIndex: number; colName: string } | null>(null);
  const [cellErrors, setCellErrors] = useState<CellErrorState[]>([]);
  const [savingCells, setSavingCells] = useState<CellSavingState[]>([]);
  const [contextMenu, setContextMenu] = useState<{ x: number; y: number; rowIndex: number } | null>(null);

  const colHelper = createColumnHelper<RowData>();

  const columnDefs: ColumnDef<RowData, WireCell>[] = data.columns.map((col, colIdx) => {
    const tableCol = table.columns.find((c) => c.name === col.name);

    return colHelper.accessor((row) => row[colIdx], {
      id: col.name,
      header: col.name,
      cell: (info) => {
        const rowIndex = info.row.index;
        const cell = info.getValue() as WireCell;
        const isEditing = editingCell?.rowIndex === rowIndex && editingCell?.colName === col.name;
        const errState = cellErrors.find((e) => e.rowIndex === rowIndex && e.column === col.name);
        const savState = savingCells.find((s) => s.rowIndex === rowIndex && s.column === col.name);

        if (isEditing && table.editable) {
          return (
            <CellEditor
              cell={cell}
              nullable={tableCol?.nullable ?? true}
              enums={enums}
              enumName={tableCol?.enum_name ?? null}
              onCommit={(newVal) => {
                setEditingCell(null);
                setSavingCells((prev) => [...prev, { rowIndex, column: col.name }]);
                setCellErrors((prev) => prev.filter((e) => !(e.rowIndex === rowIndex && e.column === col.name)));
                onEditCell(rowIndex, col.name, newVal);
                // Saving state cleared externally via prop effect
                setSavingCells((prev) => prev.filter((s) => !(s.rowIndex === rowIndex && s.column === col.name)));
              }}
              onCancel={() => setEditingCell(null)}
            />
          );
        }

        return (
          <CellDisplay
            cell={cell}
            colName={col.name}
            tableSchema={table}
            schema={table.schema}
            table={table.name}
            hasError={!!errState}
            errorMsg={errState?.message}
            isSaving={!!savState}
            onDoubleClick={() => {
              if (table.editable) {
                setCellErrors((prev) => prev.filter((e) => !(e.rowIndex === rowIndex && e.column === col.name)));
                setEditingCell({ rowIndex, colName: col.name });
              }
            }}
          />
        );
      },
    });
  });

  // Add row-actions column
  const actionsCol: ColumnDef<RowData, unknown> = {
    id: "__actions__",
    header: "",
    size: 32,
    cell: (info) => {
      const rowIndex = info.row.index;
      return (
        <button
          onClick={(e) => {
            e.stopPropagation();
            setContextMenu({ x: e.clientX, y: e.clientY, rowIndex });
          }}
          className="text-slate-300 hover:text-slate-600 text-sm px-1"
          title="Row actions"
        >
          ⋯
        </button>
      );
    },
  };

  const allCols: ColumnDef<RowData, unknown>[] = table.editable
    ? [...(columnDefs as ColumnDef<RowData, unknown>[]), actionsCol]
    : (columnDefs as ColumnDef<RowData, unknown>[]);

  const tanSorts: SortingState = sorts.map((s) => ({ id: s.column, desc: s.desc }));

  const reactTable = useReactTable<RowData>({
    data: data.rows,
    columns: allCols,
    getCoreRowModel: getCoreRowModel(),
    manualPagination: true,
    manualFiltering: true,
    manualSorting: true,
    rowCount: data.page.total,
    state: {
      sorting: tanSorts,
      pagination: { pageIndex: Math.floor(page.offset / page.limit), pageSize: page.limit },
    },
    onSortingChange: (updater) => {
      const next = typeof updater === "function" ? updater(tanSorts) : updater;
      onSortsChange(next.map((s) => ({ column: s.id, desc: s.desc })));
    },
    onPaginationChange: (updater) => {
      const prev = { pageIndex: Math.floor(page.offset / page.limit), pageSize: page.limit };
      const next = typeof updater === "function" ? updater(prev) : updater;
      onPageChange({ limit: next.pageSize, offset: next.pageIndex * next.pageSize });
    },
  });

  const totalPages = Math.max(1, Math.ceil(data.page.total / page.limit));
  const currentPage = Math.floor(page.offset / page.limit) + 1;

  const removeSortColumn = useCallback(
    (col: string) => {
      onSortsChange(sorts.filter((s) => s.column !== col));
    },
    [sorts, onSortsChange]
  );

  const handleHeaderClick = useCallback(
    (colName: string, e: React.MouseEvent) => {
      const existing = sorts.find((s) => s.column === colName);
      if (e.shiftKey) {
        if (!existing) {
          onSortsChange([...sorts, { column: colName, desc: false }]);
        } else if (!existing.desc) {
          onSortsChange(sorts.map((s) => (s.column === colName ? { ...s, desc: true } : s)));
        } else {
          onSortsChange(sorts.filter((s) => s.column !== colName));
        }
      } else {
        if (!existing) {
          onSortsChange([{ column: colName, desc: false }]);
        } else if (!existing.desc) {
          onSortsChange([{ column: colName, desc: true }]);
        } else {
          onSortsChange([]);
        }
      }
    },
    [sorts, onSortsChange]
  );

  return (
    <div className="flex flex-col h-full overflow-hidden">
      {/* No-PK / readonly banner */}
      {!table.editable && (
        <div className="bg-yellow-50 border-b border-yellow-200 px-4 py-2 text-xs text-yellow-800 shrink-0">
          {table.editable_reason === "no_primary_key"
            ? "This table has no primary key — editing is disabled."
            : table.editable_reason === "is_view"
            ? "This is a view — editing is disabled."
            : table.editable_reason === "is_matview"
            ? "This is a materialized view — editing is disabled."
            : `Editing is disabled: ${table.editable_reason ?? "unknown reason"}.`}
        </div>
      )}

      {/* Filter bar */}
      <FilterBar
        columns={data.columns.map((c) => ({ name: c.name, editor: c.editor }))}
        filters={filters}
        onFiltersChange={onFiltersChange}
      />

      {/* Sort chips */}
      {sorts.length > 0 && (
        <div className="px-2 py-1 border-b border-slate-100 shrink-0">
          <SortChips sorts={sorts} onRemove={removeSortColumn} />
        </div>
      )}

      {/* Add row button */}
      {table.editable && (
        <div className="px-2 py-1 border-b border-slate-100 shrink-0">
          <button
            onClick={onAddRow}
            className="text-xs px-2 py-1 bg-blue-600 text-white rounded hover:bg-blue-700"
          >
            + Add row
          </button>
        </div>
      )}

      {/* Table */}
      <div className="flex-1 overflow-auto">
        {loading && (
          <div className="absolute inset-0 bg-white/60 flex items-center justify-center z-10 pointer-events-none">
            <span className="text-slate-400 text-sm">Loading…</span>
          </div>
        )}
        <table className="w-full text-xs border-collapse">
          <thead className="sticky top-0 bg-slate-50 z-10">
            {reactTable.getHeaderGroups().map((hg) => (
              <tr key={hg.id}>
                {hg.headers.map((header) => {
                  const colName = header.id;
                  const sort = sorts.find((s) => s.column === colName);
                  const isActions = colName === "__actions__";
                  return (
                    <th
                      key={header.id}
                      className={`px-2 py-1.5 text-left font-medium text-slate-600 border-b border-slate-200 whitespace-nowrap ${
                        !isActions ? "cursor-pointer hover:bg-slate-100 select-none" : ""
                      }`}
                      onClick={isActions ? undefined : (e) => handleHeaderClick(colName, e)}
                      style={{ width: isActions ? 32 : undefined }}
                    >
                      {isActions ? null : (
                        <span className="flex items-center gap-1">
                          {flexRender(header.column.columnDef.header, header.getContext())}
                          {sort ? (sort.desc ? " ↓" : " ↑") : <span className="text-slate-300"> ↕</span>}
                        </span>
                      )}
                    </th>
                  );
                })}
              </tr>
            ))}
          </thead>
          <tbody>
            {reactTable.getRowModel().rows.length === 0 ? (
              <tr>
                <td
                  colSpan={allCols.length}
                  className="px-4 py-8 text-center text-slate-400 text-sm"
                >
                  No rows match these filters.
                </td>
              </tr>
            ) : (
              reactTable.getRowModel().rows.map((row) => (
                <tr
                  key={row.id}
                  className="hover:bg-slate-50 border-b border-slate-100 group"
                  onContextMenu={(e) => {
                    if (!table.editable) return;
                    e.preventDefault();
                    setContextMenu({ x: e.clientX, y: e.clientY, rowIndex: row.index });
                  }}
                >
                  {row.getVisibleCells().map((cell) => (
                    <td
                      key={cell.id}
                      className="px-2 py-1 max-w-[200px] overflow-hidden"
                    >
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </td>
                  ))}
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      <div className="shrink-0 border-t border-slate-200 px-4 py-2 flex items-center gap-3 bg-white text-xs">
        <button
          onClick={() => onPageChange({ limit: page.limit, offset: Math.max(0, page.offset - page.limit) })}
          disabled={page.offset === 0}
          className="px-2 py-1 border border-slate-200 rounded disabled:opacity-40 hover:bg-slate-50"
        >
          ← Prev
        </button>
        <span className="text-slate-600">
          Page {currentPage} of {totalPages}
        </span>
        <button
          onClick={() => onPageChange({ limit: page.limit, offset: page.offset + page.limit })}
          disabled={currentPage >= totalPages}
          className="px-2 py-1 border border-slate-200 rounded disabled:opacity-40 hover:bg-slate-50"
        >
          Next →
        </button>
        <span className="ml-auto flex items-center gap-2 text-slate-500">
          <span>Rows per page:</span>
          <select
            value={page.limit}
            onChange={(e) => onPageChange({ limit: Number(e.target.value), offset: 0 })}
            className="text-xs border border-slate-200 rounded px-1 py-0.5"
          >
            {PAGE_SIZES.map((s) => (
              <option key={s} value={s}>{s}</option>
            ))}
          </select>
          <span className="text-slate-400">
            {data.page.total.toLocaleString()} total rows
          </span>
        </span>
      </div>

      {/* Context menu */}
      {contextMenu && (
        <>
          <div
            className="fixed inset-0 z-40"
            onClick={() => setContextMenu(null)}
          />
          <div
            className="fixed z-50 bg-white border border-slate-200 rounded shadow-lg py-1"
            style={{ top: contextMenu.y, left: contextMenu.x }}
          >
            <button
              onClick={() => {
                onDeleteRow(contextMenu.rowIndex);
                setContextMenu(null);
              }}
              className="block w-full text-left px-4 py-1.5 text-xs text-red-600 hover:bg-red-50"
            >
              Delete row
            </button>
          </div>
        </>
      )}
    </div>
  );
}
