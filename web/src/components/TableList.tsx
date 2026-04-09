import { useState } from "react";
import { NavLink } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { api, queryKeys } from "../lib/api";
import type { Table } from "../lib/types";

function kindBadge(table: Table) {
  if (table.kind === "v") return <span className="text-[10px] text-slate-400 ml-1">v</span>;
  if (table.kind === "m") return <span className="text-[10px] text-slate-400 ml-1">m</span>;
  if (!table.editable) return <span className="text-[10px] text-slate-400 ml-1">&#x1F512;</span>;
  return null;
}

export default function TableList() {
  const [search, setSearch] = useState("");
  const { data, isLoading, isError } = useQuery({
    queryKey: queryKeys.schema,
    queryFn: () => api.schema(),
  });

  if (isLoading) {
    return <div className="p-4 text-xs text-slate-400">Loading tables…</div>;
  }

  if (isError || !data) {
    return <div className="p-4 text-xs text-red-500">Failed to load schema.</div>;
  }

  const lowerSearch = search.toLowerCase();
  const filtered = data.tables.filter(
    (t) =>
      lowerSearch === "" ||
      t.name.toLowerCase().includes(lowerSearch) ||
      t.schema.toLowerCase().includes(lowerSearch)
  );

  // Group by schema
  const schemaGroups: Record<string, Table[]> = {};
  for (const t of filtered) {
    if (!schemaGroups[t.schema]) schemaGroups[t.schema] = [];
    schemaGroups[t.schema].push(t);
  }

  return (
    <div className="flex flex-col h-full">
      <div className="p-2 border-b border-slate-200">
        <input
          type="text"
          placeholder="Search tables…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="w-full px-2 py-1 text-xs border border-slate-200 rounded bg-white focus:outline-none focus:border-slate-400"
        />
      </div>
      <div className="flex-1 overflow-y-auto py-2">
        {Object.entries(schemaGroups).map(([schema, tables]) => (
          <div key={schema} className="mb-2">
            <div className="px-3 py-1 text-[10px] uppercase tracking-wider text-slate-400 font-semibold">
              {schema}
            </div>
            {tables.map((t) => (
              <NavLink
                key={`${t.schema}.${t.name}`}
                to={`/t/${t.schema}.${t.name}`}
                className={({ isActive }) =>
                  `flex items-center px-3 py-1.5 text-xs rounded mx-1 gap-1 ${
                    isActive
                      ? "bg-blue-600 text-white"
                      : "text-slate-700 hover:bg-slate-100"
                  }`
                }
              >
                <span className="truncate flex-1">{t.name}</span>
                {kindBadge(t)}
              </NavLink>
            ))}
          </div>
        ))}
        {filtered.length === 0 && (
          <div className="px-3 py-2 text-xs text-slate-400">No tables found.</div>
        )}
      </div>
    </div>
  );
}
