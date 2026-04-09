import { useState } from "react";

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
  const matches = input === String(affectedCount);

  return (
    <div className="fixed inset-0 bg-black/40 flex items-center justify-center z-50">
      <div className="bg-white rounded-lg shadow-xl w-full max-w-md p-6">
        <h2 className="text-base font-semibold text-slate-800 mb-2">Confirm bulk operation</h2>
        <p className="text-sm text-slate-600 mb-4">
          This update will affect{" "}
          <strong className="text-slate-900">{affectedCount}</strong> rows.
        </p>
        {sql && (
          <pre className="text-[11px] bg-slate-50 border border-slate-200 rounded p-2 mb-4 overflow-x-auto text-slate-700 font-mono">
            {sql}
          </pre>
        )}
        <p className="text-xs text-slate-500 mb-2">
          Type <strong>{affectedCount}</strong> to confirm:
        </p>
        <input
          type="text"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          className="w-full border border-slate-300 rounded px-2 py-1.5 text-sm mb-4 focus:outline-none focus:border-blue-400"
          placeholder={String(affectedCount)}
          autoFocus
        />
        <div className="flex gap-2 justify-end">
          <button
            onClick={onCancel}
            className="px-3 py-1.5 text-sm border border-slate-200 rounded text-slate-600 hover:bg-slate-50"
          >
            Cancel
          </button>
          <button
            onClick={() => matches && onConfirm(affectedCount)}
            disabled={!matches}
            className="px-3 py-1.5 text-sm bg-red-600 text-white rounded disabled:opacity-40 hover:bg-red-700"
          >
            Confirm
          </button>
        </div>
      </div>
    </div>
  );
}
