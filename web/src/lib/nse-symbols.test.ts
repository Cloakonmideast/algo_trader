import { describe, expect, it } from "vitest";
import symbols from "./nse-symbols.json";
import { SYMBOL_RE } from "./trades";

describe("nse-symbols.json", () => {
  it("lists NSE equities as [symbol, company], sorted, unique, every symbol valid", () => {
    expect(symbols.length).toBeGreaterThan(2000);
    let prev = "";
    for (const [symbol, name] of symbols) {
      expect(symbol).toMatch(SYMBOL_RE);
      expect(name.length).toBeGreaterThan(0);
      expect(symbol > prev).toBe(true); // sorted and unique
      prev = symbol;
    }
    expect(symbols.find(([s]) => s === "M&M")?.[1]).toMatch(/Mahindra/);
  });
});
