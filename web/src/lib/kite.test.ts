import { createHash } from "node:crypto";
import { describe, expect, it, vi } from "vitest";
import { completeKiteLogin, kiteChecksum, kiteLoginUrl } from "./kite";

const env = { apiKey: "key123", apiSecret: "sec456", userId: "AB1234" };

function kiteResponse(body: unknown, status = 200) {
  return vi.fn<typeof fetch>(
    async () => new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } }),
  );
}

const asFetch = (f: unknown) => f as typeof fetch;

describe("kiteLoginUrl / kiteChecksum", () => {
  it("builds the v3 login URL", () => {
    expect(kiteLoginUrl("key123")).toBe("https://kite.zerodha.com/connect/login?v=3&api_key=key123");
  });
  it("hashes api_key + request_token + api_secret in that order", () => {
    expect(kiteChecksum("abc", "def", "ghi")).toBe(createHash("sha256").update("abcdefghi").digest("hex"));
  });
});

describe("completeKiteLogin", () => {
  it("exchanges the request token and saves the session", async () => {
    const fetch = kiteResponse({ status: "success", data: { user_id: "AB1234", access_token: "tok" } });
    const save = vi.fn<(token: string, userId: string) => Promise<void>>(async () => {});
    const res = await completeKiteLogin({ requestToken: "rt", status: "success" }, env, { fetch: asFetch(fetch), save });

    expect(res).toEqual({ ok: true });
    expect(save).toHaveBeenCalledWith("tok", "AB1234");
    const [url, init] = fetch.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe("https://api.kite.trade/session/token");
    const body = new URLSearchParams(init.body as string);
    expect(body.get("api_key")).toBe("key123");
    expect(body.get("request_token")).toBe("rt");
    expect(body.get("checksum")).toBe(kiteChecksum("key123", "rt", "sec456"));
  });

  it("rejects a login for another Zerodha account", async () => {
    const save = vi.fn<(token: string, userId: string) => Promise<void>>(async () => {});
    const res = await completeKiteLogin({ requestToken: "rt", status: "success" }, env, {
      fetch: asFetch(kiteResponse({ status: "success", data: { user_id: "ZZ9999", access_token: "tok" } })),
      save,
    });
    expect(res.ok).toBe(false);
    expect(save).not.toHaveBeenCalled();
  });

  it("matches the user ID case-insensitively", async () => {
    const res = await completeKiteLogin(
      { requestToken: "rt", status: "success" },
      { ...env, userId: "ab1234" },
      {
        fetch: asFetch(kiteResponse({ status: "success", data: { user_id: "AB1234", access_token: "tok" } })),
        save: async () => {},
      },
    );
    expect(res).toEqual({ ok: true });
  });

  it("reports Zerodha's error message", async () => {
    const res = await completeKiteLogin({ requestToken: "rt", status: "success" }, env, {
      fetch: asFetch(kiteResponse({ status: "error", message: "Token is invalid or has expired." }, 403)),
      save: async () => {},
    });
    expect(res).toEqual({ ok: false, reason: "Token is invalid or has expired." });
  });

  it("does nothing when the login was cancelled", async () => {
    const fetch = vi.fn();
    const res = await completeKiteLogin({ requestToken: null, status: "cancelled" }, env, {
      fetch: asFetch(fetch),
      save: async () => {},
    });
    expect(res.ok).toBe(false);
    expect(fetch).not.toHaveBeenCalled();
  });

  it("fails clearly when Kite isn't configured", async () => {
    const res = await completeKiteLogin({ requestToken: "rt", status: "success" }, {}, {
      fetch: asFetch(vi.fn()),
      save: async () => {},
    });
    expect(res).toEqual({
      ok: false,
      reason: "Kite is not configured on the server (KITE_API_KEY, KITE_API_SECRET, KITE_USER_ID).",
    });
  });

  it("survives network and save failures", async () => {
    const down = await completeKiteLogin({ requestToken: "rt", status: "success" }, env, {
      fetch: asFetch(
        vi.fn(async () => {
          throw new Error("ECONNRESET");
        }),
      ),
      save: async () => {},
    });
    expect(down).toEqual({ ok: false, reason: "Could not reach Zerodha." });

    const saveFails = await completeKiteLogin({ requestToken: "rt", status: "success" }, env, {
      fetch: asFetch(kiteResponse({ status: "success", data: { user_id: "AB1234", access_token: "tok" } })),
      save: async () => {
        throw new Error("db down");
      },
    });
    expect(saveFails).toEqual({ ok: false, reason: "Logged in, but the token could not be saved: db down" });
  });
});
