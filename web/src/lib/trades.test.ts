import { describe, expect, it } from "vitest";
import symbols from "./nse-symbols.json";
import {
  MAX_PRICE,
  chunk,
  lookupSymbol,
  manualTrade,
  manualTradeBody,
  normalizeSymbol,
  parseTradeFilter,
  parseTradeInput,
  signalSource,
  sortTrades,
  tradeFromSignal,
  tradeMarksFrom,
} from "./trades";
import type { Signal, Trade } from "./types";

const TODAY = "2026-09-28";

describe("normalizeSymbol", () => {
  it.each([
    [" reliance ", "RELIANCE"],
    ["m&m", "M&M"],
    ["BAJAJ-AUTO", "BAJAJ-AUTO"],
    ["20MICRONS", "20MICRONS"],
  ])("%s → %s", (raw, want) => {
    expect(normalizeSymbol(raw)).toBe(want);
  });
  it.each(["", "   ", "RELIANCE INDUSTRIES", "TCS;DROP", "A".repeat(31), 42, null, undefined])("rejects %j", (raw) => {
    expect(normalizeSymbol(raw)).toBeNull();
  });
});

describe("parseTradeInput", () => {
  it("reads a signal trade and ignores every other field", () => {
    expect(parseTradeInput({ signalId: 17, side: "buy", symbol: "HACK", price: 1 }, TODAY)).toEqual({
      ok: true,
      value: { kind: "signal", signalId: 17, side: "buy" },
    });
  });
  it.each([0, -3, 1.5, "17", null])("rejects signalId %j", (signalId) => {
    expect(parseTradeInput({ signalId, side: "sell" }, TODAY)).toMatchObject({ ok: false });
  });
  it("reads a manual trade, defaulting price to none and date to today", () => {
    expect(parseTradeInput({ symbol: "m&m", side: "sell" }, TODAY)).toEqual({
      ok: true,
      value: { kind: "manual", symbol: "M&M", side: "sell", price: null, tradeDate: TODAY },
    });
  });
  it("rounds the price to paise and keeps a past date", () => {
    expect(parseTradeInput({ symbol: "TCS", side: "buy", price: 3521.456, tradeDate: "2026-09-01" }, TODAY)).toEqual({
      ok: true,
      value: { kind: "manual", symbol: "TCS", side: "buy", price: 3521.46, tradeDate: "2026-09-01" },
    });
  });
  it("treats an empty price or date as not given", () => {
    expect(parseTradeInput({ symbol: "TCS", side: "buy", price: null, tradeDate: "" }, TODAY)).toMatchObject({
      ok: true,
      value: { price: null, tradeDate: TODAY },
    });
  });
  it("accepts today and refuses tomorrow", () => {
    expect(parseTradeInput({ symbol: "TCS", side: "buy", tradeDate: TODAY }, TODAY)).toMatchObject({ ok: true });
    expect(parseTradeInput({ symbol: "TCS", side: "buy", tradeDate: "2026-09-29" }, TODAY)).toMatchObject({ ok: false });
  });
  it.each([
    [{ side: "buy" }, "symbol"],
    [{ symbol: "TCS" }, "side"],
    [{ symbol: "TCS", side: "hold" }, "side"],
    [{ symbol: "TCS", side: "buy", price: 0 }, "Price"],
    [{ symbol: "TCS", side: "buy", price: -5 }, "Price"],
    [{ symbol: "TCS", side: "buy", price: MAX_PRICE }, "Price"],
    [{ symbol: "TCS", side: "buy", price: "100" }, "Price"],
    [{ symbol: "TCS", side: "buy", tradeDate: "2026-09-29" }, "future"],
    [{ symbol: "TCS", side: "buy", tradeDate: "28-09-2026" }, "YYYY-MM-DD"],
    [{ symbol: "TCS", side: "buy", tradeDate: "2026-02-30" }, "YYYY-MM-DD"],
  ])("rejects %j with a message about %s", (body, fragment) => {
    const r = parseTradeInput(body, TODAY);
    expect(r.ok).toBe(false);
    if (!r.ok) expect(r.error).toContain(fragment);
  });
  it.each([null, "text", 42, []])("rejects the non-object body %j", (body) => {
    expect(parseTradeInput(body, TODAY)).toMatchObject({ ok: false });
  });
});

