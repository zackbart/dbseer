import { Routes, Route, Navigate, Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { api, queryKeys } from "./lib/api";
import ConnectionBanner from "./components/ConnectionBanner";
import TableList from "./components/TableList";
import TableView from "./routes/TableView";
import HistoryView from "./routes/HistoryView";

function DefaultRedirect() {
  const { data, isLoading } = useQuery({
    queryKey: queryKeys.schema,
    queryFn: () => api.schema(),
  });

  if (isLoading) {
    return <div className="p-6 text-sm text-slate-400">Loading…</div>;
  }

  // Navigate to first editable table, or first table, or empty state
  const first =
    data?.tables.find((t) => t.editable) ?? data?.tables[0];

  if (first) {
    return <Navigate to={`/t/${first.schema}.${first.name}`} replace />;
  }

  return (
    <div className="flex flex-col items-center justify-center h-full text-slate-500">
      <div className="text-4xl mb-4">🗄️</div>
      <div className="text-base font-medium mb-1">No tables found</div>
      <div className="text-sm text-slate-400">
        Connect to a Postgres database to get started.
      </div>
    </div>
  );
}

function NotFound() {
  return (
    <div className="flex flex-col items-center justify-center h-full text-slate-500">
      <div className="text-4xl mb-4">404</div>
      <div className="text-sm mb-4">Page not found.</div>
      <Link to="/" className="text-blue-600 hover:underline text-sm">
        Go home
      </Link>
    </div>
  );
}

export default function App() {
  return (
    <div className="flex flex-col h-full">
      <ConnectionBanner />
      <div className="flex flex-1 overflow-hidden">
        {/* Left sidebar */}
        <aside className="w-72 border-r border-slate-200 bg-slate-50 flex-shrink-0 flex flex-col overflow-hidden">
          <div className="px-3 py-2 border-b border-slate-200 flex items-center justify-between">
            <span className="text-xs font-semibold text-slate-500 uppercase tracking-wider">
              dbseer
            </span>
            <Link
              to="/history"
              className="text-[11px] text-slate-400 hover:text-slate-700"
            >
              history
            </Link>
          </div>
          <div className="flex-1 overflow-hidden">
            <TableList />
          </div>
        </aside>

        {/* Main content */}
        <main className="flex-1 overflow-hidden flex flex-col">
          <Routes>
            <Route path="/" element={<DefaultRedirect />} />
            <Route path="/t/:schemaTable" element={<TableView />} />
            <Route path="/history" element={<HistoryView />} />
            <Route path="*" element={<NotFound />} />
          </Routes>
        </main>
      </div>
    </div>
  );
}
