import { NextResponse, type NextRequest } from "next/server";
import { completeKiteLogin } from "@/lib/kite";
import { getStore } from "@/lib/store";

export async function GET(request: NextRequest) {
  const p = request.nextUrl.searchParams;
  const result = await completeKiteLogin(
    { requestToken: p.get("request_token"), status: p.get("status") },
    { apiKey: process.env.KITE_API_KEY, apiSecret: process.env.KITE_API_SECRET, userId: process.env.KITE_USER_ID },
    { fetch, save: (token, userId) => getStore().saveKiteSession(token, userId) },
  );

  const url = new URL("/", request.url);
  url.searchParams.set("kite", result.ok ? "ok" : "error");
  if (!result.ok) url.searchParams.set("reason", result.reason);
  return NextResponse.redirect(url);
}
