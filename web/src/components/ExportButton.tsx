"use client";

import { useState } from "react";
import { fetchAllSignals } from "@/lib/export-fetch";
import type { SignalFilter } from "@/lib/types";

export function ExportButton({ filter, label = "Export to Excel" }: { filter: SignalFilter; label?: string }) {
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function run() {
    setError(null);
    setBusy("Fetching…");
    try {
      const rows = await fetchAllSignals(filter, fetch, (done, total) => setBusy(`Fetching ${done}/${total}…`));
      if (rows.length === 0) {
        setError("Nothing to export for this selection.");
        return;
      }
      setBusy("Building file…");
      const { buildWorkbook, workbookBlob, exportFileName } = await import("@/lib/excel");
      const blob = await workbookBlob(buildWorkbook(rows));
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = exportFileName(filter);
      a.click();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(null);
    }
  }

  return (
    <span className="inline-flex flex-col items-end gap-1">
      <button
        type="button"
        onClick={run}
        disabled={busy !== null}
        className="rounded-md border border-emerald-600 px-3 py-1.5 text-sm font-medium text-emerald-700 hover:bg-emerald-50 disabled:opacity-60 dark:text-emerald-400 dark:hover:bg-emerald-950"
      >
        {busy ?? label}
      </button>
      {error && <span className="max-w-xs text-right text-xs text-red-600">{error}</span>}
    </span>
  );
}
