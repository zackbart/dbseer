import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { api, ApiError } from "../lib/api";
import type { WireCell, Table } from "../lib/types";

interface FkLinkProps {
  schema: string;
  table: string;
  col: string;
  val: WireCell;
  display: string;
  tableSchema: Table;
}

export default function FkLink({ schema, table, col, val, display, tableSchema }: FkLinkProps) {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fk = tableSchema.foreign_keys.find((fk) => fk.columns.includes(col));

  const handleClick = async () => {
    if (!fk) return;
    setLoading(true);
    setError(null);
    try {
      const target = await api.fkTarget(schema, table, col, val);
      const params = new URLSearchParams();
      for (const [pk, filterVal] of Object.entries(target.filter)) {
        params.set(`op[${pk}]`, filterVal.op);
        params.set(`val[${pk}]`, filterVal.val);
      }
      navigate(`/t/${target.schema}.${target.table}?${params.toString()}`);
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message);
      } else {
        setError("Navigation failed");
      }
    } finally {
      setLoading(false);
    }
  };

  if (loading) {
    return (
      <span className="inline-flex items-center gap-1 text-primary">
        <span className="text-[10px]">&#x27F3;</span>
        <span className="truncate max-w-[80px]">{display}</span>
      </span>
    );
  }

  if (!fk) {
    return <span>{display}</span>;
  }

  return (
    <span title={error ?? undefined}>
      <button
        onClick={handleClick}
        className={`text-primary underline underline-offset-2 hover:text-primary/80 truncate max-w-[120px] text-left ${
          error ? "text-destructive" : ""
        }`}
      >
        {display}
      </button>
    </span>
  );
}
