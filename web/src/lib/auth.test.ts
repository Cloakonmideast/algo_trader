import { describe, expect, it } from "vitest";
import { isValidUnlock, passcodeMatches, safeNext, unlockToken } from "./auth";

describe("unlock cookie", () => {
  it("is stable per secret and versioned", () => {
    expect(unlockToken("s1")).toBe(unlockToken("s1"));
    expect(unlockToken("s1")).toMatch(/^v1\./);
    expect(unlockToken("s1")).not.toBe(unlockToken("s2"));
  });
  it("validates only the token for the current secret", () => {
    expect(isValidUnlock(unlockToken("s1"), "s1")).toBe(true);
    expect(isValidUnlock(unlockToken("s1"), "s2")).toBe(false);
    expect(isValidUnlock(unlockToken("s1") + "x", "s1")).toBe(false);
    expect(isValidUnlock(undefined, "s1")).toBe(false);
    expect(isValidUnlock(unlockToken("s1"), undefined)).toBe(false);
  });
});

describe("passcodeMatches", () => {
  it("compares exactly", () => {
    expect(passcodeMatches("open sesame", "open sesame")).toBe(true);
    expect(passcodeMatches("open sesamE", "open sesame")).toBe(false);
  });
  it("never matches when no passcode is configured", () => {
    expect(passcodeMatches("", undefined)).toBe(false);
    expect(passcodeMatches("", "")).toBe(false);
  });
});

describe("safeNext", () => {
  it("keeps same-site paths", () => {
    expect(safeNext("/history?from=2026-09-01")).toBe("/history?from=2026-09-01");
  });
  it("rejects anything that could leave the site", () => {
    expect(safeNext("//evil.com")).toBe("/");
    expect(safeNext("/\\evil.com")).toBe("/");
    expect(safeNext("https://evil.com")).toBe("/");
    expect(safeNext(null)).toBe("/");
  });
});
