import { useState, useCallback, useRef } from "react";
import { Routes, Route, Navigate, Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { api, queryKeys } from "./lib/api";
import { getJSON, setJSON } from "./lib/storage";
import { buttonVariants } from "@/components/ui/button";
import ConnectionBanner from "./components/ConnectionBanner";
import TableList from "./components/TableList";
import TableView from "./routes/TableView";
import HistoryView from "./routes/HistoryView";
import { Toaster } from "@/components/ui/sonner";

const SIDEBAR_KEY = "dbseer:sidebarWidth";
const SIDEBAR_MIN = 200;
const SIDEBAR_MAX = 600;
const SIDEBAR_DEFAULT = 288;

function DefaultRedirect() {
  const { data, isLoading } = useQuery({
    queryKey: queryKeys.schema,
    queryFn: () => api.schema(),
  });

  if (isLoading) {
    return <div className="p-6 text-sm text-muted-foreground">Loading...</div>;
  }

  const first =
    data?.tables.find((t) => t.editable) ?? data?.tables[0];

  if (first) {
    return <Navigate to={`/t/${first.schema}.${first.name}`} replace />;
  }

  return (
    <div className="flex flex-col items-center justify-center h-full text-muted-foreground">
      <div className="text-4xl mb-4">&#x1F5C4;&#xFE0F;</div>
      <div className="text-base font-medium mb-1 text-foreground">No tables found</div>
      <div className="text-sm">
        Connect to a Postgres database to get started.
      </div>
    </div>
  );
}

function NotFound() {
  return (
    <div className="flex flex-col items-center justify-center h-full text-muted-foreground">
      <div className="text-4xl mb-4">404</div>
      <div className="text-sm mb-4">Page not found.</div>
      <Link to="/" className="text-primary hover:underline text-sm">
        Go home
      </Link>
    </div>
  );
}

export default function App() {
  const [sidebarWidth, setSidebarWidth] = useState<number>(
    () => {
      const saved = getJSON<number>(SIDEBAR_KEY, SIDEBAR_DEFAULT);
      return Math.max(SIDEBAR_MIN, Math.min(SIDEBAR_MAX, saved));
    }
  );

  const dragging = useRef(false);

  const handleMouseDown = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    dragging.current = true;

    const handleMouseMove = (e: MouseEvent) => {
      if (!dragging.current) return;
      const w = Math.max(SIDEBAR_MIN, Math.min(SIDEBAR_MAX, e.clientX));
      setSidebarWidth(w);
    };

    const handleMouseUp = () => {
      dragging.current = false;
      document.removeEventListener("mousemove", handleMouseMove);
      document.removeEventListener("mouseup", handleMouseUp);
      setSidebarWidth((w) => {
        setJSON(SIDEBAR_KEY, w);
        return w;
      });
    };

    document.addEventListener("mousemove", handleMouseMove);
    document.addEventListener("mouseup", handleMouseUp);
  }, []);

  return (
    <div className="flex flex-col h-full">
      <Toaster />
      <ConnectionBanner />
      <div className="flex flex-1 overflow-hidden">
        <aside
          className="border-r border-border bg-muted flex-shrink-0 flex flex-col overflow-hidden"
          style={{ width: sidebarWidth }}
        >
          <div className="px-3 py-2 border-b border-border flex items-center justify-between">
            <span className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
              dbseer
            </span>
            <Link to="/history" className={buttonVariants({ variant: "outline", size: "xs" }) + " text-[11px]"}>
              History
            </Link>
          </div>
          <div className="flex-1 overflow-hidden">
            <TableList />
          </div>
        </aside>

        <div
          className="w-1 cursor-col-resize hover:bg-primary/40 active:bg-primary transition-colors flex-shrink-0"
          onMouseDown={handleMouseDown}
        />

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
