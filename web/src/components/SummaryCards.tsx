import { summarize } from "@/lib/summary";
import type { Signal } from "@/lib/types";

export function SummaryCards({ rows }: { rows: Signal[] }) {
  const counts = summarize(rows);
  const long = rows.filter((r) => r.direction === "long").length;
  const cards = [{ strategy: "All signals", total: rows.length, long, short: rows.length - long }, ...counts];

  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
      {cards.map((c) => (
        <div
          key={c.strategy}
          className="rounded-lg border border-slate-200 bg-white p-3 dark:border-slate-800 dark:bg-slate-900"
        >
          <div className="truncate text-xs text-slate-500">{c.strategy}</div>
          <div className="text-2xl font-semibold tabular-nums">{c.total}</div>
          <div className="text-xs tabular-nums">
            <span className="text-emerald-700 dark:text-emerald-400">{c.long} long</span>
            {" · "}
            <span className="text-rose-700 dark:text-rose-400">{c.short} short</span>
          </div>
        </div>
      ))}
    </div>
  );
}
