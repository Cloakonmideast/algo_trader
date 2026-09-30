import { describe, expect, it } from "vitest";
import { nextSort } from "./table-sort";

const firstOrder = (k: string) => (k === "time" ? "desc" : "asc");

describe("nextSort", () => {
  it("starts a new column in its natural order: newest first for time, A→Z otherwise", () => {
    expect(nextSort({ key: "time", order: "desc" }, "signal", firstOrder)).toEqual({ key: "signal", order: "asc" });
    expect(nextSort({ key: "signal", order: "asc" }, "time", firstOrder)).toEqual({ key: "time", order: "desc" });
  });
  it("flips the order when the same column is clicked again", () => {
    expect(nextSort({ key: "side", order: "asc" }, "side", firstOrder)).toEqual({ key: "side", order: "desc" });
    expect(nextSort({ key: "side", order: "desc" }, "side", firstOrder)).toEqual({ key: "side", order: "asc" });
  });
});
