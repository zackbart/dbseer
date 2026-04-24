import { useState, useCallback, useEffect, useMemo } from "react";
import {
  useReactTable,
  getCoreRowModel,
  flexRender,
  createColumnHelper,
} from "@tanstack/react-table";
import type { ColumnDef, SortingState, ColumnSizingState } from "@tanstack/react-table";
import { getJSON, setJSON } from "../lib/storage";
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
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { cn } from "@/lib/utils";
import { ArrowUpDown, Plus, ScanSearch, SlidersHorizontal, Trash2 } from "lucide-react";

export interface DataGridProps {
  table: TableSchema;
  data: BrowseResponse;
  loading: boolean;
  filters: Filter[];
  sorts: Sort[];
  page: { limit: number; offset: number };
  enums: EnumType[];
  savingCells: Set<string>;
  cellErrors: Map<string, string>;
  colSizingKey?: string;
  onFiltersChange: (f: Filter[]) => void;
  onSortsChange: (s: Sort[]) => void;
  onPageChange: (p: { limit: number; offset: number }) => void;
  onEditCell: (rowIndex: number, column: string, newValue: WireCell) => void;
  onDeleteRow: (rowIndex: number) => void;
  onDeleteRows: (rowIndexes: number[]) => void;
  onAddRow: () => void;
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
      <span className="text-muted-foreground text-xs">
        <span className="animate-spin inline-block">⟳</span>
      </span>
    );
  }

  if (cell.v === null) {
    return (
      <span
        className={`italic text-muted-foreground text-xs cursor-pointer select-none ${hasError ? "border border-destructive rounded" : ""}`}
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
        className={`cursor-pointer select-none ${hasError ? "border border-destructive rounded" : ""}`}
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
        className={`cursor-pointer select-none ${hasError ? "border border-destructive rounded px-0.5" : ""}`}
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
        className="text-muted-foreground italic text-xs cursor-pointer"
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
      className={`cursor-pointer select-none truncate block ${rightAlign ? "text-right" : ""} ${mono ? "font-mono" : ""} ${hasError ? "border border-destructive rounded px-0.5" : ""}`}
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
        className={`text-xs text-muted-foreground hover:text-foreground cursor-pointer ${hasError ? "border border-destructive rounded px-0.5" : ""}`}
        title={errorMsg}
        onClick={() => setOpen((o) => !o)}
        onDoubleClick={onDoubleClick}
      >
        {"{…}"}
      </button>
      {open && (
        <div className="absolute left-0 top-full z-10 bg-card border border-border rounded shadow-lg p-2 w-64 max-h-48 overflow-auto">
          <pre className="text-[10px] font-mono text-foreground whitespace-pre-wrap">
            {JSON.stringify(cell.v, null, 2)}
          </pre>
          <button
            onClick={() => setOpen(false)}
            className="absolute top-1 right-1 text-muted-foreground hover:text-foreground text-xs"
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
  savingCells,
  cellErrors,
  colSizingKey,
  onFiltersChange,
  onSortsChange,
  onPageChange,
  onEditCell,
  onDeleteRow,
  onDeleteRows,
  onAddRow,
}: DataGridProps) {
  const [editingCell, setEditingCell] = useState<{ rowIndex: number; colName: string } | null>(
    null
  );
  const [contextMenu, setContextMenu] = useState<{
    x: number;
    y: number;
    rowIndex: number;
    colName?: string;
  } | null>(null);
  const [focusedCell, setFocusedCell] = useState<{ rowIndex: number; colName: string } | null>(
    null
  );
  const [selectedRows, setSelectedRows] = useState<Set<number>>(new Set());
  const [selectionAnchor, setSelectionAnchor] = useState<number | null>(null);
  const [columnSizing, setColumnSizing] = useState<ColumnSizingState>(() =>
    colSizingKey ? getJSON<ColumnSizingState>(colSizingKey, {}) : {}
  );

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
        const ck = `${rowIndex}:${col.name}`;
        const errMsg = cellErrors.get(ck);
        const isSaving = savingCells.has(ck);

        if (isEditing && table.editable) {
          return (
            <CellEditor
              cell={cell}
              nullable={tableCol?.nullable ?? true}
              enums={enums}
              enumName={tableCol?.enum_name ?? null}
              onCommit={(newVal) => {
                setEditingCell(null);
                onEditCell(rowIndex, col.name, newVal);
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
            hasError={!!errMsg}
            errorMsg={errMsg}
            isSaving={isSaving}
            onDoubleClick={() => {
              if (table.editable) {
                setEditingCell({ rowIndex, colName: col.name });
              }
            }}
          />
        );
      },
    });
  });

  const rowNumberCol: ColumnDef<RowData, unknown> = {
    id: "__rownum__",
    header: "#",
    size: 56,
    enableResizing: false,
    cell: (info) => (
      <span className="block text-right text-[11px] font-medium tabular-nums text-muted-foreground">
        {page.offset + info.row.index + 1}
      </span>
    ),
  };

  // Add row-actions column
  const actionsCol: ColumnDef<RowData, unknown> = {
    id: "__actions__",
    header: "",
    size: 40,
    cell: (info) => {
      const rowIndex = info.row.index;
      return (
        <button
          onClick={(e) => {
            e.stopPropagation();
            setContextMenu({ x: e.clientX, y: e.clientY, rowIndex });
          }}
          className="rounded-md px-1.5 py-0.5 text-sm text-muted-foreground/60 transition-colors hover:bg-muted hover:text-foreground"
          title="Row actions"
        >
          ⋯
        </button>
      );
    },
  };

  const allCols: ColumnDef<RowData, unknown>[] = table.editable
    ? [rowNumberCol, ...(columnDefs as ColumnDef<RowData, unknown>[]), actionsCol]
    : [rowNumberCol, ...(columnDefs as ColumnDef<RowData, unknown>[])];

  const tanSorts: SortingState = sorts.map((s) => ({ id: s.column, desc: s.desc }));

  const reactTable = useReactTable<RowData>({
    data: data.rows,
    columns: allCols,
    getCoreRowModel: getCoreRowModel(),
    manualPagination: true,
    manualFiltering: true,
    manualSorting: true,
    enableColumnResizing: true,
    columnResizeMode: "onChange",
    defaultColumn: { minSize: 50 },
    rowCount: data.page.total,
    state: {
      sorting: tanSorts,
      pagination: { pageIndex: Math.floor(page.offset / page.limit), pageSize: page.limit },
      columnSizing,
    },
    onColumnSizingChange: (updater) => {
      const next = typeof updater === "function" ? updater(columnSizing) : updater;
      setColumnSizing(next);
      if (colSizingKey) setJSON(colSizingKey, next);
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
  const visibleRowCount = reactTable.getRowModel().rows.length;
  const systemCols = new Set(["__rownum__", "__actions__"]);
  const selectedRowIndexes = useMemo(
    () => Array.from(selectedRows).sort((a, b) => a - b),
    [selectedRows]
  );

  useEffect(() => {
    setSelectedRows((prev) => {
      const next = new Set(Array.from(prev).filter((idx) => idx >= 0 && idx < data.rows.length));
      return next.size === prev.size ? prev : next;
    });
    setSelectionAnchor((prev) => (prev !== null && prev >= data.rows.length ? null : prev));
  }, [data.rows.length]);

  const removeSortColumn = useCallback(
    (col: string) => {
      onSortsChange(sorts.filter((s) => s.column !== col));
    },
    [sorts, onSortsChange]
  );

  const colNames = data.columns.map((c) => c.name);
  const rowCount = reactTable.getRowModel().rows.length;

  const selectRow = useCallback(
    (rowIndex: number, e: React.MouseEvent) => {
      setSelectedRows((prev) => {
        const next = new Set(prev);

        if (e.shiftKey && selectionAnchor !== null) {
          const start = Math.min(selectionAnchor, rowIndex);
          const end = Math.max(selectionAnchor, rowIndex);
          for (let idx = start; idx <= end; idx += 1) {
            next.add(idx);
          }
          return next;
        }

        if (e.metaKey || e.ctrlKey) {
          if (next.has(rowIndex)) {
            next.delete(rowIndex);
          } else {
            next.add(rowIndex);
          }
          return next;
        }

        return new Set([rowIndex]);
      });
      setSelectionAnchor((prev) => (e.shiftKey && prev !== null ? prev : rowIndex));
    },
    [selectionAnchor]
  );

  const deleteSelectedRows = useCallback(() => {
    if (selectedRowIndexes.length === 0) return;
    onDeleteRows(selectedRowIndexes);
    setSelectedRows(new Set());
    setSelectionAnchor(null);
  }, [onDeleteRows, selectedRowIndexes]);

  const handleGridKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      const tag = (e.target as HTMLElement).tagName;
      if (["INPUT", "SELECT", "TEXTAREA", "BUTTON"].includes(tag)) return;

      if (e.key === "Escape") {
        setEditingCell(null);
        setFocusedCell(null);
        return;
      }

      // Copy focused cell value
      if ((e.metaKey || e.ctrlKey) && e.key === "c" && focusedCell) {
        const colIdx = colNames.indexOf(focusedCell.colName);
        if (colIdx >= 0) {
          const row = data.rows[focusedCell.rowIndex];
          if (row) {
            const cellVal = row[colIdx];
            const text = cellVal ? formatCellDisplay(cellVal) : "";
            void navigator.clipboard.writeText(text);
          }
        }
        e.preventDefault();
        return;
      }

      // Enter to edit focused cell
      if (e.key === "Enter" && focusedCell && table.editable) {
        setEditingCell({ rowIndex: focusedCell.rowIndex, colName: focusedCell.colName });
        e.preventDefault();
        return;
      }

      if (
        (e.key === "Delete" || e.key === "Backspace") &&
        table.editable &&
        selectedRows.size > 0
      ) {
        deleteSelectedRows();
        e.preventDefault();
        return;
      }

      // Arrow navigation
      if (["ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight"].includes(e.key)) {
        e.preventDefault();
        setFocusedCell((prev) => {
          const r = prev?.rowIndex ?? 0;
          const ci = prev ? colNames.indexOf(prev.colName) : 0;
          let nr = r,
            nc = ci;
          if (e.key === "ArrowUp") nr = Math.max(0, r - 1);
          if (e.key === "ArrowDown") nr = Math.min(rowCount - 1, r + 1);
          if (e.key === "ArrowLeft") nc = Math.max(0, ci - 1);
          if (e.key === "ArrowRight") nc = Math.min(colNames.length - 1, ci + 1);
          return { rowIndex: nr, colName: colNames[nc] };
        });
      }
    },
    [
      focusedCell,
      colNames,
      rowCount,
      data.rows,
      table.editable,
      selectedRows.size,
      deleteSelectedRows,
    ]
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
        <div className="shrink-0 border-b border-amber-200 bg-amber-50/90 px-4 py-2 text-xs text-amber-950">
          {table.editable_reason === "no_primary_key"
            ? "This table has no primary key or unique constraint, so row edits are disabled."
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

      <div className="shrink-0 border-b border-border bg-background/70 px-3 py-2 backdrop-blur-sm">
        <div className="flex flex-col gap-2 lg:flex-row lg:items-center lg:justify-between">
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant="outline" className="gap-1 text-[11px]">
              <ScanSearch className="size-3.5" />
              {visibleRowCount.toLocaleString()} visible
            </Badge>
            <Badge variant="outline" className="gap-1 text-[11px]">
              <SlidersHorizontal className="size-3.5" />
              {filters.length} filter{filters.length === 1 ? "" : "s"}
            </Badge>
            <Badge variant="outline" className="gap-1 text-[11px]">
              <ArrowUpDown className="size-3.5" />
              {sorts.length} sort{sorts.length === 1 ? "" : "s"}
            </Badge>
            <span className="text-[11px] text-muted-foreground">
              Shift-click selects rows, Delete removes selected rows.
            </span>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            {table.editable && selectedRows.size > 0 && (
              <>
                <span className="text-[11px] text-muted-foreground">
                  {selectedRows.size.toLocaleString()} selected
                </span>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => {
                    setSelectedRows(new Set());
                    setSelectionAnchor(null);
                  }}
                >
                  Clear selection
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={deleteSelectedRows}
                  className="gap-1.5 text-destructive hover:bg-destructive/10 hover:text-destructive"
                >
                  <Trash2 className="size-3.5" />
                  Delete selected
                </Button>
              </>
            )}
            {filters.length > 0 && (
              <Button size="sm" variant="outline" onClick={() => onFiltersChange([])}>
                Clear filters
              </Button>
            )}
            {sorts.length > 0 && (
              <Button size="sm" variant="outline" onClick={() => onSortsChange([])}>
                Reset sort
              </Button>
            )}
            {table.editable && (
              <Button size="sm" onClick={onAddRow} className="gap-1.5">
                <Plus className="size-3.5" />
                Add row
              </Button>
            )}
          </div>
        </div>

        {sorts.length > 0 && (
          <div className="mt-2">
            <SortChips sorts={sorts} onRemove={removeSortColumn} />
          </div>
        )}
      </div>

      {/* Table */}
      <div
        className="relative flex-1 overflow-auto outline-none"
        tabIndex={0}
        onKeyDown={handleGridKeyDown}
      >
        {loading && (
          <div className="pointer-events-none absolute inset-x-0 top-0 z-10 flex justify-center p-3">
            <div className="rounded-full border border-border bg-background/95 px-3 py-1 text-xs text-muted-foreground shadow-sm backdrop-blur-sm">
              Refreshing rows…
            </div>
          </div>
        )}
        <table className="w-full text-xs border-collapse">
          <thead className="sticky top-0 z-10 bg-muted/95 backdrop-blur-sm">
            {reactTable.getHeaderGroups().map((hg) => (
              <tr key={hg.id}>
                {hg.headers.map((header) => {
                  const colName = header.id;
                  const sort = sorts.find((s) => s.column === colName);
                  const isActions = colName === "__actions__";
                  const isRowNum = colName === "__rownum__";
                  return (
                    <th
                      key={header.id}
                      className={`relative border-b border-border px-2 py-2 text-left font-medium whitespace-nowrap text-muted-foreground ${
                        !systemCols.has(colName)
                          ? "cursor-pointer select-none hover:bg-muted/80"
                          : ""
                      }`}
                      onClick={
                        systemCols.has(colName) ? undefined : (e) => handleHeaderClick(colName, e)
                      }
                      style={{ width: isActions ? 40 : isRowNum ? 56 : header.getSize() }}
                    >
                      {isActions ? null : isRowNum ? (
                        <span className="block text-right">#</span>
                      ) : (
                        <span className="flex items-center gap-1">
                          {flexRender(header.column.columnDef.header, header.getContext())}
                          {sort ? (
                            sort.desc ? (
                              " ↓"
                            ) : (
                              " ↑"
                            )
                          ) : (
                            <span className="text-muted-foreground/40"> ↕</span>
                          )}
                        </span>
                      )}
                      {!systemCols.has(colName) && (
                        <div
                          onMouseDown={header.getResizeHandler()}
                          onTouchStart={header.getResizeHandler()}
                          onClick={(e) => e.stopPropagation()}
                          className={`absolute right-0 top-0 h-full w-1 cursor-col-resize select-none touch-none ${
                            header.column.getIsResizing()
                              ? "bg-primary"
                              : "hover:bg-muted-foreground/30"
                          }`}
                        />
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
                  className="px-4 py-12 text-center text-sm text-muted-foreground"
                >
                  <div className="font-medium text-foreground">No rows match this view.</div>
                  <div className="mt-1">Adjust filters or refresh the table.</div>
                  {filters.length > 0 && (
                    <button
                      onClick={() => onFiltersChange([])}
                      className="mt-2 text-xs text-primary hover:text-primary/80 underline"
                    >
                      Clear all filters
                    </button>
                  )}
                </td>
              </tr>
            ) : (
              reactTable.getRowModel().rows.map((row) => {
                const isSelected = selectedRows.has(row.index);
                return (
                  <tr
                    key={row.id}
                    className={cn(
                      "group border-b border-border/80 hover:bg-muted/35",
                      isSelected && "bg-primary/10 hover:bg-primary/15"
                    )}
                  >
                    {row.getVisibleCells().map((cell) => {
                      const isFocused =
                        focusedCell?.rowIndex === row.index &&
                        focusedCell?.colName === cell.column.id;
                      const isActionsCol = cell.column.id === "__actions__";
                      const isRowNumCol = cell.column.id === "__rownum__";
                      return (
                        <td
                          key={cell.id}
                          className={cn(
                            "overflow-hidden px-2 py-1.5 align-top",
                            isRowNumCol && "bg-muted/30",
                            isFocused && "ring-2 ring-ring ring-inset"
                          )}
                          style={{ width: cell.column.getSize() }}
                          onClick={(e) => {
                            if (table.editable) {
                              selectRow(row.index, e);
                            }
                            if (!isActionsCol && !isRowNumCol) {
                              setFocusedCell({ rowIndex: row.index, colName: cell.column.id });
                            }
                          }}
                          onContextMenu={(e) => {
                            if (!table.editable || isActionsCol || isRowNumCol) return;
                            e.preventDefault();
                            setContextMenu({
                              x: e.clientX,
                              y: e.clientY,
                              rowIndex: row.index,
                              colName: cell.column.id,
                            });
                          }}
                        >
                          {flexRender(cell.column.columnDef.cell, cell.getContext())}
                        </td>
                      );
                    })}
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      <div className="shrink-0 border-t border-border bg-card/90 px-4 py-2 text-xs backdrop-blur-sm">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-center">
          <div className="flex items-center gap-2">
            <button
              onClick={() => onPageChange({ limit: page.limit, offset: 0 })}
              disabled={page.offset === 0}
              className="rounded border border-border px-2 py-1 hover:bg-muted/50 disabled:opacity-40"
              title="First page"
            >
              &laquo;
            </button>
            <button
              onClick={() =>
                onPageChange({ limit: page.limit, offset: Math.max(0, page.offset - page.limit) })
              }
              disabled={page.offset === 0}
              className="rounded border border-border px-2 py-1 hover:bg-muted/50 disabled:opacity-40"
            >
              Prev
            </button>
            <span className="flex items-center gap-1 text-muted-foreground">
              Page
              <Input
                type="number"
                min={1}
                max={totalPages}
                value={currentPage}
                onChange={(e) => {
                  const p = Math.max(1, Math.min(totalPages, Number(e.target.value) || 1));
                  onPageChange({ limit: page.limit, offset: (p - 1) * page.limit });
                }}
                className="h-7 w-12 px-1 text-center text-xs"
              />
              of {totalPages}
            </span>
            <button
              onClick={() => onPageChange({ limit: page.limit, offset: page.offset + page.limit })}
              disabled={currentPage >= totalPages}
              className="rounded border border-border px-2 py-1 hover:bg-muted/50 disabled:opacity-40"
            >
              Next
            </button>
            <button
              onClick={() =>
                onPageChange({ limit: page.limit, offset: (totalPages - 1) * page.limit })
              }
              disabled={currentPage >= totalPages}
              className="rounded border border-border px-2 py-1 hover:bg-muted/50 disabled:opacity-40"
              title="Last page"
            >
              &raquo;
            </button>
          </div>

          <span className="flex items-center gap-2 text-muted-foreground">
            <span>Rows per page:</span>
            <Select
              value={String(page.limit)}
              onValueChange={(v: string | null) => {
                if (v) onPageChange({ limit: Number(v), offset: 0 });
              }}
            >
              <SelectTrigger size="sm" className="h-7 w-auto text-xs">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {PAGE_SIZES.map((s) => (
                  <SelectItem key={s} value={String(s)}>
                    {s}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <span
              className="text-muted-foreground"
              title={
                data.page.is_estimated
                  ? "Estimated from pg_class.reltuples — exact count skipped because the table is large."
                  : undefined
              }
            >
              {data.page.is_estimated ? "~" : ""}
              {data.page.total.toLocaleString()} total rows
            </span>
          </span>

          <span className="text-muted-foreground lg:ml-auto">
            Showing {visibleRowCount.toLocaleString()} row{visibleRowCount === 1 ? "" : "s"} on this
            page
          </span>
        </div>
      </div>

      {/* Context menu */}
      {contextMenu && (
        <>
          <div className="fixed inset-0 z-40" onClick={() => setContextMenu(null)} />
          <div
            className="fixed z-50 bg-card border border-border rounded shadow-lg py-1 min-w-[140px]"
            style={{ top: contextMenu.y, left: contextMenu.x }}
          >
            {contextMenu.colName && (
              <>
                <button
                  onClick={() => {
                    setEditingCell({
                      rowIndex: contextMenu.rowIndex,
                      colName: contextMenu.colName!,
                    });
                    setContextMenu(null);
                  }}
                  className="block w-full text-left px-4 py-1.5 text-xs text-foreground hover:bg-muted"
                >
                  Edit cell
                </button>
                <button
                  onClick={() => {
                    const colIdx = data.columns.findIndex((c) => c.name === contextMenu.colName);
                    if (colIdx >= 0) {
                      const row = data.rows[contextMenu.rowIndex];
                      if (row) {
                        const cellVal = row[colIdx];
                        const text = cellVal ? formatCellDisplay(cellVal) : "";
                        void navigator.clipboard.writeText(text);
                      }
                    }
                    setContextMenu(null);
                  }}
                  className="block w-full text-left px-4 py-1.5 text-xs text-foreground hover:bg-muted"
                >
                  Copy value
                </button>
                <div className="border-t border-border my-0.5" />
              </>
            )}
            <button
              onClick={() => {
                onDeleteRow(contextMenu.rowIndex);
                setContextMenu(null);
              }}
              className="block w-full text-left px-4 py-1.5 text-xs text-destructive hover:bg-destructive/10"
            >
              Delete row
            </button>
          </div>
        </>
      )}
    </div>
  );
}
