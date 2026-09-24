import { NextResponse, type NextRequest } from "next/server";
import { UNLOCK_COOKIE, isValidUnlock } from "@/lib/auth";

const PUBLIC_PATHS = new Set(["/unlock", "/api/unlock"]);

export function proxy(request: NextRequest) {
  const { pathname, search } = request.nextUrl;
  if (PUBLIC_PATHS.has(pathname)) return NextResponse.next();

  if (isValidUnlock(request.cookies.get(UNLOCK_COOKIE)?.value, process.env.COOKIE_SECRET)) {
    return NextResponse.next();
  }
  if (pathname.startsWith("/api/")) {
    return NextResponse.json({ error: "Locked. Open the site and enter the passcode." }, { status: 401 });
  }
  const url = new URL("/unlock", request.url);
  url.searchParams.set("next", pathname + search);
  return NextResponse.redirect(url);
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico).*)"],
};
