export function KiteFlash({ kite, reason }: { kite?: string; reason?: string }) {
  if (kite === "ok") {
    return (
      <div className="rounded-md bg-emerald-100 px-4 py-2 text-sm text-emerald-900 dark:bg-emerald-950 dark:text-emerald-200">
        ✓ Kite token saved. The engine picks it up within a minute.
      </div>
    );
  }
  if (kite === "error") {
    return (
      <div className="rounded-md bg-rose-100 px-4 py-2 text-sm text-rose-900 dark:bg-rose-950 dark:text-rose-200">
        Kite login failed: {reason ?? "unknown error"}
      </div>
    );
  }
  return null;
}
