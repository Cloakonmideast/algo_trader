import { describe, expect, it, vi } from "vitest";
import { tradeMarksOrEmpty } from "./marks";

describe("tradeMarksOrEmpty", () => {
  it("passes the store's marks through", async () => {
    const store = { tradeMarks: async () => ({ 5: { buy: 1 } }) };
    expect(await tradeMarksOrEmpty(store, [5])).toEqual({ 5: { buy: 1 } });
  });
  it("returns no marks, and logs, when the lookup fails (e.g. 002_trades.sql not run)", async () => {
    const log = vi.spyOn(console, "error").mockImplementation(() => {});
    const store = {
      tradeMarks: async () => {
        throw new Error('Supabase: relation "trades" does not exist');
      },
    };
    expect(await tradeMarksOrEmpty(store, [5])).toEqual({});
    expect(log).toHaveBeenCalled();
    log.mockRestore();
  });
  it("skips the lookup when no signals are shown", async () => {
    const tradeMarks = vi.fn();
    expect(await tradeMarksOrEmpty({ tradeMarks }, [])).toEqual({});
    expect(tradeMarks).not.toHaveBeenCalled();
  });
});
