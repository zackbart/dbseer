import { useState, useEffect, useRef } from "react";
import type { WireCell, TypeHint, EnumType } from "../lib/types";

interface CellEditorProps {
  cell: WireCell;
  nullable: boolean;
  enums: EnumType[];
  enumName: string | null;
  onCommit: (cell: WireCell) => void;
  onCancel: () => void;
}

const READONLY_TYPES: TypeHint[] = [
  "tsvector", "xml", "bytea", "geometry", "oid", "bit", "inet", "cidr", "macaddr", "range", "array",
];

function cellDisplayValue(cell: WireCell): string {
  if (cell.v === null) return "";
  if (typeof cell.v === "string") return cell.v;
  if (typeof cell.v === "number" || typeof cell.v === "boolean") return String(cell.v);
  return JSON.stringify(cell.v);
}

export default function CellEditor({
  cell,
  nullable,
  enums,
  enumName,
  onCommit,
  onCancel,
}: CellEditorProps) {
  const [isNull, setIsNull] = useState(cell.v === null);
  const [strVal, setStrVal] = useState(cellDisplayValue(cell));
  const [jsonError, setJsonError] = useState(false);
  const inputRef = useRef<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>(null);

  useEffect(() => {
    if (inputRef.current) {
      inputRef.current.focus();
      if (inputRef.current instanceof HTMLInputElement || inputRef.current instanceof HTMLTextAreaElement) {
        inputRef.current.select();
      }
    }
  }, []);

  if (READONLY_TYPES.includes(cell.t)) {
    return (
      <span
        className="text-slate-400 text-xs italic cursor-not-allowed"
        title="read-only in v0.1"
      >
        {cellDisplayValue(cell)}
      </span>
    );
  }

  const commit = () => {
    if (isNull) {
      onCommit({ v: null, t: "" });
      return;
    }
    if (cell.t === "json" || cell.t === "jsonb") {
      try {
        const parsed: unknown = JSON.parse(strVal);
        setJsonError(false);
        onCommit({ v: parsed, t: cell.t });
      } catch {
        setJsonError(true);
        return;
      }
    } else if (cell.t === "bool") {
      if (strVal === "true") onCommit({ v: true, t: "bool" });
      else if (strVal === "false") onCommit({ v: false, t: "bool" });
      else onCommit({ v: null, t: "" });
    } else if (cell.t === "int") {
      const n = parseInt(strVal, 10);
      onCommit({ v: isNaN(n) ? null : n, t: "int" });
    } else if (cell.t === "float") {
      const n = parseFloat(strVal);
      onCommit({ v: isNaN(n) ? null : n, t: "float" });
    } else {
      onCommit({ v: strVal, t: cell.t });
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && cell.t !== "jsonb" && cell.t !== "json") {
      e.preventDefault();
      commit();
    }
    if (e.key === "Escape") {
      onCancel();
    }
  };

  const nullToggle = nullable ? (
    <button
      onClick={() => setIsNull((n) => !n)}
      className={`ml-1 text-[10px] px-1 py-0.5 rounded border ${
        isNull ? "bg-slate-200 text-slate-600 border-slate-300" : "text-slate-400 border-slate-200 hover:bg-slate-100"
      }`}
      title="Toggle NULL"
    >
      NULL
    </button>
  ) : null;

  if (isNull) {
    return (
      <span className="flex items-center gap-1">
        <span className="text-xs italic text-slate-400">NULL</span>
        {nullToggle}
        <button onClick={commit} className="text-[10px] px-1 py-0.5 bg-blue-600 text-white rounded">OK</button>
        <button onClick={onCancel} className="text-[10px] px-1 py-0.5 border border-slate-200 rounded">✕</button>
      </span>
    );
  }

  // Bool: three-state toggle
  if (cell.t === "bool") {
    const options = ["true", "false", ...(nullable ? ["null"] : [])];
    return (
      <span className="flex items-center gap-1">
        <select
          ref={inputRef as React.RefObject<HTMLSelectElement>}
          value={strVal}
          onChange={(e) => setStrVal(e.target.value)}
          onKeyDown={handleKeyDown}
          onBlur={commit}
          className="text-xs border border-slate-300 rounded px-1 py-0.5"
        >
          {options.map((o) => (
            <option key={o} value={o}>{o}</option>
          ))}
        </select>
        {nullToggle}
      </span>
    );
  }

  // Enum: select dropdown
  if (cell.t === "enum" && enumName) {
    const enumType = enums.find((e) => e.name === enumName);
    return (
      <span className="flex items-center gap-1">
        <select
          ref={inputRef as React.RefObject<HTMLSelectElement>}
          value={strVal}
          onChange={(e) => setStrVal(e.target.value)}
          onKeyDown={handleKeyDown}
          onBlur={commit}
          className="text-xs border border-slate-300 rounded px-1 py-0.5"
        >
          {(enumType?.values ?? []).map((v) => (
            <option key={v} value={v}>{v}</option>
          ))}
        </select>
        {nullToggle}
      </span>
    );
  }

  // JSON/JSONB: textarea
  if (cell.t === "json" || cell.t === "jsonb") {
    return (
      <span className="flex flex-col gap-1">
        <textarea
          ref={inputRef as React.RefObject<HTMLTextAreaElement>}
          value={strVal}
          onChange={(e) => { setStrVal(e.target.value); setJsonError(false); }}
          onKeyDown={handleKeyDown}
          onBlur={commit}
          rows={3}
          className={`text-xs border rounded px-1 py-0.5 font-mono w-48 ${
            jsonError ? "border-red-500" : "border-slate-300"
          }`}
        />
        <span className="flex items-center gap-1">
          {nullToggle}
          <button onClick={commit} className="text-[10px] px-1 py-0.5 bg-blue-600 text-white rounded">OK</button>
          <button onClick={onCancel} className="text-[10px] px-1 py-0.5 border border-slate-200 rounded">✕</button>
        </span>
      </span>
    );
  }

  // Date/timestamp inputs
  let inputType = "text";
  if (cell.t === "date") inputType = "date";
  else if (cell.t === "timestamp" || cell.t === "timestamptz") inputType = "datetime-local";
  else if (cell.t === "int") inputType = "number";
  else if (cell.t === "float" || cell.t === "numeric") inputType = "number";

  return (
    <span className="flex items-center gap-1">
      <input
        ref={inputRef as React.RefObject<HTMLInputElement>}
        type={inputType}
        value={strVal}
        step={cell.t === "int" ? 1 : undefined}
        onChange={(e) => setStrVal(e.target.value)}
        onKeyDown={handleKeyDown}
        onBlur={commit}
        className="text-xs border border-slate-300 rounded px-1 py-0.5 w-32"
      />
      {nullToggle}
    </span>
  );
}
