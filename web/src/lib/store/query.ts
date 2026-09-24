import type { Direction, Signal, SignalFilter } from "@/lib/types";

export const SIGNAL_COLUMNS =
  "id,fired_at,trade_date,token,symbol,strategy,signal,direction,price,rsi_daily,rsi_weekly,rsi_monthly";

export interface SignalRow {
  id: number;
  fired_at: string;
  trade_date: string;
  token: number;
  symbol: string;
  strategy: string;
  signal: string;
  direction: Direction;
  price: number | string;
  rsi_daily: number | string | null;
  rsi_weekly: number | string | null;
  rsi_monthly: number | string | null;
}

const num = (v: number | string | null): number | null => (v === null ? null : Number(v));

export function toSignal(r: SignalRow): Signal {
  return {
    id: r.id,
    firedAt: r.fired_at,
    tradeDate: r.trade_date,
    token: r.token,
    symbol: r.symbol,
    strategy: r.strategy,
    signal: r.signal,
    direction: r.direction,
    price: Number(r.price),
    rsiDaily: num(r.rsi_daily),
    rsiWeekly: num(r.rsi_weekly),
    rsiMonthly: num(r.rsi_monthly),
  };
}

/** The subset of the PostgREST builder this module drives, so it can be tested with a fake. */
export interface FilterableQuery {
  gte(column: string, value: string): FilterableQuery;
  lte(column: string, value: string): FilterableQuery;
  eq(column: string, value: string): FilterableQuery;
  or(filters: string): FilterableQuery;
  like(column: string, pattern: string): FilterableQuery;
  order(column: string, options: { ascending: boolean }): FilterableQuery;
}

/** Double-quote a value for a PostgREST `in.(...)` list; names contain spaces and parentheses. */
export function pgQuote(value: string): string {
  return `"${value.replace(/\\/g, "\\\\").replace(/"/g, '\\"')}"`;
}

export function selectionFilter(f: SignalFilter): string | null {
  const parts: string[] = [];
  if (f.strategies.length) parts.push(`strategy.in.(${f.strategies.map(pgQuote).join(",")})`);
  if (f.signals.length) parts.push(`signal.in.(${f.signals.map(pgQuote).join(",")})`);
  return parts.length ? parts.join(",") : null;
}

/** Applies the filter and ordering. Every builder method returns the same builder, so the result is still a T. */
export function applyFilterToQuery<T extends FilterableQuery>(query: T, f: SignalFilter): T {
  let q: FilterableQuery = query;
  if (f.from) q = q.gte("trade_date", f.from);
  if (f.to) q = q.lte("trade_date", f.to);
  const selection = selectionFilter(f);
  if (selection) q = q.or(selection);
  if (f.direction) q = q.eq("direction", f.direction);
  if (f.symbol) q = q.like("symbol", `${f.symbol}%`); // parseFilter guarantees [A-Z0-9&-]

  const asc = f.order === "asc";
  if (f.sort === "symbol" || f.sort === "signal") {
    q = q.order(f.sort, { ascending: asc }).order("fired_at", { ascending: false }).order("id", { ascending: false });
  } else {
    q = q.order("fired_at", { ascending: asc }).order("id", { ascending: asc });
  }
  return q as T;
}
