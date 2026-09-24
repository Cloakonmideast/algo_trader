import { describe, expect, it, vi } from "vitest";
import { fetchAllSignals } from "./export-fetch";
import { baseFilter } from "./filters";

function pages(total: number, pageSize = 1000) {
  return vi.fn<typeof fetch>(async (input) => {
    const page = Number(new URL(String(input), "http://x").searchParams.get("page"));
    const start = (page - 1) * pageSize;
    const rows = Array.from({ length: Math.max(0, Math.min(pageSize, total - start)) }, (_, i) => ({ id: start + i + 1 }));
    return new Response(JSON.stringify({ rows, total, page, pageSize }), { status: 200 });
  });
}

describe("fetchAllSignals", () => {
  it("walks every page and keeps the filter", async () => {
    const f = pages(2500);
    const rows = await fetchAllSignals(baseFilter({ strategies: ["RSI"] }), f);
    expect(rows).toHaveLength(2500);
    expect(f).toHaveBeenCalledTimes(3);
    expect(String(f.mock.calls[0][0])).toContain("strategy=RSI");
    expect(String(f.mock.calls[0][0])).toContain("pageSize=1000");
  });

  it("refuses more than 100,000 rows before downloading them", async () => {
    const f = pages(150_000);
    await expect(fetchAllSignals(baseFilter(), f)).rejects.toThrow(/150,000/);
    expect(f).toHaveBeenCalledTimes(1);
  });

  it("surfaces the server's error message", async () => {
    const f = vi.fn<typeof fetch>(async () => new Response(JSON.stringify({ error: "Locked." }), { status: 401 }));
    await expect(fetchAllSignals(baseFilter(), f)).rejects.toThrow("Locked.");
  });
});
