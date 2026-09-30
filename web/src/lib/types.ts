export type Direction = "long" | "short";

export interface Signal {
  id: number;
  firedAt: string; // ISO timestamp
  tradeDate: string; // YYYY-MM-DD, IST
  token: number;
  symbol: string;
  strategy: string; // family, e.g. "RSI"
  signal: string; // exact variant, e.g. "RSI (Daily Strong Bullish)"
  direction: Direction;
  price: number;
  rsiDaily: number | null;
  rsiWeekly: number | null;
  rsiMonthly: number | null;
}

export type SortKey = "time" | "symbol" | "signal";

export interface SignalFilter {
  from?: string; // inclusive trade_date
  to?: string; // inclusive trade_date
  strategies: string[]; // a row matches if its strategy OR its signal is selected
  signals: string[];
  direction?: Direction;
  symbol?: string; // uppercase prefix
  sort: SortKey;
  order: "asc" | "desc";
}

export interface EngineStatus {
  lastSeen: string;
  instruments: number;
  feedConnected: boolean;
  tokenValid: boolean;
}

export interface Status {
  tokenCreatedAt: string | null;
  tokenValidToday: boolean;
  engine: EngineStatus | null;
}

export type Side = "buy" | "sell";

export interface Trade {
  id: number;
  createdAt: string; // ISO timestamp when it was recorded
  tradeDate: string; // YYYY-MM-DD, IST
  symbol: string;
  side: Side;
  price: number | null;
  signalId: number | null; // the signal it was recorded from; null = manual, or that signal was deleted
  signal: string | null; // that signal's name, e.g. "IchimokuKinkoHyo (Future BUY)"; null = manual
}

export interface TradeFilter {
  side?: Side;
  symbol?: string; // uppercase prefix
}

/** For each signal on screen, the ids of its recorded buy and/or sell trade. */
export type TradeMarks = Record<number, { buy?: number; sell?: number }>;

/** What a POST /api/trades body asks for, once validated. */
export type TradeInput =
  | { kind: "signal"; signalId: number; side: Side }
  | { kind: "manual"; symbol: string; side: Side; price: number | null; tradeDate: string };

/** A row to insert into the trades table. */
export interface NewTrade {
  tradeDate: string;
  symbol: string;
  side: Side;
  price: number | null;
  signalId: number | null;
  signal: string | null;
}
