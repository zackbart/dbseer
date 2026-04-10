import { useState, useRef, useEffect, useCallback } from "react";
import type { Filter, FilterOp, TypeHint } from "../lib/types";

interface FilterBarProps {
  columns: { name: string; editor: TypeHint }[];
  filters: Filter[];
  onFiltersChange: (f: Filter[]) => void;
}

function operatorsForType(type: TypeHint): FilterOp[] {
  switch (type) {
    case "text":
    case "uuid":
      return ["contains", "equals", "starts_with", "ends_with", "is_null", "is_not_null"];
    case "int":
    case "float":
    case "numeric":
    case "money":
      return ["eq", "ne", "lt", "lte", "gt", "gte", "is_null", "is_not_null"];
    case "date":
    case "timestamp":
    case "timestamptz":
      return ["eq", "ne", "lt", "lte", "gt", "gte", "is_null", "is_not_null"];
    case "bool":
      return ["is_true", "is_false", "is_null"];
    case "enum":
      return ["in", "is_null", "is_not_null"];
    default:
      return ["is_null", "is_not_null"];
  }
}

function valueInputType(type: TypeHint): string {
  if (type === "date") return "date";
  if (type === "timestamp" || type === "timestamptz") return "datetime-local";
  if (type === "int") return "number";
  if (type === "float" || type === "numeric") return "number";
  return "text";
}

const NO_VALUE_OPS: FilterOp[] = ["is_null", "is_not_null", "is_true", "is_false"];

interface FilterPopoverProps {
  colName: string;
  colType: TypeHint;
  existing: Filter | undefined;
  onApply: (f: Filter) => void;
  onClear: () => void;
  onClose: () => void;
}

function FilterPopover({ colName, colType, existing, onApply, onClear, onClose }: FilterPopoverProps) {
  const ops = operatorsForType(colType);
  const [op, setOp] = useState<FilterOp>(existing?.op ?? ops[0]);
  const [val, setVal] = useState(existing?.val ?? "");
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handler(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        onClose();
      }
    }
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [onClose]);

  const needsValue = !NO_VALUE_OPS.includes(op);

  return (
    <div
      ref={ref}
      className="absolute top-full left-0 z-20 mt-1 bg-card border border-border rounded shadow-lg p-3 w-56"
    >
      <div className="text-[11px] font-medium text-muted-foreground mb-2 uppercase tracking-wide">{colName}</div>
      <select
        value={op}
        onChange={(e) => setOp(e.target.value as FilterOp)}
        className="w-full text-xs border border-border rounded px-1 py-1 mb-2"
      >
        {ops.map((o) => (
          <option key={o} value={o}>{o}</option>
        ))}
      </select>
      {needsValue && (
        <input
          type={valueInputType(colType)}
          value={val}
          onChange={(e) => setVal(e.target.value)}
          className="w-full text-xs border border-border rounded px-1 py-1 mb-2"
          placeholder="value"
          autoFocus
        />
      )}
      <div className="flex gap-1 justify-end">
        <button
          onClick={onClear}
          className="text-[11px] px-2 py-1 text-muted-foreground border border-border rounded hover:bg-muted"
        >
          Clear
        </button>
        <button
          onClick={() => {
            onApply({ column: colName, op, val: needsValue ? val : undefined });
            onClose();
          }}
          className="text-[11px] px-2 py-1 bg-primary text-primary-foreground rounded hover:bg-primary/90"
        >
          Apply
        </button>
      </div>
    </div>
  );
}

interface AddFilterDropdownProps {
  columns: { name: string; editor: TypeHint }[];
  activeColumns: Set<string>;
  onSelect: (colName: string) => void;
  onClose: () => void;
}

