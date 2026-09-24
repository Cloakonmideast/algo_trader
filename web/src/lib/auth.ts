import { createHash, createHmac, timingSafeEqual } from "node:crypto";

export const UNLOCK_COOKIE = "unlock";
export const UNLOCK_MAX_AGE = 60 * 60 * 24 * 365;

/** Cookie value proving the device entered the passcode. Rotating the secret revokes every device. */
export function unlockToken(secret: string): string {
  return "v1." + createHmac("sha256", secret).update("unlock:v1").digest("base64url");
}

// Hash both sides first so timingSafeEqual always compares equal-length buffers.
function safeEqual(a: string, b: string): boolean {
  const ha = createHash("sha256").update(a).digest();
  const hb = createHash("sha256").update(b).digest();
  return timingSafeEqual(ha, hb);
}

export function isValidUnlock(value: string | undefined, secret: string | undefined): boolean {
  if (!value || !secret) return false;
  return safeEqual(value, unlockToken(secret));
}

export function passcodeMatches(input: string, expected: string | undefined): boolean {
  if (!expected) return false;
  return safeEqual(input, expected);
}

/** Only allow redirects to paths on this site. */
export function safeNext(next: string | null | undefined): string {
  if (!next || !next.startsWith("/") || next.startsWith("//") || next.startsWith("/\\")) return "/";
  return next;
}
