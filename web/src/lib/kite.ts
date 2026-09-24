import { createHash } from "node:crypto";

export const KITE_LOGIN_URL = "https://kite.zerodha.com/connect/login";
export const KITE_SESSION_URL = "https://api.kite.trade/session/token";

export interface KiteEnv {
  apiKey?: string;
  apiSecret?: string;
  userId?: string;
}

export type KiteResult = { ok: true } | { ok: false; reason: string };

export function kiteLoginUrl(apiKey: string): string {
  return `${KITE_LOGIN_URL}?v=3&api_key=${encodeURIComponent(apiKey)}`;
}

export function kiteChecksum(apiKey: string, requestToken: string, apiSecret: string): string {
  return createHash("sha256").update(apiKey + requestToken + apiSecret).digest("hex");
}

interface SessionResponse {
  status?: string;
  message?: string;
  data?: { user_id?: string; access_token?: string };
}

export async function completeKiteLogin(
  input: { requestToken: string | null; status: string | null },
  env: KiteEnv,
  deps: { fetch: typeof fetch; save(accessToken: string, userId: string): Promise<void> },
): Promise<KiteResult> {
  if (!env.apiKey || !env.apiSecret || !env.userId) {
    return { ok: false, reason: "Kite is not configured on the server (KITE_API_KEY, KITE_API_SECRET, KITE_USER_ID)." };
  }
  if (input.status !== "success" || !input.requestToken) {
    return { ok: false, reason: "The Zerodha login was cancelled or failed." };
  }

  let res: Response;
  try {
    res = await deps.fetch(KITE_SESSION_URL, {
      method: "POST",
      headers: { "X-Kite-Version": "3", "Content-Type": "application/x-www-form-urlencoded" },
      body: new URLSearchParams({
        api_key: env.apiKey,
        request_token: input.requestToken,
        checksum: kiteChecksum(env.apiKey, input.requestToken, env.apiSecret),
      }).toString(),
    });
  } catch {
    return { ok: false, reason: "Could not reach Zerodha." };
  }

  const body = (await res.json().catch(() => null)) as SessionResponse | null;
  const session = body?.data;
  if (!res.ok || body?.status !== "success" || !session?.access_token || !session.user_id) {
    return { ok: false, reason: body?.message ?? `Token exchange failed (HTTP ${res.status}).` };
  }
  if (session.user_id.trim().toUpperCase() !== env.userId.trim().toUpperCase()) {
    return { ok: false, reason: `That login was for ${session.user_id}, not the configured account.` };
  }

  try {
    await deps.save(session.access_token, session.user_id);
  } catch (e) {
    return {
      ok: false,
      reason: `Logged in, but the token could not be saved: ${e instanceof Error ? e.message : String(e)}`,
    };
  }
  return { ok: true };
}
