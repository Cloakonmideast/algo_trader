import { describe, expect, it } from "vitest";
import { baseFilter, filterToParams, parseFilter } from "./filters";

describe("parseFilter", () => {
  it("reads every field", () => {
    const f = parseFilter(
      new URLSearchParams(
        "from=2026-09-01&to=2026-09-24&strategy=RSI&signal=IchimokuKinkoHyo+(BUY)+1A&direction=short&symbol=+rel&sort=symbol&order=asc",
      ),
    );
    expect(f).toEqual({
      from: "2026-09-01",
      to: "2026-09-24",
      strategies: ["RSI"],
      signals: ["IchimokuKinkoHyo (BUY) 1A"],
      direction: "short",
      symbol: "REL",
      sort: "symbol",
      order: "asc",
    });
  });

  it("falls back to defaults for missing or invalid values", () => {
    const f = parseFilter({ from: "garbage", direction: "sideways", sort: "price" }, { from: "2026-09-18", to: "2026-09-24" });
    expect(f).toEqual(baseFilter({ from: "2026-09-18", to: "2026-09-24" }));
  });

  it("swaps a reversed range", () => {
    const f = parseFilter({ from: "2026-09-24", to: "2026-09-01" });
    expect([f.from, f.to]).toEqual(["2026-09-01", "2026-09-24"]);
  });

  it("strips characters that never appear in NSE symbols", () => {
    expect(parseFilter({ symbol: "m&m%_" }).symbol).toBe("M&M");
  });

  it("accepts repeated keys from a Next.js searchParams record", () => {
    expect(parseFilter({ strategy: ["RSI", "IchimokuKinkoHyo"] }).strategies).toEqual(["RSI", "IchimokuKinkoHyo"]);
  });
});

describe("filterToParams", () => {
  it("round-trips through parseFilter", () => {
    const f = baseFilter({
      from: "2026-09-01",
      to: "2026-09-24",
      strategies: ["RSI"],
      signals: ["IchimokuKinkoHyo (SELL) 1A"],
      direction: "long",
      symbol: "TCS",
      sort: "signal",
      order: "asc",
    });
    expect(parseFilter(filterToParams(f))).toEqual(f);
  });

  it("omits defaults", () => {
    expect(filterToParams(baseFilter()).toString()).toBe("");
  });
});
