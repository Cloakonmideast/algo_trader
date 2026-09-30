"use client";

import { useEffect, useState } from "react";
import { baseFilter } from "@/lib/filters";
import { istTime } from "@/lib/time";
import type { Signal, TradeMarks } from "@/lib/types";
import { ExportButton } from "./ExportButton";
import { SignalsTable } from "./SignalsTable";
import { SummaryCards } from "./SummaryCards";

const REFRESH_MS = 30_000;

export function LiveSignals({ date, initialRows, marks }: { date: string; initialRows: Signal[]; marks: TradeMarks }) {
  const [rows, setRows] = useState(initialRows);
  const [updated, setUpdated] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    const refresh = async () => {
      try {
        const res = await fetch(`/api/signals?from=${date}&to=${date}&all=1`, { cache: "no-store" });
        if (!res.ok) return;
        const body = (await res.json()) as { rows: Signal[] };
        if (!cancelled) {
          setRows(body.rows);
          setUpdated(istTime(new Date()));
        }
      } catch {
        // Offline or redeploying; try again on the next tick.
      }
    };
    const id = setInterval(refresh, REFRESH_MS);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [date]);

  return (
    <div className="space-y-4">
      <SummaryCards rows={rows} />
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h2 className="text-lg font-semibold">Latest signals</h2>
          <p className="text-xs text-slate-500">{updated ? `Updated ${updated} IST` : "Refreshes every 30 seconds"}</p>
        </div>
        <ExportButton filter={baseFilter({ from: date, to: date })} label="Export today" />
      </div>
      <SignalsTable rows={rows} marks={marks} emptyText="No signals yet today." />
    </div>
  );
}
