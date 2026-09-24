import { shortSignalName } from "@/lib/catalog";
import type { StrategyCount } from "@/lib/summary";

export function StrategySidebar({
  model,
  date,
  strategy,
  signal,
}: {
  model: StrategyCount[];
  date: string;
  strategy?: string;
  signal?: string;
}) {
  const href = (p: Record<string, string>) => `/strategies?${new URLSearchParams({ date, ...p })}`;
  const item = (active: boolean) =>
    `flex items-center justify-between gap-2 rounded-md px-2 py-1 text-sm ${
      active ? "bg-slate-900 text-white dark:bg-slate-100 dark:text-slate-900" : "hover:bg-slate-200 dark:hover:bg-slate-800"
    }`;
  const count = (n: number) => <span className={`tabular-nums ${n === 0 ? "opacity-40" : ""}`}>{n}</span>;

  return (
    <nav className="w-full shrink-0 space-y-4 md:w-72">
      <a href={href({})} className={item(!strategy && !signal)}>
        <span className="font-medium">All strategies</span>
        {count(model.reduce((n, s) => n + s.total, 0))}
      </a>
      {model.map((s) => (
        <div key={s.strategy} className="space-y-1">
          <a href={href({ strategy: s.strategy })} className={item(strategy === s.strategy && !signal)}>
            <span className="font-medium">{s.strategy}</span>
            {count(s.total)}
          </a>
          <ul className="ml-3 space-y-0.5 border-l border-slate-200 pl-2 dark:border-slate-800">
            {s.signals.map((g) => (
              <li key={g.signal}>
                <a href={href({ signal: g.signal })} className={item(signal === g.signal)}>
                  <span className="truncate">{shortSignalName(g.signal, s.strategy)}</span>
                  {count(g.total)}
                </a>
              </li>
            ))}
          </ul>
        </div>
      ))}
    </nav>
  );
}
