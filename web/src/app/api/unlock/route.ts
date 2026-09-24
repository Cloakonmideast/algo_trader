import { NextResponse, type NextRequest } from "next/server";
import { UNLOCK_COOKIE, UNLOCK_MAX_AGE, passcodeMatches, safeNext, unlockToken } from "@/lib/auth";

export async function POST(request: NextRequest) {
  const form = await request.formData();
  const passcode = String(form.get("passcode") ?? "");
  const next = safeNext(String(form.get("next") ?? "/"));
  const secret = process.env.COOKIE_SECRET;

  if (!secret || !passcodeMatches(passcode, process.env.SITE_PASSCODE)) {
    await new Promise((r) => setTimeout(r, 750)); // slow down guessing
    const url = new URL("/unlock", request.url);
    url.searchParams.set("error", secret ? "1" : "config");
    url.searchParams.set("next", next);
    return NextResponse.redirect(url, 303);
  }

  const res = NextResponse.redirect(new URL(next, request.url), 303);
  res.cookies.set(UNLOCK_COOKIE, unlockToken(secret), {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax",
    maxAge: UNLOCK_MAX_AGE,
    path: "/",
  });
  return res;
}
