import { NextRequest } from "next/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { addDays, todayIST } from "@/lib/time";
import type { Signal, Trade } from "@/lib/types";

const store = vi.hoisted(() => ({ getSignal: vi.fn(), addTrade: vi.fn() }));
vi.mock("@/lib/store", () => ({ getStore: () => store }));

import { POST } from "./route";

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
const trade: Trade = {
  id: 1,
  createdAt: "2026-09-28T05:00:00Z",
  tradeDate: "2026-09-28",
  symbol: "SCHAEFFLER",
  side: "buy",
  price: 4217.6,
  signalId: 9,
  signal: "IchimokuKinkoHyo (Future BUY)",
};

function post(body: unknown, origin: string | null = "http://localhost:3000") {
  const headers: Record<string, string> = { host: "localhost:3000", "content-type": "application/json" };
  if (origin) headers.origin = origin;
  const payload = typeof body === "string" ? body : JSON.stringify(body);
  return POST(new NextRequest("http://localhost:3000/api/trades", { method: "POST", headers, body: payload }));
}

beforeEach(() => {
  store.getSignal.mockReset();
  store.addTrade.mockReset();
});
afterEach(() => vi.useRealTimers());

describe("POST /api/trades", () => {
  it("records a trade from a signal with the signal's details: 201", async () => {
    store.getSignal.mockResolvedValue(signal);
    store.addTrade.mockResolvedValue({ trade, created: true });
    const res = await post({ signalId: 9, side: "buy", price: 1, symbol: "HACK" });
    expect(res.status).toBe(201);
    expect(await res.json()).toEqual({ trade });
    expect(store.addTrade).toHaveBeenCalledWith({
      tradeDate: "2026-09-28",
      symbol: "SCHAEFFLER",
      side: "buy",
      price: 4217.6,
      signalId: 9,
      signal: "IchimokuKinkoHyo (Future BUY)",
    });
  });
  it("answers a repeat click with the existing trade: 200", async () => {
    store.getSignal.mockResolvedValue(signal);
    store.addTrade.mockResolvedValue({ trade, created: false });
    const res = await post({ signalId: 9, side: "buy" });
    expect(res.status).toBe(200);
    expect(await res.json()).toEqual({ trade });
  });
  it("says so when the signal no longer exists: 404", async () => {
    store.getSignal.mockResolvedValue(null);
    const res = await post({ signalId: 9, side: "buy" });
    expect(res.status).toBe(404);
    expect((await res.json()).error).toContain("no longer exists");
    expect(store.addTrade).not.toHaveBeenCalled();
  });
  it("records a manual trade dated today (IST): 201", async () => {
    store.addTrade.mockResolvedValue({ trade, created: true });
    const res = await post({ symbol: "m&m", side: "sell" });
    expect(res.status).toBe(201);
    expect(store.addTrade).toHaveBeenCalledWith({
      tradeDate: todayIST(),
      symbol: "M&M",
      side: "sell",
      price: null,
      signalId: null,
      signal: null,
    });
  });
  it("uses the IST day just after midnight, when UTC is still on the previous day", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-09-28T19:00:00Z")); // 00:30 IST on the 29th
    store.addTrade.mockResolvedValue({ trade, created: true });
    expect((await post({ symbol: "TCS", side: "buy", tradeDate: "2026-09-29" })).status).toBe(201);
    expect((await post({ symbol: "TCS", side: "buy", tradeDate: "2026-09-30" })).status).toBe(400);
  });
  it("refuses a future date: 400", async () => {
    const res = await post({ symbol: "TCS", side: "buy", tradeDate: addDays(todayIST(), 1) });
    expect(res.status).toBe(400);
    expect((await res.json()).error).toContain("future");
    expect(store.addTrade).not.toHaveBeenCalled();
  });
  it("refuses a body that isn't JSON: 400", async () => {
    expect((await post("not json")).status).toBe(400);
  });
  it("refuses a cross-site request: 403", async () => {
    const res = await post({ symbol: "TCS", side: "buy" }, "https://evil.example");
    expect(res.status).toBe(403);
    expect(store.addTrade).not.toHaveBeenCalled();
  });
  it("passes a Supabase failure on: 500 with its message", async () => {
    store.addTrade.mockRejectedValue(new Error('Supabase: relation "trades" does not exist'));
    const res = await post({ symbol: "TCS", side: "buy" });
    expect(res.status).toBe(500);
    expect((await res.json()).error).toContain('relation "trades" does not exist');
  });
});
