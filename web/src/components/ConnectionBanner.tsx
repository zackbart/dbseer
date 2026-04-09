import { useQuery } from "@tanstack/react-query";
import { api, queryKeys } from "../lib/api";

export default function ConnectionBanner() {
  const { data, isError } = useQuery({
    queryKey: queryKeys.discover,
    queryFn: () => api.discover(),
  });

  if (isError || !data) {
    return (
      <div className="h-8 bg-red-50 border-b border-red-200 flex items-center px-4 text-xs text-red-700">
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
    <div className="h-8 bg-slate-100 border-b border-slate-200 flex items-center px-4 gap-3 text-xs text-slate-600 shrink-0">
      <span className="font-medium text-slate-800">
        {data.host}:{data.port}/{data.database}
      </span>
      <span className="text-slate-400">—</span>
      <span>{data.user}</span>
      <span className="ml-auto flex items-center gap-2">
        <span className="bg-slate-200 text-slate-600 px-2 py-0.5 rounded text-[11px]">
          {sourceLabel[data.source] ?? data.source}
        </span>
        {data.readonly && (
          <span className="bg-red-100 text-red-700 px-2 py-0.5 rounded text-[11px] font-medium">
            readonly
          </span>
        )}
        {data.host !== "127.0.0.1" && data.host !== "localhost" && (
          <span className="bg-yellow-100 text-yellow-800 px-2 py-0.5 rounded text-[11px]">
            remote host
          </span>
        )}
      </span>
    </div>
  );
}
