import { describe, expect, it } from "vitest";
import type { FilterableQuery } from "./query";
import { applyTradeFilter, toTrade } from "./trade-query";

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

describe("applyTradeFilter", () => {
  it("filters by side and symbol prefix, newest first", () => {
    expect(applyTradeFilter(new FakeQuery(), { side: "sell", symbol: "M&M" }).calls).toEqual([
      "eq side sell",
      "like symbol M&M%",
      "order trade_date desc",
      "order id desc",
    ]);
  });
  it("lists everything newest first without a filter", () => {
    expect(applyTradeFilter(new FakeQuery(), {}).calls).toEqual(["order trade_date desc", "order id desc"]);
  });
});

describe("toTrade", () => {
  it("maps snake_case rows and coerces the price", () => {
    expect(
      toTrade({
        id: 3,
        created_at: "2026-09-28T05:00:00+00:00",
        trade_date: "2026-09-28",
        symbol: "TCS",
        side: "buy",
        price: "3521.40",
        signal_id: 12,
        signal: "RSI (GFS)",
      }),
    ).toEqual({
      id: 3,
      createdAt: "2026-09-28T05:00:00+00:00",
      tradeDate: "2026-09-28",
      symbol: "TCS",
      side: "buy",
      price: 3521.4,
      signalId: 12,
      signal: "RSI (GFS)",
    });
  });
  it("keeps a missing price as null", () => {
    const t = toTrade({
      id: 4,
      created_at: "2026-09-28T05:00:00+00:00",
      trade_date: "2026-09-28",
      symbol: "TCS",
      side: "sell",
      price: null,
      signal_id: null,
      signal: null,
    });
    expect(t.price).toBeNull();
  });
});
