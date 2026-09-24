import ExcelJS from "exceljs";
import { describe, expect, it } from "vitest";
import { EXCEL_HEADERS, buildWorkbook, exportFileName } from "./excel";
import { baseFilter } from "./filters";
import type { Signal } from "./types";

const rows: Signal[] = [
  {
    id: 1,
    firedAt: "2026-09-24T04:05:06Z",
    tradeDate: "2026-09-24",
    token: 1,
    symbol: "INFY",
    strategy: "RSI",
    signal: "RSI (GFS)",
    direction: "long",
    price: 1523.4,
    rsiDaily: 41.25,
    rsiWeekly: 63,
    rsiMonthly: 66.1,
  },
  {
    id: 2,
    firedAt: "2026-09-24T05:00:00Z",
    tradeDate: "2026-09-24",
    token: 2,
    symbol: "TCS",
    strategy: "IchimokuKinkoHyo",
    signal: "IchimokuKinkoHyo (SELL) 1A",
    direction: "short",
    price: 3999,
    rsiDaily: null,
    rsiWeekly: null,
    rsiMonthly: null,
  },
];

async function roundTrip(wb: ExcelJS.Workbook) {
  const loaded = new ExcelJS.Workbook();
  await loaded.xlsx.load(await wb.xlsx.writeBuffer());
  return loaded;
}

describe("buildWorkbook", () => {
  it("writes a Signals sheet with the agreed columns", async () => {
    const wb = await roundTrip(buildWorkbook(rows));
    const ws = wb.getWorksheet("Signals")!;
    expect((ws.getRow(1).values as unknown[]).slice(1)).toEqual(EXCEL_HEADERS);
    expect((ws.getRow(2).values as unknown[]).slice(1)).toEqual([
      "2026-09-24",
      "09:35:06",
      "INFY",
      "RSI",
      "RSI (GFS)",
      "long",
      1523.4,
      41.25,
      63,
      66.1,
    ]);
    expect(ws.getRow(3).getCell(8).value).toBeNull(); // no RSI for Ichimoku
    expect(ws.rowCount).toBe(3);
  });

  it("writes a Summary sheet with per-signal counts and a total", async () => {
    const wb = await roundTrip(buildWorkbook(rows));
    const ws = wb.getWorksheet("Summary")!;
    const values = [2, 3, 4].map((r) => (ws.getRow(r).values as unknown[]).slice(1));
    expect(values).toEqual([
      ["RSI", "RSI (GFS)", 1, 0, 1],
      ["IchimokuKinkoHyo", "IchimokuKinkoHyo (SELL) 1A", 0, 1, 1],
      ["All", undefined, 1, 1, 2],
    ]);
  });
});

describe("exportFileName", () => {
  it("names a single day", () => {
    expect(exportFileName(baseFilter({ from: "2026-09-24", to: "2026-09-24" }))).toBe("signals_2026-09-24.xlsx");
  });
  it("names a range with a single selected strategy or signal", () => {
    expect(exportFileName(baseFilter({ from: "2026-09-01", to: "2026-09-24", strategies: ["RSI"] }))).toBe(
      "signals_2026-09-01_to_2026-09-24_RSI.xlsx",
    );
    expect(
      exportFileName(baseFilter({ from: "2026-09-24", to: "2026-09-24", signals: ["IchimokuKinkoHyo (BUY) 1A"] })),
    ).toBe("signals_2026-09-24_IchimokuKinkoHyo-BUY-1A.xlsx");
  });
  it("handles open ranges and multi-selections", () => {
    expect(exportFileName(baseFilter())).toBe("signals_all.xlsx");
    expect(exportFileName(baseFilter({ to: "2026-09-24", strategies: ["RSI", "IchimokuKinkoHyo"] }))).toBe(
      "signals_start_to_2026-09-24.xlsx",
    );
  });
});
