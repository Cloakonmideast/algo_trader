import { describe, expect, it } from "vitest";
import { kiteChartUrl } from "./kite-chart";

describe("kiteChartUrl", () => {
  it("opens the stock's NSE chart on Kite by symbol and instrument token", () => {
    expect(kiteChartUrl("INFY", 408065)).toBe("https://kite.zerodha.com/chart/ext/ciq/NSE/INFY/408065");
  });
  it("escapes symbols with characters that mean something in a URL", () => {
    expect(kiteChartUrl("M&M", 519937)).toBe("https://kite.zerodha.com/chart/ext/ciq/NSE/M%26M/519937");
  });
});
