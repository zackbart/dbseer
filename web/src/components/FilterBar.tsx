import { useState, useRef, useEffect } from "react";
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
      className="absolute top-full left-0 z-20 mt-1 bg-white border border-slate-200 rounded shadow-lg p-3 w-56"
    >
      <div className="text-[11px] font-medium text-slate-500 mb-2 uppercase tracking-wide">{colName}</div>
      <select
        value={op}
        onChange={(e) => setOp(e.target.value as FilterOp)}
        className="w-full text-xs border border-slate-200 rounded px-1 py-1 mb-2"
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
          className="w-full text-xs border border-slate-200 rounded px-1 py-1 mb-2"
          placeholder="value"
          autoFocus
        />
      )}
      <div className="flex gap-1 justify-end">
        <button
          onClick={onClear}
          className="text-[11px] px-2 py-1 text-slate-500 border border-slate-200 rounded hover:bg-slate-50"
        >
          Clear
        </button>
        <button
          onClick={() => {
            onApply({ column: colName, op, val: needsValue ? val : undefined });
            onClose();
          }}
          className="text-[11px] px-2 py-1 bg-blue-600 text-white rounded hover:bg-blue-700"
        >
          Apply
        </button>
      </div>
    </div>
  );
}

export default function FilterBar({ columns, filters, onFiltersChange }: FilterBarProps) {
  const [openCol, setOpenCol] = useState<string | null>(null);

  const filterMap = new Map(filters.map((f) => [f.column, f]));

  const applyFilter = (f: Filter) => {
    const updated = filters.filter((x) => x.column !== f.column);
    onFiltersChange([...updated, f]);
  };

  const clearFilter = (col: string) => {
    onFiltersChange(filters.filter((f) => f.column !== col));
    setOpenCol(null);
  };

  return (
    <div className="flex items-center gap-1 flex-wrap px-2 py-1 bg-white border-b border-slate-100">
      <span className="text-[11px] text-slate-400 mr-1">Filter:</span>
      {columns.map((col) => {
        const active = filterMap.has(col.name);
        return (
          <div key={col.name} className="relative">
            <button
              onClick={() => setOpenCol(openCol === col.name ? null : col.name)}
              className={`text-[11px] px-2 py-0.5 rounded border ${
                active
                  ? "bg-blue-50 border-blue-300 text-blue-700 font-medium"
                  : "border-slate-200 text-slate-500 hover:bg-slate-50"
              }`}
            >
              {col.name}
              {active && <span className="ml-1">✓</span>}
            </button>
            {openCol === col.name && (
              <FilterPopover
                colName={col.name}
                colType={col.editor}
                existing={filterMap.get(col.name)}
                onApply={applyFilter}
                onClear={() => clearFilter(col.name)}
                onClose={() => setOpenCol(null)}
              />
            )}
          </div>
        );
      })}
      {filters.length > 0 && (
        <button
          onClick={() => onFiltersChange([])}
          className="text-[11px] text-red-500 hover:text-red-700 ml-1"
        >
          Clear all
        </button>
      )}
    </div>
  );
}
