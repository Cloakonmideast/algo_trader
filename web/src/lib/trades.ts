import { shortSignalName } from "./catalog";
import { first, type SearchParamsLike } from "./filters";
import { isISODate } from "./time";
import type { NewTrade, Side, Signal, TradeFilter, TradeInput, TradeMarks } from "./types";

/** NSE trading symbols: capitals, digits, "&" and "-" (e.g. M&M, BAJAJ-AUTO). Same rule as the SQL check. */
export const SYMBOL_RE = /^[A-Z0-9&-]{1,30}$/;

/** ₹10 crore. The column holds more, but a price this large is a typo. */
export const MAX_PRICE = 100_000_000;

export function normalizeSymbol(s: unknown): string | null {
  if (typeof s !== "string") return null;
  const v = s.trim().toUpperCase();
  return SYMBOL_RE.test(v) ? v : null;
}

function isSide(v: unknown): v is Side {
  return v === "buy" || v === "sell";
}

const blank = (v: unknown) => v === undefined || v === null || v === "";

/** Validates a POST /api/trades body. `today` is the IST date, YYYY-MM-DD. */
export function parseTradeInput(
  body: unknown,
  today: string,
): { ok: true; value: TradeInput } | { ok: false; error: string } {
  if (typeof body !== "object" || body === null) return { ok: false, error: "Send the trade as a JSON object." };
  const b = body as Record<string, unknown>;
  if (!isSide(b.side)) return { ok: false, error: 'The side must be "buy" or "sell".' };

  if (b.signalId !== undefined) {
    const id = b.signalId;
    if (typeof id !== "number" || !Number.isSafeInteger(id) || id <= 0) {
      return { ok: false, error: "signalId must be a positive whole number." };
    }
    return { ok: true, value: { kind: "signal", signalId: id, side: b.side } };
  }

  const symbol = normalizeSymbol(b.symbol);
  if (!symbol) return { ok: false, error: "Enter a valid NSE symbol, e.g. RELIANCE or M&M." };

  let price: number | null = null;
  if (!blank(b.price)) {
    const p = b.price;
    if (typeof p !== "number" || !Number.isFinite(p) || p <= 0 || p >= MAX_PRICE) {
      return { ok: false, error: "Price must be a number above 0 and below ₹10 crore." };
    }
    price = Math.round(p * 100) / 100;
  }

  let tradeDate = today;
  if (!blank(b.tradeDate)) {
    if (typeof b.tradeDate !== "string" || !isISODate(b.tradeDate)) {
      return { ok: false, error: "The date must be a YYYY-MM-DD date." };
    }
    if (b.tradeDate > today) return { ok: false, error: "The date can't be in the future." };
    tradeDate = b.tradeDate;
  }
  return { ok: true, value: { kind: "manual", symbol, side: b.side, price, tradeDate } };
}

/** The Trades page's `side` and `symbol` query parameters; symbol is an uppercase prefix. */
export function parseTradeFilter(p: SearchParamsLike): TradeFilter {
  const get = (k: string) => (p instanceof URLSearchParams ? (p.get(k) ?? undefined) : first(p[k]));
  const side = get("side");
  const symbol = (get("symbol") ?? "").toUpperCase().replace(/[^A-Z0-9&-]/g, "");
  return { side: isSide(side) ? side : undefined, symbol: symbol || undefined };
}

export function tradeMarksFrom(rows: { id: number; signalId: number | null; side: Side }[]): TradeMarks {
  const marks: TradeMarks = {};
  for (const r of rows) {
    if (r.signalId === null) continue;
    (marks[r.signalId] ??= {})[r.side] = r.id;
  }
  return marks;
}

export function chunk<T>(items: T[], size: number): T[][] {
  const out: T[][] = [];
  for (let i = 0; i < items.length; i += size) out.push(items.slice(i, i + size));
  return out;
}

/** "IchimokuKinkoHyo (Future BUY)" → { family: "IchimokuKinkoHyo", name: "Future BUY" }. */
export function signalSource(signal: string | null): { family: string; name: string } | null {
  if (!signal) return null;
  const family = signal.split(" ")[0];
  return { family, name: shortSignalName(signal, family) };
}

/** A trade recorded from a signal takes the signal's stock, price, day and name. */
export function tradeFromSignal(signal: Signal, side: Side): NewTrade {
  return {
    tradeDate: signal.tradeDate,
    symbol: signal.symbol,
    side,
    price: signal.price,
    signalId: signal.id,
    signal: signal.signal,
  };
}

export type SymbolLookup =
  | { kind: "known"; symbol: string }
  | { kind: "company"; matches: [string, string][] } // the text is a company name, e.g. "infosys"
  | { kind: "unknown"; symbol: string } // symbol-shaped but not in NSE's list, e.g. a new listing
  | { kind: "invalid" };

/**
 * What the add form's Stock field holds, checked against the bundled NSE list
 * ([symbol, company] pairs), so a company name isn't saved as a made-up symbol.
 */
export function lookupSymbol(input: string, list: readonly (readonly string[])[]): SymbolLookup {
  const symbol = normalizeSymbol(input);
  if (symbol && list.some(([s]) => s === symbol)) return { kind: "known", symbol };
  const typed = input.trim().toLowerCase();
  if (typed.length >= 3) {
    const matches = list.filter(([, name]) => name.toLowerCase().startsWith(typed)).map(([s, name]): [string, string] => [s, name]);
    if (matches.length > 0) return { kind: "company", matches };
  }
  return symbol ? { kind: "unknown", symbol } : { kind: "invalid" };
}

/**
 * The body the add form posts. An untouched date is left out so the server picks
 * today (IST): a page left open overnight must not record yesterday.
 */
export function manualTradeBody(f: { symbol: string; side: Side; price: number | null; date: string; dateTouched: boolean }) {
  return { symbol: f.symbol, side: f.side, price: f.price, ...(f.dateTouched ? { tradeDate: f.date } : {}) };
}

export function manualTrade(input: Extract<TradeInput, { kind: "manual" }>): NewTrade {
  return {
    tradeDate: input.tradeDate,
    symbol: input.symbol,
    side: input.side,
    price: input.price,
    signalId: null,
    signal: null,
  };
}
