import { useEffect, useMemo, useState } from "react";
import type { Column, EnumType, Table, WireCell } from "../lib/types";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

interface InsertRowDialogProps {
  open: boolean;
  table: Table;
  enums: EnumType[];
  saving: boolean;
  onClose: () => void;
  onSubmit: (values: Record<string, WireCell>) => Promise<void>;
}

interface DraftField {
  value: string;
  isNull: boolean;
}

const DEFAULT_SENTINEL = "__dbseer_default__";

function editableColumns(table: Table): Column[] {
  return table.columns.filter((column) => !column.is_generated && !column.is_identity);
}

function isRequiredColumn(column: Column): boolean {
  return !column.nullable && column.default === null && !column.is_generated && !column.is_identity;
}

function defaultDraftState(columns: Column[]): Record<string, DraftField> {
  return Object.fromEntries(columns.map((column) => [column.name, { value: "", isNull: false }]));
}

function buildWireCell(column: Column, draft: DraftField): WireCell {
  if (draft.isNull) {
    return { v: null, t: "" };
  }

  switch (column.editor) {
    case "bool":
      return { v: draft.value === "true", t: "bool" };
    case "int": {
      const n = Number.parseInt(draft.value, 10);
      if (Number.isNaN(n)) {
        throw new Error(`${column.name} must be an integer`);
      }
      return { v: n, t: "int" };
    }
    case "float": {
      const n = Number.parseFloat(draft.value);
      if (Number.isNaN(n)) {
        throw new Error(`${column.name} must be a number`);
      }
      return { v: n, t: "float" };
    }
    case "json":
    case "jsonb":
      try {
        return { v: JSON.parse(draft.value), t: column.editor };
      } catch {
        throw new Error(`${column.name} must be valid JSON`);
      }
    default:
      return { v: draft.value, t: column.editor };
  }
}

