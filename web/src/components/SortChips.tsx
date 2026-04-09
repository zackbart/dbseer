import type { Sort } from "../lib/types";

interface SortChipsProps {
  sorts: Sort[];
  onRemove: (column: string) => void;
}

export default function SortChips({ sorts, onRemove }: SortChipsProps) {
  if (sorts.length === 0) return null;

  return (
    <div className="flex items-center gap-1 flex-wrap">
      {sorts.map((sort) => (
        <span
          key={sort.column}
          className="inline-flex items-center gap-1 bg-slate-100 border border-slate-200 rounded px-2 py-0.5 text-xs text-slate-700"
        >
          <span className="font-medium">{sort.column}</span>
          <span>{sort.desc ? "↓" : "↑"}</span>
          <button
            onClick={() => onRemove(sort.column)}
            className="text-slate-400 hover:text-slate-700 ml-0.5"
            aria-label={`Remove sort on ${sort.column}`}
          >
            ×
          </button>
        </span>
      ))}
    </div>
  );
}
