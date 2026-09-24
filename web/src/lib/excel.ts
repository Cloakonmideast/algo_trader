import ExcelJS from "exceljs";
import { summarize } from "./summary";
import { istTime } from "./time";
import type { Signal, SignalFilter } from "./types";

const COLUMNS = [
  { header: "Date", key: "date", width: 12 },
  { header: "Time (IST)", key: "time", width: 11 },
  { header: "Symbol", key: "symbol", width: 16 },
  { header: "Strategy", key: "strategy", width: 18 },
  { header: "Signal", key: "signal", width: 38 },
  { header: "Direction", key: "direction", width: 10 },
  { header: "Price", key: "price", width: 11 },
  { header: "RSI Daily", key: "rsiDaily", width: 10 },
  { header: "RSI Weekly", key: "rsiWeekly", width: 11 },
  { header: "RSI Monthly", key: "rsiMonthly", width: 12 },
];

export const EXCEL_HEADERS = COLUMNS.map((c) => c.header);

export function buildWorkbook(rows: Signal[]): ExcelJS.Workbook {
  const wb = new ExcelJS.Workbook();
  wb.created = new Date();

  const ws = wb.addWorksheet("Signals", { views: [{ state: "frozen", ySplit: 1 }] });
  ws.columns = COLUMNS;
  for (const r of rows) {
    ws.addRow({
      date: r.tradeDate,
      time: istTime(r.firedAt),
      symbol: r.symbol,
      strategy: r.strategy,
      signal: r.signal,
      direction: r.direction,
      price: r.price,
      rsiDaily: r.rsiDaily ?? null,
      rsiWeekly: r.rsiWeekly ?? null,
      rsiMonthly: r.rsiMonthly ?? null,
    });
  }
  ws.getRow(1).font = { bold: true };
  ws.autoFilter = { from: { row: 1, column: 1 }, to: { row: 1, column: COLUMNS.length } };
  ws.getColumn("price").numFmt = "#,##0.00";
  for (const key of ["rsiDaily", "rsiWeekly", "rsiMonthly"]) ws.getColumn(key).numFmt = "0.00";

  const sum = wb.addWorksheet("Summary", { views: [{ state: "frozen", ySplit: 1 }] });
  sum.columns = [
    { header: "Strategy", key: "strategy", width: 18 },
    { header: "Signal", key: "signal", width: 38 },
    { header: "Long", key: "long", width: 8 },
    { header: "Short", key: "short", width: 8 },
    { header: "Total", key: "total", width: 8 },
  ];
  let long = 0;
  let short = 0;
  for (const s of summarize(rows)) {
    long += s.long;
    short += s.short;
    for (const g of s.signals) sum.addRow(g);
  }
  sum.addRow({ strategy: "All", long, short, total: rows.length }).font = { bold: true };
  sum.getRow(1).font = { bold: true };

  return wb;
}

export async function workbookBlob(wb: ExcelJS.Workbook): Promise<Blob> {
  const buf = await wb.xlsx.writeBuffer();
  return new Blob([buf], { type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" });
}

export function exportFileName(f: SignalFilter): string {
  const range =
    !f.from && !f.to ? "all" : f.from && f.from === f.to ? f.from : `${f.from ?? "start"}_to_${f.to ?? "today"}`;
  const only =
    f.signals.length === 1 && !f.strategies.length
      ? f.signals[0]
      : f.strategies.length === 1 && !f.signals.length
        ? f.strategies[0]
        : "";
  const slug = only.replace(/[^A-Za-z0-9]+/g, "-").replace(/^-+|-+$/g, "");
  return `signals_${range}${slug ? `_${slug}` : ""}.xlsx`;
}
