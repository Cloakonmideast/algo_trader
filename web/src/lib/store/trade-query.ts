import type { Side, Trade, TradeFilter } from "@/lib/types";
import type { FilterableQuery } from "./query";

export const TRADE_COLUMNS = "id,created_at,trade_date,symbol,side,price,signal_id,signal";

export interface TradeRow {
  id: number;
  created_at: string;
  trade_date: string;
  symbol: string;
  side: Side;
  price: number | string | null;
  signal_id: number | null;
  signal: string | null;
}

export function toTrade(r: TradeRow): Trade {
  return {
    id: r.id,
    createdAt: r.created_at,
    tradeDate: r.trade_date,
    symbol: r.symbol,
    side: r.side,
    price: r.price === null ? null : Number(r.price),
    signalId: r.signal_id,
    signal: r.signal,
  };
}

/** Filters by side and symbol prefix and orders newest first. Every builder method returns the same builder. */
export function applyTradeFilter<T extends FilterableQuery>(query: T, f: TradeFilter): T {
  let q: FilterableQuery = query;
  if (f.side) q = q.eq("side", f.side);
  if (f.symbol) q = q.like("symbol", `${f.symbol}%`); // parseTradeFilter guarantees [A-Z0-9&-]
  return q.order("trade_date", { ascending: false }).order("id", { ascending: false }) as T;
}