describe("parseTradeFilter", () => {
  it("reads the side and a cleaned symbol prefix", () => {
    expect(parseTradeFilter({ side: "sell", symbol: " m&m " })).toEqual({ side: "sell", symbol: "M&M" });
  });
  it("drops unknown sides and empty symbols", () => {
    expect(parseTradeFilter({ side: "hold", symbol: "" })).toEqual({ side: undefined, symbol: undefined });
    expect(parseTradeFilter(new URLSearchParams("side=buy"))).toEqual({ side: "buy", symbol: undefined });
  });
});

describe("tradeMarksFrom", () => {
  it("maps each signal to its buy and sell trade ids, skipping manual trades", () => {
    expect(
      tradeMarksFrom([
        { id: 1, signalId: 10, side: "buy" },
        { id: 2, signalId: 10, side: "sell" },
        { id: 3, signalId: 11, side: "sell" },
        { id: 4, signalId: null, side: "buy" },
      ]),
    ).toEqual({ 10: { buy: 1, sell: 2 }, 11: { sell: 3 } });
  });
});

describe("chunk", () => {
  it("splits into batches and keeps the remainder", () => {
    expect(chunk([1, 2, 3, 4, 5], 2)).toEqual([[1, 2], [3, 4], [5]]);
    expect(chunk([], 200)).toEqual([]);
  });
});

describe("signalSource", () => {
  it("names the signal a trade came from", () => {
    expect(signalSource("IchimokuKinkoHyo (Future BUY)")).toEqual({ family: "IchimokuKinkoHyo", name: "Future BUY" });
    expect(signalSource("IchimokuKinkoHyo (BUY) 1A")).toEqual({ family: "IchimokuKinkoHyo", name: "(BUY) 1A" });
    expect(signalSource("RSI (Daily Strong Bullish)")).toEqual({ family: "RSI", name: "Daily Strong Bullish" });
    expect(signalSource(null)).toBeNull();
  });
});

describe("tradeFromSignal / manualTrade", () => {
  it("copies the signal's stock, price, day and name", () => {
    const signal: Signal = {
      id: 9,
      firedAt: "2026-09-28T04:00:00Z",
      tradeDate: "2026-09-28",
      token: 258817,
      symbol: "SCHAEFFLER",
      strategy: "IchimokuKinkoHyo",
      signal: "IchimokuKinkoHyo (Future BUY)",
      direction: "long",
      price: 4217.6,
      rsiDaily: 67.4,
      rsiWeekly: 56.3,
      rsiMonthly: 55.8,
    };
    expect(tradeFromSignal(signal, "buy")).toEqual({
      tradeDate: "2026-09-28",
      symbol: "SCHAEFFLER",
      side: "buy",
      price: 4217.6,
      signalId: 9,
      signal: "IchimokuKinkoHyo (Future BUY)",
    });
  });
  it("marks a manual trade as having no signal", () => {
    expect(manualTrade({ kind: "manual", symbol: "M&M", side: "sell", price: null, tradeDate: TODAY })).toEqual({
      tradeDate: TODAY,
      symbol: "M&M",
      side: "sell",
      price: null,
      signalId: null,
      signal: null,
    });
  });
});

