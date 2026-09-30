import type { NewTrade, Signal, SignalFilter, Status, Trade, TradeFilter, TradeMarks } from "@/lib/types";

export const EXPORT_CAP = 100_000;
export const ALL_CAP = 5_000;
export const MAX_PAGE_SIZE = 1_000;
/** The Trades tab shows at most this many trades, newest first. */
export const TRADES_CAP = 1_000;
/** Signal ids per trades lookup, so request URLs stay short. */
export const MARKS_BATCH = 200;

export class TooManyRowsError extends Error {
  constructor(
    public readonly total: number,
    public readonly cap: number,
  ) {
    super(`${total.toLocaleString("en-US")} signals match — narrow the range to ${cap.toLocaleString("en-US")} or fewer.`);
    this.name = "TooManyRowsError";
  }
}

export interface SignalStore {
  list(f: SignalFilter, offset: number, limit: number): Promise<{ rows: Signal[]; total: number }>;
  listAll(f: SignalFilter, cap: number): Promise<Signal[]>;
  status(): Promise<Status>;
  saveKiteSession(accessToken: string, userId: string): Promise<void>;

  getSignal(id: number): Promise<Signal | null>;
  /** Inserts a trade. A signal can be bought once and sold once: a repeat returns the existing trade with created=false. */
  addTrade(t: NewTrade): Promise<{ trade: Trade; created: boolean }>;
  deleteTrade(id: number): Promise<void>;
  listTrades(f: TradeFilter, cap: number): Promise<{ rows: Trade[]; total: number }>;
  tradeMarks(signalIds: number[]): Promise<TradeMarks>;
}
