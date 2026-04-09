import { useQuery } from "@tanstack/react-query";
import { api, queryKeys } from "../lib/api";

export default function HistoryView() {
  const { data, isLoading, isError } = useQuery({
    queryKey: queryKeys.history({ limit: 100 }),
    queryFn: () => api.history({ limit: 100 }),
  });

  if (isLoading) {
    return <div className="p-6 text-sm text-slate-400">Loading history…</div>;
  }

  if (isError || !data) {
    return <div className="p-6 text-sm text-red-500">Failed to load history.</div>;
  }

  return (
    <div className="p-4 overflow-auto h-full">
      <h2 className="text-sm font-semibold text-slate-700 mb-3">Query History (last 100)</h2>
      {data.entries.length === 0 ? (
        <div className="text-sm text-slate-400">No history yet.</div>
      ) : (
        <table className="w-full text-xs border-collapse">
          <thead>
            <tr className="bg-slate-50 border-b border-slate-200">
              <th className="px-2 py-1.5 text-left font-medium text-slate-600 whitespace-nowrap">Timestamp</th>
              <th className="px-2 py-1.5 text-left font-medium text-slate-600">Op</th>
              <th className="px-2 py-1.5 text-left font-medium text-slate-600">Table</th>
              <th className="px-2 py-1.5 text-right font-medium text-slate-600">Affected</th>
              <th className="px-2 py-1.5 text-left font-medium text-slate-600">SQL</th>
            </tr>
          </thead>
          <tbody>
            {data.entries.map((entry, i) => (
              <tr key={i} className="border-b border-slate-100 hover:bg-slate-50">
                <td className="px-2 py-1 whitespace-nowrap text-slate-500 font-mono">
                  {new Date(entry.ts).toLocaleString()}
                </td>
                <td className="px-2 py-1 font-medium text-slate-700">{entry.op}</td>
                <td className="px-2 py-1 text-slate-700">{entry.table}</td>
                <td className="px-2 py-1 text-right text-slate-700">{entry.affected}</td>
                <td className="px-2 py-1 font-mono text-slate-500 max-w-xs truncate" title={entry.sql}>
                  {entry.sql.length > 120 ? entry.sql.slice(0, 120) + "…" : entry.sql}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