describe("lookupSymbol", () => {
  const list = [
    ["INFY", "Infosys Limited"],
    ["M&M", "Mahindra & Mahindra Limited"],
    ["TATACHEM", "Tata Chemicals Limited"],
    ["TATASTEEL", "Tata Steel Limited"],
  ];
  it("recognises a listed symbol in any case", () => {
    expect(lookupSymbol(" infy ", list)).toEqual({ kind: "known", symbol: "INFY" });
    expect(lookupSymbol("m&m", list)).toEqual({ kind: "known", symbol: "M&M" });
  });
  it("treats a company name as a company, not a symbol", () => {
    expect(lookupSymbol("infosys", list)).toEqual({ kind: "company", matches: [["INFY", "Infosys Limited"]] });
    expect(lookupSymbol("Infosys Limited", list)).toEqual({ kind: "company", matches: [["INFY", "Infosys Limited"]] });
    expect(lookupSymbol("tata", list)).toEqual({
      kind: "company",
      matches: [
        ["TATACHEM", "Tata Chemicals Limited"],
        ["TATASTEEL", "Tata Steel Limited"],
      ],
    });
  });
  it("flags a valid-looking symbol that isn't listed (e.g. a new listing)", () => {
    expect(lookupSymbol("newco", list)).toEqual({ kind: "unknown", symbol: "NEWCO" });
    expect(lookupSymbol("ab", list)).toEqual({ kind: "unknown", symbol: "AB" }); // too short to search names
  });
  it("calls anything else invalid", () => {
    expect(lookupSymbol("!!", list)).toEqual({ kind: "invalid" });
    expect(lookupSymbol("", list)).toEqual({ kind: "invalid" });
  });
  it("finds INFY for 'infosys' in the bundled NSE list", () => {
    const r = lookupSymbol("infosys", symbols);
    expect(r.kind).toBe("company");
    if (r.kind === "company") expect(r.matches.map(([s]) => s)).toContain("INFY");
  });
});

describe("manualTradeBody", () => {
  it("leaves an untouched date out, so the server picks today (IST)", () => {
    expect(manualTradeBody({ symbol: "TCS", side: "buy", price: null, date: "2026-09-27", dateTouched: false })).toEqual({
      symbol: "TCS",
      side: "buy",
      price: null,
    });
  });
  it("sends a date the user picked", () => {
    expect(manualTradeBody({ symbol: "TCS", side: "sell", price: 3500, date: "2026-09-01", dateTouched: true })).toEqual({
      symbol: "TCS",
      side: "sell",
      price: 3500,
      tradeDate: "2026-09-01",
    });
  });
});

describe("sortTrades", () => {
  const t = (p: Partial<Trade>): Trade => ({
    id: 1,
    createdAt: "2026-09-28T04:00:00Z",
    tradeDate: "2026-09-28",
    symbol: "TCS",
    side: "buy",
    price: null,
    signalId: null,
    signal: null,
    ...p,
  });
  // Newest first, as the store returns them.
  const trades = [
    t({ id: 5, tradeDate: "2026-09-29", symbol: "INFY", side: "sell", signalId: 9, signal: "RSI (GFS)" }),
    t({ id: 4, tradeDate: "2026-09-29", symbol: "TCS", side: "buy" }),
    t({ id: 3, tradeDate: "2026-09-28", symbol: "M&M", side: "buy", signalId: 7, signal: "IchimokuKinkoHyo (Future BUY)" }),
    t({ id: 2, tradeDate: "2026-09-27", symbol: "WIPRO", side: "sell" }),
  ];
  const ids = (xs: Trade[]) => xs.map((x) => x.id);

  it("sorts by side, buys first, newest first within a side", () => {
    expect(ids(sortTrades(trades, "side", "asc"))).toEqual([4, 3, 5, 2]);
    expect(ids(sortTrades(trades, "side", "desc"))).toEqual([5, 2, 4, 3]);
  });
  it("sorts by the signal a trade came from, with manual trades last either way", () => {
    expect(ids(sortTrades(trades, "from", "asc"))).toEqual([3, 5, 4, 2]);
    expect(ids(sortTrades(trades, "from", "desc"))).toEqual([5, 3, 4, 2]);
  });
  it("sorts by date and by stock", () => {
    expect(ids(sortTrades(trades, "date", "asc"))).toEqual([2, 3, 4, 5]);
    expect(ids(sortTrades(trades, "date", "desc"))).toEqual([5, 4, 3, 2]);
    expect(ids(sortTrades(trades, "stock", "asc"))).toEqual([5, 3, 4, 2]);
  });
  it("leaves the input array alone", () => {
    const copy = [...trades];
    sortTrades(trades, "side", "desc");
    expect(trades).toEqual(copy);
  });
});
