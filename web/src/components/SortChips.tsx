import type { Sort } from "../lib/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

interface SortChipsProps {
  sorts: Sort[];
  onRemove: (column: string) => void;
}

export default function SortChips({ sorts, onRemove }: SortChipsProps) {
  if (sorts.length === 0) return null;

  return (
    <div className="flex items-center gap-1 flex-wrap">
      {sorts.map((sort) => (
        <Badge key={sort.column} variant="secondary" className="gap-1 px-2 py-0.5 text-xs">
          <span className="font-medium">{sort.column}</span>
          <span>{sort.desc ? "\u2193" : "\u2191"}</span>
          <Button
            variant="ghost"
            size="icon"
            onClick={() => onRemove(sort.column)}
            className="h-3 w-3 p-0 text-muted-foreground hover:text-foreground hover:bg-transparent ml-0.5"
            aria-label={`Remove sort on ${sort.column}`}
          >
            &#xd7;
          </Button>
        </Badge>
      ))}
    </div>
  );
}
