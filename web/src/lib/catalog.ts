import type { Direction } from "./types";

// Mirrors the names the Go engine emits (strategy.RSISignalNames and the
// Ichimoku variants in internal/strategy/breakout.go). Signals not listed here
// still display; this list only drives the sidebar and filter checkboxes.
export const STRATEGIES: Record<string, readonly string[]> = {
  RSI: [
    "RSI (Weekly Strong Bullish)",
    "RSI (Weekly Bullish Range-Shift)",
    "RSI (Weekly Strong Bearish)",
    "RSI (Weekly Bearish Range-Shift)",
    "RSI (Daily Strong Bullish)",
    "RSI (Daily Bullish Range-Shift)",
    "RSI (Daily Strong Bearish)",
    "RSI (Daily Bearish Range-Shift)",
    "RSI (Advanced GFS)",
    "RSI (GFS)",
    "RSI (Bearish Advanced GFS)",
    "RSI (Bearish GFS)",
  ],
  IchimokuKinkoHyo: [
    "IchimokuKinkoHyo (BUY)",
    "IchimokuKinkoHyo (BUY) 1A",
    "IchimokuKinkoHyo (BUY) Gold Cross",
    "IchimokuKinkoHyo (BUY) Gold Cross A",
    "IchimokuKinkoHyo (SELL)",
    "IchimokuKinkoHyo (SELL) 1A",
    "IchimokuKinkoHyo (SELL) Death Cross",
    "IchimokuKinkoHyo (SELL) Death Cross A",
  ],
};

export const STRATEGY_NAMES = Object.keys(STRATEGIES);

// Same vocabulary as the engine's backtest.InferDirection.
const SHORT_MARKERS = ["SELL", "BEARISH", "DEATH", "SHORT"];

export function directionOf(signal: string): Direction {
  const upper = signal.toUpperCase();
  return SHORT_MARKERS.some((m) => upper.includes(m)) ? "short" : "long";
}

/** "RSI (Daily Strong Bullish)" → "Daily Strong Bullish"; "IchimokuKinkoHyo (BUY) 1A" → "(BUY) 1A". */
export function shortSignalName(signal: string, strategy: string): string {
  const rest = signal.startsWith(strategy + " ") ? signal.slice(strategy.length + 1) : signal;
  const lone = /^\(([^()]*)\)$/.exec(rest);
  return lone ? lone[1] : rest;
}
