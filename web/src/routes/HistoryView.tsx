import { useQuery } from "@tanstack/react-query";
import { api, queryKeys } from "../lib/api";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

export default function HistoryView() {
  const { data, isLoading, isError } = useQuery({
    queryKey: queryKeys.history({ limit: 100 }),
    queryFn: () => api.history({ limit: 100 }),
  });

  if (isLoading) {
    return <div className="p-6 text-sm text-muted-foreground">Loading history...</div>;
  }

  if (isError || !data) {
    return <div className="p-6 text-sm text-destructive">Failed to load history.</div>;
  }

  return (
    <div className="p-4 overflow-auto h-full">
      <h2 className="text-sm font-semibold text-foreground mb-3">Query History (last 100)</h2>
      {data.entries.length === 0 ? (
        <div className="text-sm text-muted-foreground">No history yet.</div>
      ) : (
        <Table className="text-xs">
          <TableHeader>
            <TableRow className="bg-muted">
              <TableHead className="px-2 py-1.5 whitespace-nowrap">Timestamp</TableHead>
              <TableHead className="px-2 py-1.5">Op</TableHead>
              <TableHead className="px-2 py-1.5">Table</TableHead>
              <TableHead className="px-2 py-1.5 text-right">Affected</TableHead>
              <TableHead className="px-2 py-1.5">SQL</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {data.entries.map((entry, i) => (
              <TableRow key={i} className="border-b border-border hover:bg-muted/50">
                <TableCell className="px-2 py-1 whitespace-nowrap text-muted-foreground font-mono">
                  {new Date(entry.ts).toLocaleString()}
                </TableCell>
                <TableCell className="px-2 py-1 font-medium text-foreground">{entry.op}</TableCell>
                <TableCell className="px-2 py-1 text-foreground">{entry.table}</TableCell>
                <TableCell className="px-2 py-1 text-right text-foreground">{entry.affected}</TableCell>
                <TableCell className="px-2 py-1 font-mono text-muted-foreground max-w-xs truncate" title={entry.sql}>
                  {entry.sql.length > 120 ? entry.sql.slice(0, 120) + "..." : entry.sql}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  );
}
