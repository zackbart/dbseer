import { useQuery } from "@tanstack/react-query";
import { api, queryKeys } from "../lib/api";
import { Badge } from "@/components/ui/badge";

export default function ConnectionBanner() {
  const { data, isError } = useQuery({
    queryKey: queryKeys.discover,
    queryFn: () => api.discover(),
  });

  if (isError || !data) {
    return (
      <div className="h-8 bg-destructive/10 border-b border-destructive/20 flex items-center px-4 text-xs text-destructive">
        Could not connect to dbseer backend.
      </div>
    );
  }

  const sourceLabel: Record<string, string> = {
    env: "via .env",
    prisma: "via schema.prisma",
    drizzle: "via drizzle.config",
    compose: "via docker-compose",
    "dbseer-config": "via .dbseer.json",
    flag: "via --url flag",
    none: "no source",
  };

  return (
    <div className="h-8 bg-muted border-b border-border flex items-center px-4 gap-3 text-xs text-muted-foreground shrink-0">
      <span className="font-medium text-foreground">
        {data.host}:{data.port}/{data.database}
      </span>
      <span className="text-muted-foreground/50">—</span>
      <span>{data.user}</span>
      <span className="ml-auto flex items-center gap-2">
        <Badge variant="outline" className="text-[11px] font-normal" title={data.path}>
          {sourceLabel[data.source] ?? data.source}
        </Badge>
        {data.readonly && (
          <Badge variant="outline" className="text-[11px] text-destructive border-destructive/30">
            readonly
          </Badge>
        )}
        {data.host !== "127.0.0.1" && data.host !== "localhost" && (
          <Badge variant="outline" className="text-[11px] text-amber-600 border-amber-300">
            remote host
          </Badge>
        )}
      </span>
    </div>
  );
}
