import { describe, expect, it } from "vitest";
import { baseFilter } from "@/lib/filters";
import { applyFilterToQuery, pgQuote, selectionFilter, toSignal, type FilterableQuery } from "./query";

class FakeQuery implements FilterableQuery {
  calls: string[] = [];
  gte(c: string, v: string) {
    this.calls.push(`gte ${c} ${v}`);
    return this;
  }
  lte(c: string, v: string) {
    this.calls.push(`lte ${c} ${v}`);
    return this;
  }
  eq(c: string, v: string) {
    this.calls.push(`eq ${c} ${v}`);
    return this;
  }
  or(s: string) {
    this.calls.push(`or ${s}`);
    return this;
  }
  like(c: string, p: string) {
    this.calls.push(`like ${c} ${p}`);
    return this;
  }
  order(c: string, o: { ascending: boolean }) {
    this.calls.push(`order ${c} ${o.ascending ? "asc" : "desc"}`);
    return this;
  }
}

describe("pgQuote / selectionFilter", () => {
  it("quotes values containing spaces and parentheses", () => {
    expect(pgQuote("RSI (GFS)")).toBe('"RSI (GFS)"');
    expect(pgQuote('a"b\\c')).toBe('"a\\"b\\\\c"');
  });
  it("ORs strategies and signals", () => {
    expect(selectionFilter(baseFilter({ strategies: ["RSI"], signals: ["IchimokuKinkoHyo (BUY) 1A"] }))).toBe(
      'strategy.in.("RSI"),signal.in.("IchimokuKinkoHyo (BUY) 1A")',
    );
    expect(selectionFilter(baseFilter())).toBeNull();
  });
});

describe("applyFilterToQuery", () => {
  it("maps every filter field", () => {
    const q = applyFilterToQuery(
      new FakeQuery(),
      baseFilter({
        from: "2026-09-01",
        to: "2026-09-24",
        strategies: ["RSI"],
        direction: "long",
        symbol: "TC",
        sort: "symbol",
        order: "asc",
      }),
    );
    expect(q.calls).toEqual([
      "gte trade_date 2026-09-01",
      "lte trade_date 2026-09-24",
      'or strategy.in.("RSI")',
      "eq direction long",
      "like symbol TC%",
      "order symbol asc",
      "order fired_at desc",
      "order id desc",
    ]);
  });
  it("sorts by time with an id tie-break in the same direction", () => {
    expect(applyFilterToQuery(new FakeQuery(), baseFilter()).calls).toEqual(["order fired_at desc", "order id desc"]);
    expect(applyFilterToQuery(new FakeQuery(), baseFilter({ order: "asc" })).calls).toEqual([
      "order fired_at asc",
      "order id asc",
    ]);
  });
});

describe("toSignal", () => {
  it("maps snake_case rows and coerces numerics", () => {
    expect(
      toSignal({
        id: 7,
        fired_at: "2026-09-24T04:00:00+00:00",
        trade_date: "2026-09-24",
        token: 408065,
        symbol: "INFY",
        strategy: "RSI",
        signal: "RSI (GFS)",
        direction: "long",
        price: "1523.40",
        rsi_daily: 41.2,
        rsi_weekly: null,
        rsi_monthly: "66.10",
      }),
    ).toEqual({
      id: 7,
      firedAt: "2026-09-24T04:00:00+00:00",
      tradeDate: "2026-09-24",
      token: 408065,
      symbol: "INFY",
      strategy: "RSI",
      signal: "RSI (GFS)",
      direction: "long",
      price: 1523.4,
      rsiDaily: 41.2,
      rsiWeekly: null,
      rsiMonthly: 66.1,
    });
  });
});
