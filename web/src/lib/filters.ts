import { isISODate } from "./time";
import type { SignalFilter } from "./types";

export type SearchParamsLike = URLSearchParams | Record<string, string | string[] | undefined>;

export function first(v: string | string[] | undefined): string | undefined {
  return Array.isArray(v) ? v[0] : v;
}

function all(p: SearchParamsLike, key: string): string[] {
  if (p instanceof URLSearchParams) return p.getAll(key);
  const v = p[key];
  return v === undefined ? [] : Array.isArray(v) ? v : [v];
}

export function baseFilter(overrides: Partial<SignalFilter> = {}): SignalFilter {
  return { strategies: [], signals: [], sort: "time", order: "desc", ...overrides };
}

export function parseFilter(p: SearchParamsLike, defaults: Partial<SignalFilter> = {}): SignalFilter {
  const get = (k: string) => all(p, k)[0];
  let from = isISODate(get("from")) ? get("from") : defaults.from;
  let to = isISODate(get("to")) ? get("to") : defaults.to;
  if (from && to && from > to) [from, to] = [to, from];

  const direction = get("direction");
  const sort = get("sort");
  const symbol = (get("symbol") ?? "").toUpperCase().replace(/[^A-Z0-9&-]/g, "");

  return {
    ...baseFilter(),
    from,
    to,
    strategies: all(p, "strategy").filter(Boolean),
    signals: all(p, "signal").filter(Boolean),
    direction: direction === "long" || direction === "short" ? direction : undefined,
    symbol: symbol || undefined,
    sort: sort === "symbol" || sort === "signal" ? sort : "time",
    order: get("order") === "asc" ? "asc" : "desc",
  };
}

export function filterToParams(f: SignalFilter): URLSearchParams {
  const p = new URLSearchParams();
  if (f.from) p.set("from", f.from);
  if (f.to) p.set("to", f.to);
  for (const s of f.strategies) p.append("strategy", s);
  for (const s of f.signals) p.append("signal", s);
  if (f.direction) p.set("direction", f.direction);
  if (f.symbol) p.set("symbol", f.symbol);
  if (f.sort !== "time") p.set("sort", f.sort);
  if (f.order !== "desc") p.set("order", f.order);
  return p;
}