function AddFilterDropdown({ columns, activeColumns, onSelect, onClose }: AddFilterDropdownProps) {
  const [search, setSearch] = useState("");
  const [highlightIndex, setHighlightIndex] = useState(0);
  const ref = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  const filtered = columns.filter(
    (c) => !activeColumns.has(c.name) && c.name.toLowerCase().includes(search.toLowerCase())
  );

  // Reset highlight when filtered list changes
  useEffect(() => {
    setHighlightIndex(0);
  }, [search]);

  // Click outside closes
  useEffect(() => {
    function handler(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        onClose();
      }
    }
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [onClose]);

  // Autofocus input
  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLInputElement>) => {
      if (e.key === "ArrowDown") {
        e.preventDefault();
        setHighlightIndex((i) => Math.min(i + 1, filtered.length - 1));
      } else if (e.key === "ArrowUp") {
        e.preventDefault();
        setHighlightIndex((i) => Math.max(i - 1, 0));
      } else if (e.key === "Enter") {
        e.preventDefault();
        if (filtered[highlightIndex]) {
          onSelect(filtered[highlightIndex].name);
        }
      } else if (e.key === "Escape") {
        e.preventDefault();
        onClose();
      }
    },
    [filtered, highlightIndex, onSelect, onClose]
  );

  return (
    <div
      ref={ref}
      className="absolute top-full left-0 z-20 mt-1 bg-card border border-border rounded shadow-lg w-48"
    >
      <div className="p-1.5 border-b border-border">
        <input
          ref={inputRef}
          type="text"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="Search columns..."
          className="w-full text-xs border border-border rounded px-1.5 py-1 outline-none focus:border-ring"
        />
      </div>
      <div className="max-h-48 overflow-y-auto">
        {filtered.length === 0 ? (
          <div className="text-[11px] text-muted-foreground px-2 py-2 text-center">No columns match</div>
        ) : (
          filtered.map((col, idx) => (
            <button
              key={col.name}
              onMouseDown={(e) => {
                e.preventDefault();
                onSelect(col.name);
              }}
              onMouseEnter={() => setHighlightIndex(idx)}
              className={`w-full text-left text-[11px] px-2 py-1.5 ${
                idx === highlightIndex
                  ? "bg-primary/10 text-primary"
                  : "text-foreground hover:bg-muted"
              }`}
            >
              {col.name}
            </button>
          ))
        )}
      </div>
    </div>
  );
}

function chipLabel(f: Filter): string {
  if (NO_VALUE_OPS.includes(f.op)) {
    return `${f.column} ${f.op}`;
  }
  return `${f.column} ${f.op} ${f.val ?? ""}`;
}

export default function FilterBar({ columns, filters, onFiltersChange }: FilterBarProps) {
  const [openCol, setOpenCol] = useState<string | null>(null);
  const [showAddDropdown, setShowAddDropdown] = useState(false);

  const filterMap = new Map(filters.map((f) => [f.column, f]));
  const activeColumns = new Set(filters.map((f) => f.column));

  const colMap = new Map(columns.map((c) => [c.name, c]));

  const applyFilter = (f: Filter) => {
    const updated = filters.filter((x) => x.column !== f.column);
    onFiltersChange([...updated, f]);
  };

  const clearFilter = (col: string) => {
    onFiltersChange(filters.filter((f) => f.column !== col));
    setOpenCol(null);
  };

  const handleSelectColumn = (colName: string) => {
    setShowAddDropdown(false);
    setOpenCol(colName);
  };

  return (
    <div className="flex items-center gap-1 flex-wrap px-2 py-1 bg-card border-b border-border">
      {/* Active filter chips */}
      {filters.map((f) => {
        const col = colMap.get(f.column);
        return (
          <div key={f.column} className="relative flex items-center">
            <button
              onClick={() => setOpenCol(openCol === f.column ? null : f.column)}
              className="text-[11px] px-2 py-0.5 rounded-l border border-r-0 bg-primary/10 border-primary/30 text-primary font-medium hover:bg-primary/15"
            >
              {chipLabel(f)}
            </button>
            <button
              onClick={() => clearFilter(f.column)}
              className="text-[11px] px-1.5 py-0.5 rounded-r border bg-primary/10 border-primary/30 text-primary/60 hover:bg-primary/15 hover:text-primary leading-none"
              aria-label={`Remove filter on ${f.column}`}
            >
              ×
            </button>
            {openCol === f.column && col && (
              <FilterPopover
                colName={f.column}
                colType={col.editor}
                existing={filterMap.get(f.column)}
                onApply={applyFilter}
                onClear={() => clearFilter(f.column)}
                onClose={() => setOpenCol(null)}
              />
            )}
          </div>
        );
      })}

      {/* Add filter button + dropdown */}
      <div className="relative">
        <button
          onClick={() => setShowAddDropdown((v) => !v)}
          className="text-[11px] px-2 py-0.5 rounded border border-border text-muted-foreground hover:bg-muted"
        >
          + Add filter
        </button>
        {showAddDropdown && (
          <AddFilterDropdown
            columns={columns}
            activeColumns={activeColumns}
            onSelect={handleSelectColumn}
            onClose={() => setShowAddDropdown(false)}
          />
        )}
      </div>

      {/* Clear all */}
      {filters.length > 0 && (
        <button
          onClick={() => onFiltersChange([])}
          className="text-[11px] text-destructive hover:text-destructive/80 ml-1"
        >
          Clear all
        </button>
      )}
    </div>
  );
}
