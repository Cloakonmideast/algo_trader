import { AddTradeForm } from "@/components/AddTradeForm";
import { TradesTable } from "@/components/TradesTable";
import { getStore } from "@/lib/store";
import { TRADES_CAP } from "@/lib/store/types";
import { todayIST } from "@/lib/time";
import { parseTradeFilter } from "@/lib/trades";
import type { Side } from "@/lib/types";

type SP = Promise<Record<string, string | string[] | undefined>>;

const TABS: { label: string; side?: Side }[] = [
  { label: "All" },
  { label: "Bought", side: "buy" },
  { label: "Sold", side: "sell" },
];

export default async function TradesPage({ searchParams }: { searchParams: SP }) {
  const filter = parseTradeFilter(await searchParams);
  const { rows, total } = await getStore().listTrades(filter, TRADES_CAP);
  const today = todayIST();

  const href = (side?: Side) => {
    const p = new URLSearchParams();
    if (side) p.set("side", side);
    if (filter.symbol) p.set("symbol", filter.symbol);
    const qs = p.toString();
    return qs ? `/trades?${qs}` : "/trades";
  };

  return (
    <>
      <div>
        <h1 className="text-xl font-semibold">Trades</h1>
        <p className="text-sm text-slate-500">
          {total.toLocaleString("en-US")} trade{total === 1 ? "" : "s"}
          {filter.symbol ? ` · stocks starting ${filter.symbol}` : ""}
        </p>
      </div>
      {/* Keyed by the day, so a page left open overnight gets a fresh form after a refresh. */}
      <AddTradeForm key={today} today={today} />
      <div className="flex flex-wrap items-center justify-between gap-3">
        <nav className="flex gap-1">
          {TABS.map((t) => (
            <a
              key={t.label}
              href={href(t.side)}
              className={`rounded-md px-3 py-1.5 text-sm font-medium ${
                filter.side === t.side
                  ? "bg-slate-900 text-white dark:bg-slate-100 dark:text-slate-900"
                  : "text-slate-600 hover:bg-slate-200 dark:text-slate-300 dark:hover:bg-slate-800"
              }`}
            >
              {t.label}
            </a>
          ))}
        </nav>
        <form method="get" action="/trades" className="flex gap-2">
          {filter.side && <input type="hidden" name="side" value={filter.side} />}
          <input
            type="text"
            name="symbol"
            defaultValue={filter.symbol}
            placeholder="Stock starts with…"
            className="w-44 rounded-md border border-slate-300 bg-transparent px-2 py-1.5 text-sm uppercase placeholder:normal-case dark:border-slate-700"
          />
          <button className="rounded-md border border-slate-300 px-3 py-1.5 text-sm dark:border-slate-700">Search</button>
        </form>
      </div>
      {total > rows.length && (
        <p className="text-xs text-slate-500">
          Showing the newest {rows.length.toLocaleString("en-US")} of {total.toLocaleString("en-US")}.
        </p>
      )}
      <TradesTable rows={rows} />
    </>
  );
}
