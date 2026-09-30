import { ExportButton } from "@/components/ExportButton";
import { HistoryFilters } from "@/components/HistoryFilters";
import { Pager } from "@/components/Pager";
import { SignalsTable } from "@/components/SignalsTable";
import { filterToParams, first, parseFilter } from "@/lib/filters";
import { getStore } from "@/lib/store";
import { tradeMarksOrEmpty } from "@/lib/store/marks";
import { addDays, todayIST } from "@/lib/time";
import type { SignalFilter, SortKey } from "@/lib/types";

type SP = Promise<Record<string, string | string[] | undefined>>;
const PAGE_SIZE = 100;

export default async function HistoryPage({ searchParams }: { searchParams: SP }) {
  const sp = await searchParams;
  const today = todayIST();
  const filter = parseFilter(sp, { from: addDays(today, -6), to: today });
  const requested = Math.max(1, Number.parseInt(first(sp.page) ?? "1", 10) || 1);

  const store = getStore();
  let { rows, total } = await store.list(filter, (requested - 1) * PAGE_SIZE, PAGE_SIZE);
  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  // A stale link (e.g. page 5 after narrowing the filters) lands on the last page instead of an empty one.
  const page = Math.min(requested, pages);
  if (page !== requested) ({ rows, total } = await store.list(filter, (page - 1) * PAGE_SIZE, PAGE_SIZE));
  const marks = await tradeMarksOrEmpty(store, rows.map((r) => r.id));

  const withParams = (f: SignalFilter, p?: number) => {
    const params = filterToParams(f);
    if (p && p > 1) params.set("page", String(p));
    return `/history?${params}`;
  };
  const sortLinks = Object.fromEntries(
    (["time", "symbol", "signal"] as SortKey[]).map((key) => [
      key,
      withParams({ ...filter, sort: key, order: filter.sort === key && filter.order === "desc" ? "asc" : "desc" }),
    ]),
  );

  return (
    <>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold">History</h1>
          <p className="text-sm text-slate-500">
            {total.toLocaleString("en-US")} signal{total === 1 ? "" : "s"} · {filter.from ?? "start"} →{" "}
            {filter.to ?? "today"}
          </p>
        </div>
        <ExportButton filter={filter} />
      </div>
      <HistoryFilters filter={filter} today={today} />
      <SignalsTable
        rows={rows}
        showDate
        sortLinks={sortLinks}
        activeSort={filter.sort}
        order={filter.order}
        marks={marks}
        emptyText="No signals match these filters."
      />
      <Pager page={page} pages={pages} href={(p) => withParams(filter, p)} />
    </>
  );
}
