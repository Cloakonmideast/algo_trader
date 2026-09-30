"use client";

import { useMemo, useState } from "react";
import { shortSignalName } from "@/lib/catalog";
import { type SignalColumn, sortSignals } from "@/lib/filter-rows";
import { kiteChartUrl } from "@/lib/kite-chart";
import { nextSort, type SortState } from "@/lib/table-sort";
import { istTime } from "@/lib/time";
import type { Signal, SortKey, TradeMarks } from "@/lib/types";
import { SortHeader } from "./SortHeader";
import { TradeButtons } from "./TradeButtons";

const price = new Intl.NumberFormat("en-IN", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
const rsi = (v: number | null) => (v === null ? "—" : v.toFixed(1));

export function DirectionBadge({ direction }: { direction: Signal["direction"] }) {
  return direction === "long" ? (
    <span className="rounded bg-emerald-100 px-1.5 py-0.5 text-xs font-medium text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300">
      Long
    </span>
  ) : (
    <span className="rounded bg-rose-100 px-1.5 py-0.5 text-xs font-medium text-rose-800 dark:bg-rose-950 dark:text-rose-300">
      Short
    </span>
  );
}

const firstOrder = (key: SignalColumn) => (key === "time" ? "desc" : "asc");

/**
 * With `sortLinks` (History), the headers link to a server-sorted page. Without
 * them (Dashboard, Strategies), clicking a header sorts the rows here.
 */
export function SignalsTable({
  rows,
  showDate = false,
  sortLinks,
  activeSort,
  order,
  emptyText = "No signals.",
  marks,
}: {
  rows: Signal[];
  showDate?: boolean;
  sortLinks?: Partial<Record<SortKey, string>>;
  activeSort?: SortKey;
  order?: "asc" | "desc";
  emptyText?: string;
  marks?: TradeMarks;
}) {
  const [sort, setSort] = useState<SortState<SignalColumn>>({ key: "time", order: "desc" });
  const shown = useMemo(() => (sortLinks ? rows : sortSignals(rows, sort.key, sort.order)), [rows, sortLinks, sort]);

  if (rows.length === 0) {
    return (
      <p className="rounded-lg border border-dashed border-slate-300 p-6 text-center text-sm text-slate-500 dark:border-slate-700">
        {emptyText}
      </p>
    );
  }

  const head = (label: string, key: SignalColumn) => {
    if (!sortLinks) {
      return (
        <SortHeader
          label={label}
          active={sort.key === key}
          order={sort.order}
          onClick={() => setSort((cur) => nextSort(cur, key, firstOrder))}
        />
      );
    }
    const href = key !== "side" ? sortLinks[key as SortKey] : undefined;
    if (!href) return <th className="px-3 py-2">{label}</th>;
    const arrow = activeSort === key ? (order === "asc" ? " ↑" : " ↓") : "";
    return (
      <th className="px-3 py-2">
        <a href={href} className="hover:underline">
          {label}
          {arrow}
        </a>
      </th>
    );
  };

  return (
    <div className="overflow-x-auto rounded-lg border border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
      <table className="min-w-full text-sm">
        <thead className="bg-slate-100 text-left text-xs uppercase tracking-wide text-slate-600 dark:bg-slate-800 dark:text-slate-300">
          <tr>
            {showDate && <th className="px-3 py-2">Date</th>}
            {head("Time", "time")}
            {head("Symbol", "symbol")}
            {head("Signal", "signal")}
            {head("Side", "side")}
            <th className="px-3 py-2 text-right">Price</th>
            <th className="px-3 py-2 text-right">RSI D / W / M</th>
            <th className="px-3 py-2">Trade</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-100 dark:divide-slate-800">
          {shown.map((r) => (
            <tr key={r.id} className="hover:bg-slate-50 dark:hover:bg-slate-800/50">
              {showDate && <td className="whitespace-nowrap px-3 py-2 tabular-nums">{r.tradeDate}</td>}
              <td className="whitespace-nowrap px-3 py-2 tabular-nums">{istTime(r.firedAt)}</td>
              <td className="whitespace-nowrap px-3 py-2 font-medium">
                <a
                  href={kiteChartUrl(r.symbol, r.token)}
                  target="_blank"
                  rel="noopener noreferrer"
                  title={`Open ${r.symbol} on Kite`}
                  className="hover:underline"
                >
                  {r.symbol}
                </a>
              </td>
              <td className="whitespace-nowrap px-3 py-2">
                {shortSignalName(r.signal, r.strategy)}
                <span className="ml-2 text-xs text-slate-500">{r.strategy}</span>
              </td>
              <td className="px-3 py-2">
                <DirectionBadge direction={r.direction} />
              </td>
              <td className="whitespace-nowrap px-3 py-2 text-right tabular-nums">{price.format(r.price)}</td>
              <td className="whitespace-nowrap px-3 py-2 text-right tabular-nums text-slate-600 dark:text-slate-400">
                {r.rsiDaily === null && r.rsiWeekly === null && r.rsiMonthly === null
                  ? "—"
                  : `${rsi(r.rsiDaily)} / ${rsi(r.rsiWeekly)} / ${rsi(r.rsiMonthly)}`}
              </td>
              <td className="px-3 py-2">
                <TradeButtons signalId={r.id} initial={marks?.[r.id]} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
