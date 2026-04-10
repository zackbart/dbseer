// Tiny typed wrapper around localStorage with JSON serialization and
// fail-safe defaults. Used for UI preferences (sidebar width, collapsed
// schemas, column widths) that should persist across reloads but should
// never crash the app if localStorage is unavailable, full, or contains
// garbage from a previous version.

export function getJSON<T>(key: string, fallback: T): T {
  try {
    const raw = localStorage.getItem(key);
    if (raw === null) return fallback;
    return JSON.parse(raw) as T;
  } catch {
    return fallback;
  }
}

export function setJSON<T>(key: string, value: T): void {
  try {
    localStorage.setItem(key, JSON.stringify(value));
  } catch {
    // Quota exceeded, private mode, etc — silently no-op.
  }
}
