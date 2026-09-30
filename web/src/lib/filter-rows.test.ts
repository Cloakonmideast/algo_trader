import { describe, expect, it } from "vitest";
import { applyFilter, matchesFilter, sortSignals } from "./filter-rows";
import { baseFilter } from "./filters";
import type { Signal } from "./types";

function row(p: Partial<Signal>): Signal {
  return {
    id: 1,
    firedAt: "2026-09-24T04:00:00Z",
    tradeDate: "2026-09-24",
    token: 1,
    symbol: "TCS",
    strategy: "RSI",
    signal: "RSI (GFS)",
    direction: "long",
    price: 100,
    rsiDaily: null,
    rsiWeekly: null,
    rsiMonthly: null,
    ...p,
  };
}

const rows = [
  row({ id: 1, symbol: "TCS", firedAt: "2026-09-24T04:00:00Z" }),
  row({ id: 2, symbol: "INFY", signal: "RSI (Bearish GFS)", direction: "short", firedAt: "2026-09-24T06:00:00Z" }),
  row({
    id: 3,
    symbol: "TATASTEEL",
    strategy: "IchimokuKinkoHyo",
    signal: "IchimokuKinkoHyo (BUY) 1A",
    firedAt: "2026-09-23T05:00:00Z",
    tradeDate: "2026-09-23",
  }),
];

describe("matchesFilter", () => {
  it("bounds by trade date inclusively", () => {
    const f = baseFilter({ from: "2026-09-24", to: "2026-09-24" });
    expect(rows.filter((r) => matchesFilter(r, f)).map((r) => r.id)).toEqual([1, 2]);
  });
  it("matches a selected strategy OR a selected signal", () => {
    const f = baseFilter({ strategies: ["IchimokuKinkoHyo"], signals: ["RSI (Bearish GFS)"] });
    expect(rows.filter((r) => matchesFilter(r, f)).map((r) => r.id)).toEqual([2, 3]);
  });
  it("filters by direction and symbol prefix", () => {
    expect(rows.filter((r) => matchesFilter(r, baseFilter({ direction: "short" }))).map((r) => r.id)).toEqual([2]);
    expect(rows.filter((r) => matchesFilter(r, baseFilter({ symbol: "TA" }))).map((r) => r.id)).toEqual([3]);
  });
});

describe("applyFilter sorting", () => {
  it("defaults to newest first", () => {
    expect(applyFilter(rows, baseFilter()).map((r) => r.id)).toEqual([2, 1, 3]);
  });
  it("sorts by symbol ascending", () => {
    expect(applyFilter(rows, baseFilter({ sort: "symbol", order: "asc" })).map((r) => r.symbol)).toEqual([
      "INFY",
      "TATASTEEL",
      "TCS",
    ]);
  });
  it("sorts by signal descending", () => {
    expect(applyFilter(rows, baseFilter({ sort: "signal", order: "desc" })).map((r) => r.id)).toEqual([1, 2, 3]);
  });
});

describe("sortSignals", () => {
  const ids = (xs: Signal[]) => xs.map((r) => r.id);
  const more = [
    ...rows,
    row({ id: 4, symbol: "WIPRO", signal: "RSI (Bearish GFS)", direction: "short", firedAt: "2026-09-24T07:00:00Z" }),
  ];

  it("sorts by side, Longs first, newest first within a side", () => {
    expect(ids(sortSignals(more, "side", "asc"))).toEqual([1, 3, 4, 2]);
    expect(ids(sortSignals(more, "side", "desc"))).toEqual([4, 2, 1, 3]);
  });
  it("sorts by signal name, newest first within a signal", () => {
    expect(ids(sortSignals(more, "signal", "asc"))).toEqual([3, 4, 2, 1]);
    expect(ids(sortSignals(more, "signal", "desc"))).toEqual([1, 4, 2, 3]);
  });
  it("sorts by time and by symbol", () => {
    expect(ids(sortSignals(more, "time", "desc"))).toEqual([4, 2, 1, 3]);
    expect(ids(sortSignals(more, "symbol", "asc"))).toEqual([2, 3, 1, 4]);
  });
  it("leaves the input array alone", () => {
    const copy = [...more];
    sortSignals(more, "side", "desc");
    expect(more).toEqual(copy);
  });
});
