import { describe, expect, it } from "vitest";
import { STRATEGIES, STRATEGY_NAMES, directionOf, shortSignalName } from "./catalog";

describe("directionOf", () => {
  it.each([
    ["RSI (Daily Strong Bullish)", "long"],
    ["RSI (GFS)", "long"],
    ["RSI (Bearish Advanced GFS)", "short"],
    ["RSI (Weekly Strong Bearish)", "short"],
    ["IchimokuKinkoHyo (SELL) 1A", "short"],
    ["IchimokuKinkoHyo (SELL) Death Cross", "short"],
    ["IchimokuKinkoHyo (BUY) Gold Cross", "long"],
  ])("%s → %s", (signal, want) => {
    expect(directionOf(signal)).toBe(want);
  });
});

describe("shortSignalName", () => {
  it("drops the family prefix and a lone pair of parentheses", () => {
    expect(shortSignalName("RSI (Daily Strong Bullish)", "RSI")).toBe("Daily Strong Bullish");
  });
  it("keeps parentheses when text follows them", () => {
    expect(shortSignalName("IchimokuKinkoHyo (BUY) 1A", "IchimokuKinkoHyo")).toBe("(BUY) 1A");
  });
  it("returns unknown names unchanged", () => {
    expect(shortSignalName("Something else", "RSI")).toBe("Something else");
  });
});

describe("catalog", () => {
  it("lists the 12 RSI and 8 Ichimoku signals", () => {
    expect(STRATEGIES.RSI).toHaveLength(12);
    expect(STRATEGIES.IchimokuKinkoHyo).toHaveLength(8);
    expect(STRATEGY_NAMES).toEqual(["RSI", "IchimokuKinkoHyo"]);
  });
});
