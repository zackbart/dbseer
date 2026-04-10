import { useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

interface ConfirmUnscopedProps {
  affectedCount: number;
  sql?: string;
  onConfirm: (count: number) => void;
  onCancel: () => void;
}

export default function ConfirmUnscoped({
  affectedCount,
  sql,
  onConfirm,
  onCancel,
}: ConfirmUnscopedProps) {
  const [input, setInput] = useState("");
  const confirmed = input === String(affectedCount);

  return (
    <Dialog open onOpenChange={(open) => { if (!open) onCancel(); }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Confirm bulk operation</DialogTitle>
          <DialogDescription>
            This update will affect{" "}
            <strong className="text-foreground">{affectedCount}</strong> rows.
          </DialogDescription>
        </DialogHeader>
        {sql && (
          <pre className="text-[11px] bg-muted border border-border rounded p-2 overflow-x-auto text-muted-foreground font-mono">
            {sql}
          </pre>
        )}
        <p className="text-xs text-muted-foreground">
          Type <strong className="text-foreground">{affectedCount}</strong> to confirm:
        </p>
        <Input
          type="text"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder={String(affectedCount)}
          autoFocus
        />
        <DialogFooter>
          <Button variant="outline" onClick={onCancel}>Cancel</Button>
          <Button
            variant="destructive"
            disabled={!confirmed}
            onClick={() => confirmed && onConfirm(affectedCount)}
          >
            Confirm
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
