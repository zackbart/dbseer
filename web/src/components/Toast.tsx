import { useEffect } from "react";

export interface ToastItem {
  id: number;
  type: "error" | "success";
  message: string;
}

interface ToastListProps {
  toasts: ToastItem[];
  onDismiss: (id: number) => void;
}

const AUTO_DISMISS_MS = 3000;

export default function ToastList({ toasts, onDismiss }: ToastListProps) {
  return (
    <div className="fixed bottom-4 right-4 z-[60] flex flex-col gap-2 max-w-sm pointer-events-none">
      {toasts.map((t) => (
        <ToastCard key={t.id} toast={t} onDismiss={onDismiss} />
      ))}
    </div>
  );
}

function ToastCard({ toast, onDismiss }: { toast: ToastItem; onDismiss: (id: number) => void }) {
  useEffect(() => {
    if (toast.type === "success") {
      const timer = setTimeout(() => onDismiss(toast.id), AUTO_DISMISS_MS);
      return () => clearTimeout(timer);
    }
  }, [toast.id, toast.type, onDismiss]);

  const bg = toast.type === "error" ? "bg-red-50 border-red-200 text-red-800" : "bg-green-50 border-green-200 text-green-800";

  return (
    <div
      className={`pointer-events-auto border rounded-lg shadow-lg px-4 py-3 text-xs flex items-start gap-2 ${bg}`}
    >
      <span className="flex-1 break-words">{toast.message}</span>
      <button
        onClick={() => onDismiss(toast.id)}
        className="text-current opacity-50 hover:opacity-100 shrink-0 text-sm leading-none"
      >
        &times;
      </button>
    </div>
  );
}
