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
