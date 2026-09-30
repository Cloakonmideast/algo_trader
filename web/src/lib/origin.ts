/**
 * False when the browser reports that a request came from another site. Browsers
 * always send Origin on fetch POST/DELETE; a request without one is left to the
 * unlock-cookie check (SameSite=Lax), which another site can't pass either.
 */
export function isSameOrigin(origin: string | null, host: string | null): boolean {
  if (origin === null) return true;
  if (!host) return false;
  try {
    return new URL(origin).host === host.toLowerCase();
  } catch {
    return false; // e.g. the opaque origin "null"
  }
}