export default function InsertRowDialog({
  open,
  table,
  enums,
  saving,
  onClose,
  onSubmit,
}: InsertRowDialogProps) {
  const columns = useMemo(() => editableColumns(table), [table]);
  const requiredColumns = useMemo(
    () => columns.filter((column) => isRequiredColumn(column)),
    [columns]
  );
  const optionalColumns = useMemo(
    () => columns.filter((column) => !isRequiredColumn(column)),
    [columns]
  );

  const [draft, setDraft] = useState<Record<string, DraftField>>(() => defaultDraftState(columns));
  const [showOptional, setShowOptional] = useState(requiredColumns.length === 0);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setDraft(defaultDraftState(columns));
    setShowOptional(requiredColumns.length === 0);
    setError(null);
  }, [open, columns, requiredColumns.length]);

  const setField = (columnName: string, next: Partial<DraftField>) => {
    setDraft((prev) => ({
      ...prev,
      [columnName]: { ...prev[columnName], ...next },
    }));
  };

  const submit = async () => {
    try {
      const values: Record<string, WireCell> = {};
      for (const column of columns) {
        const field = draft[column.name];
        const required = isRequiredColumn(column);
        const include = required || field.isNull || field.value !== "";
        if (!include) continue;

        if (required && !field.isNull && field.value === "") {
          throw new Error(`${column.name} is required`);
        }

        values[column.name] = buildWireCell(column, field);
      }

      await onSubmit(values);
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to prepare row values");
    }
  };

  const renderField = (column: Column) => {
    const field = draft[column.name];
    const required = isRequiredColumn(column);
    const enumValues =
      column.enum_name !== null
        ? (enums.find((enumType) => enumType.name === column.enum_name)?.values ?? [])
        : [];

    const label = (
      <div className="mb-1 flex items-center gap-2">
        <label className="text-xs font-medium text-foreground">{column.name}</label>
        {required ? (
          <Badge variant="secondary" className="text-[10px]">
            Required
          </Badge>
        ) : column.default ? (
          <Badge variant="outline" className="text-[10px]">
            Default
          </Badge>
        ) : null}
        <span className="text-[11px] text-muted-foreground">{column.type}</span>
      </div>
    );

    const nullToggle = column.nullable ? (
      <Button
        type="button"
        variant="outline"
        size="xs"
        onClick={() => setField(column.name, { isNull: !field.isNull })}
        className={cn("text-[10px]", field.isNull && "bg-muted text-foreground")}
      >
        NULL
      </Button>
    ) : null;

    let control;
    if (column.editor === "bool") {
      control = (
        <Select
          value={field.value === "" ? (required ? undefined : DEFAULT_SENTINEL) : field.value}
          onValueChange={(value) =>
            setField(column.name, {
              value: value === DEFAULT_SENTINEL ? "" : (value ?? ""),
              isNull: false,
            })
          }
        >
          <SelectTrigger size="sm" className="h-9 w-full text-sm">
            <SelectValue placeholder={required ? "Select a value" : "Use default"} />
          </SelectTrigger>
          <SelectContent>
            {!required && <SelectItem value={DEFAULT_SENTINEL}>Use default</SelectItem>}
            <SelectItem value="true">true</SelectItem>
            <SelectItem value="false">false</SelectItem>
          </SelectContent>
        </Select>
      );
    } else if (column.editor === "enum" && column.enum_name) {
      control = (
        <Select
          value={field.value === "" ? (required ? undefined : DEFAULT_SENTINEL) : field.value}
          onValueChange={(value) =>
            setField(column.name, {
              value: value === DEFAULT_SENTINEL ? "" : (value ?? ""),
              isNull: false,
            })
          }
        >
          <SelectTrigger size="sm" className="h-9 w-full text-sm">
            <SelectValue placeholder={required ? "Select a value" : "Use default"} />
          </SelectTrigger>
          <SelectContent>
            {!required && <SelectItem value={DEFAULT_SENTINEL}>Use default</SelectItem>}
            {enumValues.map((value) => (
              <SelectItem key={value} value={value}>
                {value}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      );
    } else if (column.editor === "json" || column.editor === "jsonb") {
      control = (
        <Textarea
          rows={4}
          value={field.value}
          onChange={(event) => setField(column.name, { value: event.target.value, isNull: false })}
          placeholder={required ? "{}" : "Leave blank to use default / omit"}
          className="min-h-24 font-mono text-xs"
        />
      );
    } else {
      let inputType = "text";
      if (column.editor === "date") inputType = "date";
      if (column.editor === "timestamp" || column.editor === "timestamptz")
        inputType = "datetime-local";
      if (column.editor === "int" || column.editor === "float") inputType = "number";

      control = (
        <Input
          type={inputType}
          value={field.value}
          step={column.editor === "int" ? 1 : undefined}
          onChange={(event) => setField(column.name, { value: event.target.value, isNull: false })}
          placeholder={required ? "Enter a value" : "Leave blank to use default / omit"}
          className="h-9"
        />
      );
    }

    return (
      <div key={column.name} className="rounded-lg border border-border/80 bg-background/70 p-3">
        {label}
        <div className="flex items-start gap-2">
          <div className="min-w-0 flex-1">
            {field.isNull ? (
              <div className="flex h-9 items-center rounded-md border border-dashed border-border px-3 text-sm italic text-muted-foreground">
                NULL
              </div>
            ) : (
              control
            )}
          </div>
          {nullToggle}
        </div>
      </div>
    );
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(nextOpen) => {
        if (!nextOpen && !saving) onClose();
      }}
    >
      <DialogContent className="max-w-3xl p-0 sm:max-w-3xl">
        <DialogHeader className="px-5 pt-5">
          <DialogTitle>
            Insert row into {table.schema}.{table.name}
          </DialogTitle>
          <DialogDescription>
            Required columns are shown first. Optional values can be left blank to let Postgres use
            defaults.
          </DialogDescription>
        </DialogHeader>

        <div className="max-h-[70vh] overflow-y-auto px-5 pb-5">
          {requiredColumns.length > 0 ? (
            <div className="space-y-3">{requiredColumns.map(renderField)}</div>
          ) : (
            <div className="rounded-lg border border-border/80 bg-background/70 p-3 text-sm text-muted-foreground">
              This table does not require any manual values. You can insert defaults only, or expand
              optional columns to set additional fields.
            </div>
          )}

          {optionalColumns.length > 0 && (
            <div className="mt-4 space-y-3">
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => setShowOptional((value) => !value)}
              >
                {showOptional ? "Hide" : "Show"} optional columns ({optionalColumns.length})
              </Button>
              {showOptional && <div className="space-y-3">{optionalColumns.map(renderField)}</div>}
            </div>
          )}

          {error && <div className="mt-4 text-sm text-destructive">{error}</div>}
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={onClose} disabled={saving}>
            Cancel
          </Button>
          <Button type="button" onClick={() => void submit()} disabled={saving}>
            {saving ? "Inserting…" : "Insert row"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
