import { STRATEGIES, shortSignalName } from "@/lib/catalog";
import type { SignalFilter } from "@/lib/types";

const input = "rounded-md border border-slate-300 bg-transparent px-2 py-1.5 text-sm dark:border-slate-700";

export function HistoryFilters({ filter, today }: { filter: SignalFilter; today: string }) {
  return (
    <details open className="rounded-lg border border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
      <summary className="cursor-pointer select-none px-4 py-2 text-sm font-medium">Filters</summary>
      <form method="get" action="/history" className="space-y-4 border-t border-slate-200 p-4 dark:border-slate-800">
        <div className="flex flex-wrap gap-3">
          <label className="flex flex-col gap-1 text-xs text-slate-500">
            From
            <input type="date" name="from" defaultValue={filter.from} max={today} className={input} />
          </label>
          <label className="flex flex-col gap-1 text-xs text-slate-500">
            To
            <input type="date" name="to" defaultValue={filter.to} max={today} className={input} />
          </label>
          <label className="flex flex-col gap-1 text-xs text-slate-500">
            Direction
            <select name="direction" defaultValue={filter.direction ?? ""} className={input}>
              <option value="">Both</option>
              <option value="long">Long</option>
              <option value="short">Short</option>
            </select>
          </label>
          <label className="flex flex-col gap-1 text-xs text-slate-500">
            Symbol starts with
            <input
              type="text"
              name="symbol"
              defaultValue={filter.symbol}
              placeholder="e.g. TATA"
              className={`${input} uppercase placeholder:normal-case`}
            />
          </label>
        </div>

        <div className="grid gap-4 sm:grid-cols-2">
          {Object.entries(STRATEGIES).map(([strategy, signals]) => (
            <fieldset key={strategy} className="space-y-1">
              <label className="flex items-center gap-2 text-sm font-medium">
                <input
                  type="checkbox"
                  name="strategy"
                  value={strategy}
                  defaultChecked={filter.strategies.includes(strategy)}
                />
                {strategy} (all)
              </label>
              <div className="ml-6 grid gap-0.5">
                {signals.map((signal) => (
                  <label key={signal} className="flex items-center gap-2 text-sm">
                    <input type="checkbox" name="signal" value={signal} defaultChecked={filter.signals.includes(signal)} />
                    {shortSignalName(signal, strategy)}
                  </label>
                ))}
              </div>
            </fieldset>
          ))}
        </div>

        <input type="hidden" name="sort" value={filter.sort} />
        <input type="hidden" name="order" value={filter.order} />
        <div className="flex gap-2">
          <button className="rounded-md bg-slate-900 px-4 py-1.5 text-sm font-medium text-white dark:bg-slate-100 dark:text-slate-900">
            Apply
          </button>
          <a href="/history" className="rounded-md border border-slate-300 px-4 py-1.5 text-sm dark:border-slate-700">
            Reset
          </a>
        </div>
      </form>
    </details>
  );
}
