import { getStore } from "@/lib/store";
import { ago, engineHealth } from "@/lib/time";
import { NavLinks } from "./NavLinks";

const DOT = { ok: "bg-emerald-500", stale: "bg-amber-500", down: "bg-rose-500" } as const;

export async function Header() {
  const status = await getStore().status();
  const health = engineHealth(status.engine?.lastSeen ?? null);

  return (
    <header className="border-b border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
      <div className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-3 px-4 py-3">
        <div className="flex items-center gap-4">
          <span className="font-semibold">AlgoTrader</span>
          <NavLinks />
        </div>
        <div className="flex flex-wrap items-center gap-3 text-sm">
          <span
            className="flex items-center gap-1.5"
            title={status.engine ? `${status.engine.instruments} instruments` : undefined}
          >
            <span className={`h-2 w-2 rounded-full ${DOT[health]}`} />
            Engine {status.engine ? ago(status.engine.lastSeen) : "never seen"}
          </span>
          {status.tokenValidToday ? (
            <span className="text-emerald-700 dark:text-emerald-400">✓ Kite token valid</span>
          ) : (
            <a
              href="/api/kite/login"
              className="rounded-md bg-amber-500 px-3 py-1.5 font-medium text-white hover:bg-amber-600"
            >
              Log in to Kite
            </a>
          )}
        </div>
      </div>
    </header>
  );
}
