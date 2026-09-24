import { describe, expect, it } from "vitest";
import { summarize, withCatalog } from "./summary";
import type { Signal } from "./types";

const base: Omit<Signal, "id" | "signal" | "strategy" | "direction"> = {
  firedAt: "2026-09-24T04:00:00Z",
  tradeDate: "2026-09-24",
  token: 1,
  symbol: "TCS",
  price: 1,
  rsiDaily: null,
  rsiWeekly: null,
  rsiMonthly: null,
};

const rows: Signal[] = [
  { ...base, id: 1, strategy: "RSI", signal: "RSI (GFS)", direction: "long" },
  { ...base, id: 2, strategy: "RSI", signal: "RSI (GFS)", direction: "long" },
  { ...base, id: 3, strategy: "RSI", signal: "RSI (Bearish GFS)", direction: "short" },
  { ...base, id: 4, strategy: "IchimokuKinkoHyo", signal: "IchimokuKinkoHyo (SELL)", direction: "short" },
];

describe("summarize", () => {
  it("counts per strategy and signal, busiest signal first", () => {
    expect(summarize(rows)).toEqual([
      {
        strategy: "RSI",
        long: 2,
        short: 1,
        total: 3,
        signals: [
          { strategy: "RSI", signal: "RSI (GFS)", long: 2, short: 0, total: 2 },
          { strategy: "RSI", signal: "RSI (Bearish GFS)", long: 0, short: 1, total: 1 },
        ],
      },
      {
        strategy: "IchimokuKinkoHyo",
        long: 0,
        short: 1,
        total: 1,
        signals: [{ strategy: "IchimokuKinkoHyo", signal: "IchimokuKinkoHyo (SELL)", long: 0, short: 1, total: 1 }],
      },
    ]);
  });
});

describe("withCatalog", () => {
  it("lists every catalog signal in catalog order, including zero counts", () => {
    const model = withCatalog(summarize(rows));
    const rsi = model.find((s) => s.strategy === "RSI")!;
    expect(rsi.signals).toHaveLength(12);
    expect(rsi.signals[0]).toEqual({ strategy: "RSI", signal: "RSI (Weekly Strong Bullish)", long: 0, short: 0, total: 0 });
    expect(rsi.signals.find((s) => s.signal === "RSI (GFS)")!.total).toBe(2);
    expect(rsi.total).toBe(3);
  });
  it("keeps signals and strategies the catalog doesn't know", () => {
    const extra: Signal = { ...base, id: 9, strategy: "NewThing", signal: "NewThing (X)", direction: "long" };
    const model = withCatalog(summarize([...rows, extra]));
    expect(model.map((s) => s.strategy)).toEqual(["RSI", "IchimokuKinkoHyo", "NewThing"]);
    expect(model[2].signals.map((s) => s.signal)).toEqual(["NewThing (X)"]);
  });
});
