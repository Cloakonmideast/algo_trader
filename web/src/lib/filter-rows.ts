import type { Signal, SignalFilter } from "./types";

export function matchesFilter(row: Signal, f: SignalFilter): boolean {
  if (f.from && row.tradeDate < f.from) return false;
  if (f.to && row.tradeDate > f.to) return false;
  if (
    (f.strategies.length || f.signals.length) &&
    !f.strategies.includes(row.strategy) &&
    !f.signals.includes(row.signal)
  ) {
    return false;
  }
  if (f.direction && row.direction !== f.direction) return false;
  if (f.symbol && !row.symbol.startsWith(f.symbol)) return false;
  return true;
}

const newestFirst = (a: Signal, b: Signal) => b.firedAt.localeCompare(a.firedAt) || b.id - a.id;

function comparator(f: SignalFilter): (a: Signal, b: Signal) => number {
  const dir = f.order === "asc" ? 1 : -1;
  switch (f.sort) {
    case "symbol":
      return (a, b) => dir * a.symbol.localeCompare(b.symbol) || newestFirst(a, b);
    case "signal":
      return (a, b) => dir * a.signal.localeCompare(b.signal) || newestFirst(a, b);
    default:
      return (a, b) => dir * (a.firedAt.localeCompare(b.firedAt) || a.id - b.id);
  }
}

export function applyFilter(rows: Signal[], f: SignalFilter): Signal[] {
  return rows.filter((r) => matchesFilter(r, f)).sort(comparator(f));
}
