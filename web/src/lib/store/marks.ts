import type { TradeMarks } from "@/lib/types";
import type { SignalStore } from "./types";

/**
 * Buy/Sell marks for the signals on a page. A failure (for example before
 * 002_trades.sql has been run) must not take the signal pages down, so it is
 * logged and the page shows every row unmarked.
 */
export async function tradeMarksOrEmpty(
  store: Pick<SignalStore, "tradeMarks">,
  signalIds: number[],
): Promise<TradeMarks> {
  if (signalIds.length === 0) return {};
  try {
    return await store.tradeMarks(signalIds);
  } catch (e) {
    console.error("[trades] could not load Buy/Sell marks:", e);
    return {};
  }
}
