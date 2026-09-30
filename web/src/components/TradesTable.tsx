"use client";

import { useMemo, useState } from "react";
import { nextSort, type SortState } from "@/lib/table-sort";
import { signalSource, sortTrades, type TradeColumn } from "@/lib/trades";
import type { Trade } from "@/lib/types";
import { RemoveTradeButton } from "./RemoveTradeButton";
import { SortHeader } from "./SortHeader";

const price = new Intl.NumberFormat("en-IN", { minimumFractionDigits: 2, maximumFractionDigits: 2 });

function SideBadge({ side }: { side: Trade["side"] }) {
  return side === "buy" ? (
    <span className="rounded bg-emerald-100 px-1.5 py-0.5 text-xs font-medium text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300">
      Bought
    </span>
  ) : (
    <span className="rounded bg-rose-100 px-1.5 py-0.5 text-xs font-medium text-rose-800 dark:bg-rose-950 dark:text-rose-300">
      Sold
    </span>
  );
}

const firstOrder = (key: TradeColumn) => (key === "date" ? "desc" : "asc");

/** Rows arrive newest first; clicking a header sorts them here. */
export function TradesTable({ rows }: { rows: Trade[] }) {
  const [sort, setSort] = useState<SortState<TradeColumn>>({ key: "date", order: "desc" });
  const shown = useMemo(() => sortTrades(rows, sort.key, sort.order), [rows, sort]);
  const head = (label: string, key: TradeColumn) => (
    <SortHeader
      label={label}
      active={sort.key === key}
      order={sort.order}
      onClick={() => setSort((cur) => nextSort(cur, key, firstOrder))}
    />
  );

  if (rows.length === 0) {
    return (
      <p className="rounded-lg border border-dashed border-slate-300 p-6 text-center text-sm text-slate-500 dark:border-slate-700">
        No trades yet. Use Buy or Sell on a signal, or add one above.
      </p>
    );
  }

  return (
    <div className="overflow-x-auto rounded-lg border border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
      <table className="min-w-full text-sm">
        <thead className="bg-slate-100 text-left text-xs uppercase tracking-wide text-slate-600 dark:bg-slate-800 dark:text-slate-300">
          <tr>
            {head("Date", "date")}
            {head("Stock", "stock")}
            {head("Side", "side")}
            <th className="px-3 py-2 text-right">Price</th>
            {head("From", "from")}
            <th className="px-3 py-2" />
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-100 dark:divide-slate-800">
          {shown.map((t) => {
            const src = signalSource(t.signal);
            return (
              <tr key={t.id} className="hover:bg-slate-50 dark:hover:bg-slate-800/50">
                <td className="whitespace-nowrap px-3 py-2 tabular-nums">{t.tradeDate}</td>
                <td className="whitespace-nowrap px-3 py-2 font-medium">{t.symbol}</td>
                <td className="px-3 py-2">
                  <SideBadge side={t.side} />
                </td>
                <td className="whitespace-nowrap px-3 py-2 text-right tabular-nums">
                  {t.price === null ? "—" : price.format(t.price)}
                </td>
                <td className="whitespace-nowrap px-3 py-2">
                  {src ? (
                    <>
                      {src.name}
                      <span className="ml-2 text-xs text-slate-500">{src.family}</span>
                    </>
                  ) : (
                    <span className="text-slate-500">Manual</span>
                  )}
                </td>
                <td className="px-3 py-2 text-right">
                  <RemoveTradeButton id={t.id} label={`the ${t.symbol} ${t.side} on ${t.tradeDate}`} />
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
