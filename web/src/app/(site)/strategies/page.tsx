import { DayNav } from "@/components/DayNav";
import { ExportButton } from "@/components/ExportButton";
import { SignalsTable } from "@/components/SignalsTable";
import { StrategySidebar } from "@/components/StrategySidebar";
import { STRATEGIES, STRATEGY_NAMES, shortSignalName } from "@/lib/catalog";
import { applyFilter } from "@/lib/filter-rows";
import { baseFilter, first } from "@/lib/filters";
import { getStore } from "@/lib/store";
import { ALL_CAP } from "@/lib/store/types";
import { summarize, withCatalog } from "@/lib/summary";
import { isISODate, todayIST } from "@/lib/time";

type SP = Promise<Record<string, string | string[] | undefined>>;

export default async function StrategiesPage({ searchParams }: { searchParams: SP }) {
  const sp = await searchParams;
  const today = todayIST();
  const requested = first(sp.date);
  const date = isISODate(requested) && requested <= today ? requested : today;
  const signal = first(sp.signal) || undefined;
  const strategy = signal ? undefined : first(sp.strategy) || undefined;

  const dayRows = await getStore().listAll(baseFilter({ from: date, to: date }), ALL_CAP);
  const selection = baseFilter({
    from: date,
    to: date,
    strategies: strategy ? [strategy] : [],
    signals: signal ? [signal] : [],
  });
  const shown = applyFilter(dayRows, selection);

  // A signal with no fires that day has no row to read its family from, so fall back to the catalog.
  const family = signal
    ? (dayRows.find((r) => r.signal === signal)?.strategy ??
      STRATEGY_NAMES.find((s) => STRATEGIES[s].includes(signal)) ??
      "")
    : "";
  const title = signal ? `${family} · ${shortSignalName(signal, family)}` : (strategy ?? "All strategies");
  const extra: Record<string, string> = signal ? { signal } : strategy ? { strategy } : {};

  return (
    <div className="flex flex-col gap-6 md:flex-row">
      <StrategySidebar model={withCatalog(summarize(dayRows))} date={date} strategy={strategy} signal={signal} />
      <section className="min-w-0 flex-1 space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h1 className="text-xl font-semibold">{title}</h1>
            <p className="text-sm text-slate-500">
              {shown.length} signal{shown.length === 1 ? "" : "s"} on {date}
            </p>
          </div>
          <ExportButton filter={selection} />
        </div>
        <DayNav date={date} today={today} extra={extra} />
        <SignalsTable rows={shown} emptyText="No signals for this selection on this day." />
      </section>
    </div>
  );
}
