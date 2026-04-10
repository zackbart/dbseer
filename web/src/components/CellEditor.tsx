import { useState, useEffect, useRef } from "react";
import type { WireCell, TypeHint, EnumType } from "../lib/types";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

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
  const inputRef = useRef<HTMLInputElement | HTMLTextAreaElement>(null);
  const cancelledRef = useRef(false);

  useEffect(() => {
    cancelledRef.current = false;
    if (inputRef.current) {
      inputRef.current.focus();
      inputRef.current.select();
    }
  }, []);

  if (READONLY_TYPES.includes(cell.t)) {
    return (
      <span className="text-muted-foreground text-xs italic cursor-not-allowed" title="read-only in v0.1">
        {cellDisplayValue(cell)}
      </span>
    );
  }

  const commit = () => {
    if (cancelledRef.current) {
      onCancel();
      return;
    }
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
      cancelledRef.current = true;
      onCancel();
    }
  };

  const nullToggle = nullable ? (
    <Button
      variant="outline"
      size="xs"
      onClick={() => setIsNull((n) => !n)}
      className={cn("text-[10px] h-5 px-1", isNull && "bg-muted text-foreground")}
      title="Toggle NULL"
    >
      NULL
    </Button>
  ) : null;

  if (isNull) {
    return (
      <span className="flex items-center gap-1">
        <span className="text-xs italic text-muted-foreground">NULL</span>
        {nullToggle}
        <Button size="xs" onClick={commit} className="text-[10px] h-5 px-1">OK</Button>
        <Button variant="outline" size="xs" onClick={onCancel} className="text-[10px] h-5 px-1">&#x2715;</Button>
      </span>
    );
  }

  // Bool: select with immediate commit via onValueChange
  if (cell.t === "bool") {
    const options = ["true", "false", ...(nullable ? ["null"] : [])];
    return (
      <span className="flex items-center gap-1">
        <Select
          value={strVal}
          onValueChange={(val) => {
            const v = val ?? "";
            setStrVal(v);
            cancelledRef.current = false;
            if (v === "true") onCommit({ v: true, t: "bool" });
            else if (v === "false") onCommit({ v: false, t: "bool" });
            else onCommit({ v: null, t: "" });
          }}
        >
          <SelectTrigger size="sm" className="text-xs h-6 px-1.5 py-0.5 min-w-16">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {options.map((o) => (
              <SelectItem key={o} value={o}>{o}</SelectItem>
            ))}
          </SelectContent>
        </Select>
        {nullToggle}
      </span>
    );
  }

  // Enum: select with immediate commit
  if (cell.t === "enum" && enumName) {
    const enumType = enums.find((e) => e.name === enumName);
    const enumValues = enumType?.values ?? [];
    return (
      <span className="flex items-center gap-1">
        <Select
          value={strVal}
          onValueChange={(val) => {
            const v = val ?? "";
            setStrVal(v);
            cancelledRef.current = false;
            onCommit({ v, t: cell.t });
          }}
        >
          <SelectTrigger size="sm" className="text-xs h-6 px-1.5 py-0.5 min-w-16">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {enumValues.map((v) => (
              <SelectItem key={v} value={v}>{v}</SelectItem>
            ))}
          </SelectContent>
        </Select>
        {nullToggle}
      </span>
    );
  }

  // JSON/JSONB: textarea
  if (cell.t === "json" || cell.t === "jsonb") {
    return (
      <span className="flex flex-col gap-1">
        <Textarea
          ref={inputRef as React.RefObject<HTMLTextAreaElement>}
          value={strVal}
          onChange={(e) => { setStrVal(e.target.value); setJsonError(false); }}
          onKeyDown={handleKeyDown}
          onBlur={commit}
          rows={3}
          className={cn("text-xs font-mono w-48 min-h-0 px-1 py-0.5", jsonError && "border-destructive")}
        />
        <span className="flex items-center gap-1">
          {nullToggle}
          <Button size="xs" onClick={commit} className="text-[10px] h-5 px-1">OK</Button>
          <Button variant="outline" size="xs" onClick={onCancel} className="text-[10px] h-5 px-1">&#x2715;</Button>
        </span>
      </span>
    );
  }

  // Default: text/number/date inputs
  let inputType = "text";
  if (cell.t === "date") inputType = "date";
  else if (cell.t === "timestamp" || cell.t === "timestamptz") inputType = "datetime-local";
  else if (cell.t === "int") inputType = "number";
  else if (cell.t === "float" || cell.t === "numeric") inputType = "number";

  return (
    <span className="flex items-center gap-1">
      <Input
        ref={inputRef as React.RefObject<HTMLInputElement>}
        type={inputType}
        value={strVal}
        step={cell.t === "int" ? 1 : undefined}
        onChange={(e) => setStrVal(e.target.value)}
        onKeyDown={handleKeyDown}
        onBlur={commit}
        className="text-xs h-6 px-1 py-0.5 w-32"
      />
      {nullToggle}
    </span>
  );
}
