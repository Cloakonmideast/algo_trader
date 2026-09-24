import { shortSignalName } from "@/lib/catalog";
import { istTime } from "@/lib/time";
import type { Signal, SortKey } from "@/lib/types";

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

export function SignalsTable({
  rows,
  showDate = false,
  sortLinks,
  activeSort,
  order,
  emptyText = "No signals.",
}: {
  rows: Signal[];
  showDate?: boolean;
  sortLinks?: Partial<Record<SortKey, string>>;
  activeSort?: SortKey;
  order?: "asc" | "desc";
  emptyText?: string;
}) {
  if (rows.length === 0) {
    return (
      <p className="rounded-lg border border-dashed border-slate-300 p-6 text-center text-sm text-slate-500 dark:border-slate-700">
        {emptyText}
      </p>
    );
  }

  const head = (label: string, key?: SortKey) => {
    const href = key ? sortLinks?.[key] : undefined;
    if (!href) return label;
    const arrow = activeSort === key ? (order === "asc" ? " ↑" : " ↓") : "";
    return (
      <a href={href} className="hover:underline">
        {label}
        {arrow}
      </a>
    );
  };

  return (
    <div className="overflow-x-auto rounded-lg border border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
      <table className="min-w-full text-sm">
        <thead className="bg-slate-100 text-left text-xs uppercase tracking-wide text-slate-600 dark:bg-slate-800 dark:text-slate-300">
          <tr>
            {showDate && <th className="px-3 py-2">Date</th>}
            <th className="px-3 py-2">{head("Time", "time")}</th>
            <th className="px-3 py-2">{head("Symbol", "symbol")}</th>
            <th className="px-3 py-2">{head("Signal", "signal")}</th>
            <th className="px-3 py-2">Side</th>
            <th className="px-3 py-2 text-right">Price</th>
            <th className="px-3 py-2 text-right">RSI D / W / M</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-100 dark:divide-slate-800">
          {rows.map((r) => (
            <tr key={r.id} className="hover:bg-slate-50 dark:hover:bg-slate-800/50">
              {showDate && <td className="whitespace-nowrap px-3 py-2 tabular-nums">{r.tradeDate}</td>}
              <td className="whitespace-nowrap px-3 py-2 tabular-nums">{istTime(r.firedAt)}</td>
              <td className="whitespace-nowrap px-3 py-2 font-medium">{r.symbol}</td>
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
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
