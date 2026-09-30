import { describe, expect, it } from "vitest";
import { isSameOrigin } from "./origin";

describe("isSameOrigin", () => {
  it("accepts the site's own origin", () => {
    expect(isSameOrigin("https://web-one-nu-38.vercel.app", "web-one-nu-38.vercel.app")).toBe(true);
    expect(isSameOrigin("http://localhost:3000", "localhost:3000")).toBe(true);
  });
  it("refuses other sites, other ports and opaque origins", () => {
    expect(isSameOrigin("https://evil.example", "web-one-nu-38.vercel.app")).toBe(false);
    expect(isSameOrigin("http://localhost:4000", "localhost:3000")).toBe(false);
    expect(isSameOrigin("null", "localhost:3000")).toBe(false);
    expect(isSameOrigin("https://evil.example", null)).toBe(false);
  });
  it("leaves a request without an Origin header to the cookie check", () => {
    expect(isSameOrigin(null, "localhost:3000")).toBe(true);
  });
});
