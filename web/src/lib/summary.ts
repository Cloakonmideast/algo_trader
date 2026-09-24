import { STRATEGIES, STRATEGY_NAMES } from "./catalog";
import type { Signal } from "./types";

export interface SignalCount {
  strategy: string;
  signal: string;
  long: number;
  short: number;
  total: number;
}

export interface StrategyCount {
  strategy: string;
  long: number;
  short: number;
  total: number;
  signals: SignalCount[];
}

const strategyRank = (s: string) => {
  const i = STRATEGY_NAMES.indexOf(s);
  return i === -1 ? Number.MAX_SAFE_INTEGER : i;
};

const byStrategy = (a: { strategy: string }, b: { strategy: string }) =>
  strategyRank(a.strategy) - strategyRank(b.strategy) || a.strategy.localeCompare(b.strategy);

export function summarize(rows: Signal[]): StrategyCount[] {
  const strategies = new Map<string, StrategyCount>();
  const signals = new Map<string, SignalCount>();

  for (const r of rows) {
    let s = strategies.get(r.strategy);
    if (!s) {
      s = { strategy: r.strategy, long: 0, short: 0, total: 0, signals: [] };
      strategies.set(r.strategy, s);
    }
    let g = signals.get(r.signal);
    if (!g) {
      g = { strategy: r.strategy, signal: r.signal, long: 0, short: 0, total: 0 };
      signals.set(r.signal, g);
      s.signals.push(g);
    }
    s[r.direction]++;
    s.total++;
    g[r.direction]++;
    g.total++;
  }

  const out = [...strategies.values()].sort(byStrategy);
  for (const s of out) s.signals.sort((a, b) => b.total - a.total || a.signal.localeCompare(b.signal));
  return out;
}

/** Sidebar model: every catalog signal in catalog order (zero counts included), then anything unknown. */
export function withCatalog(counts: StrategyCount[]): StrategyCount[] {
  const seen = new Map(counts.map((c) => [c.strategy, c]));
  const names = [...STRATEGY_NAMES, ...counts.map((c) => c.strategy).filter((s) => !STRATEGIES[s])];

  return names.map((strategy) => {
    const c = seen.get(strategy);
    const catalog = STRATEGIES[strategy] ?? [];
    const bySignal = new Map((c?.signals ?? []).map((g) => [g.signal, g]));
    const known = catalog.map((signal) => bySignal.get(signal) ?? { strategy, signal, long: 0, short: 0, total: 0 });
    const extra = (c?.signals ?? []).filter((g) => !catalog.includes(g.signal));
    return {
      strategy,
      long: c?.long ?? 0,
      short: c?.short ?? 0,
      total: c?.total ?? 0,
      signals: [...known, ...extra],
    };
  });
}
