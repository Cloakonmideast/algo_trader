import { NextResponse, type NextRequest } from "next/server";
import { kiteLoginUrl } from "@/lib/kite";

export function GET(request: NextRequest) {
  const apiKey = process.env.KITE_API_KEY;
  if (!apiKey) {
    const url = new URL("/", request.url);
    url.searchParams.set("kite", "error");
    url.searchParams.set("reason", "KITE_API_KEY is not configured.");
    return NextResponse.redirect(url);
  }
  return NextResponse.redirect(kiteLoginUrl(apiKey));
}
