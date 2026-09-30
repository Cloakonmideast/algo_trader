import { NextRequest } from "next/server";
import { beforeEach, describe, expect, it, vi } from "vitest";

const store = vi.hoisted(() => ({ deleteTrade: vi.fn() }));
vi.mock("@/lib/store", () => ({ getStore: () => store }));

import { DELETE } from "./route";

function del(id: string, origin = "http://localhost:3000") {
  const req = new NextRequest(`http://localhost:3000/api/trades/${id}`, {
    method: "DELETE",
    headers: { host: "localhost:3000", origin },
  });
  return DELETE(req, { params: Promise.resolve({ id }) });
}

// A block body: a function returned from beforeEach runs as a teardown, and mockReset() returns the mock itself.
beforeEach(() => {
  store.deleteTrade.mockReset();
});

describe("DELETE /api/trades/[id]", () => {
  it("deletes the trade: 204", async () => {
    store.deleteTrade.mockResolvedValue(undefined);
    const res = await del("7");
    expect(res.status).toBe(204);
    expect(store.deleteTrade).toHaveBeenCalledWith(7);
  });
  it.each(["abc", "0", "1.5", "-2", "99999999999999999999"])("refuses the id %s: 400", async (id) => {
    expect((await del(id)).status).toBe(400);
    expect(store.deleteTrade).not.toHaveBeenCalled();
  });
  it("refuses a cross-site request: 403", async () => {
    expect((await del("7", "https://evil.example")).status).toBe(403);
    expect(store.deleteTrade).not.toHaveBeenCalled();
  });
  it("passes a Supabase failure on: 500", async () => {
    store.deleteTrade.mockRejectedValue(new Error("Supabase: down"));
    const res = await del("7");
    expect(res.status).toBe(500);
    expect((await res.json()).error).toBe("Supabase: down");
  });
});
