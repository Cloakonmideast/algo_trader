import { addDays } from "@/lib/time";

export function DayNav({ date, today, extra }: { date: string; today: string; extra: Record<string, string> }) {
  const href = (d: string) => `/strategies?${new URLSearchParams({ date: d, ...extra })}`;
  const next = addDays(date, 1);
  const btn = "rounded-md border border-slate-300 px-3 py-1.5 text-sm dark:border-slate-700";

  return (
    <div className="flex flex-wrap items-center gap-2">
      <a href={href(addDays(date, -1))} className={btn} aria-label="Previous day">
        ◀
      </a>
      <form method="get" action="/strategies" className="flex items-center gap-2">
        {Object.entries(extra).map(([k, v]) => (
          <input key={k} type="hidden" name={k} value={v} />
        ))}
        <input type="date" name="date" defaultValue={date} max={today} className={`${btn} bg-transparent`} />
        <button className={btn}>Go</button>
      </form>
      {next <= today ? (
        <a href={href(next)} className={btn} aria-label="Next day">
          ▶
        </a>
      ) : (
        <span className={`${btn} opacity-40`}>▶</span>
      )}
      {date !== today && (
        <a href={href(today)} className="text-sm text-slate-600 underline dark:text-slate-400">
          Today
        </a>
      )}
    </div>
  );
}
