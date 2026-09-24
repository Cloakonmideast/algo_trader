import { describe, expect, it } from "vitest";
import { addDays, ago, engineHealth, isISODate, istDate, istTime, todayIST, tokenValidToday } from "./time";

describe("IST conversion", () => {
  it("shifts UTC into IST", () => {
    expect(istDate("2026-09-24T20:00:00Z")).toBe("2026-09-25");
    expect(istTime("2026-09-24T03:45:00Z")).toBe("09:15:00");
    expect(todayIST(new Date("2026-09-24T18:29:59Z"))).toBe("2026-09-24");
    expect(todayIST(new Date("2026-09-24T18:30:00Z"))).toBe("2026-09-25");
  });
});

describe("addDays / isISODate", () => {
  it("crosses month and year boundaries", () => {
    expect(addDays("2026-02-28", 1)).toBe("2026-03-01");
    expect(addDays("2026-01-01", -1)).toBe("2025-12-31");
  });
  it("accepts only real YYYY-MM-DD dates", () => {
    expect(isISODate("2026-09-24")).toBe(true);
    expect(isISODate("2026-02-30")).toBe(false);
    expect(isISODate("24-09-2026")).toBe(false);
    expect(isISODate("")).toBe(false);
    expect(isISODate(undefined)).toBe(false);
  });
});

describe("tokenValidToday", () => {
  const at1030 = new Date("2026-09-24T05:00:00Z"); // 10:30 IST
  it("accepts a token created after 06:00 IST today", () => {
    expect(tokenValidToday("2026-09-24T01:00:00Z", at1030)).toBe(true); // 06:30 IST
  });
  it("rejects a token created before 06:00 IST today", () => {
    expect(tokenValidToday("2026-09-24T00:29:00Z", at1030)).toBe(false); // 05:59 IST
  });
  it("before 06:00 IST, yesterday's token is still current", () => {
    const at0330 = new Date("2026-09-23T22:00:00Z"); // 03:30 IST on the 24th
    expect(tokenValidToday("2026-09-23T03:00:00Z", at0330)).toBe(true); // 08:30 IST on the 23rd
  });
  it("rejects missing or unparseable values", () => {
    expect(tokenValidToday(null, at1030)).toBe(false);
    expect(tokenValidToday("nonsense", at1030)).toBe(false);
  });
});

describe("engineHealth", () => {
  const now = new Date("2026-09-24T05:00:00Z");
  it("classifies by age", () => {
    expect(engineHealth("2026-09-24T04:58:00Z", now)).toBe("ok");
    expect(engineHealth("2026-09-24T04:55:00Z", now)).toBe("stale");
    expect(engineHealth("2026-09-24T04:40:00Z", now)).toBe("down");
    expect(engineHealth(null, now)).toBe("down");
  });
});

describe("ago", () => {
  const now = new Date("2026-09-24T05:00:00Z");
  it("renders coarse relative times", () => {
    expect(ago("2026-09-24T04:59:30Z", now)).toBe("just now");
    expect(ago("2026-09-24T04:55:00Z", now)).toBe("5 min ago");
    expect(ago("2026-09-24T02:00:00Z", now)).toBe("3 h ago");
    expect(ago("2026-09-22T05:00:00Z", now)).toBe("2 d ago");
  });
});
