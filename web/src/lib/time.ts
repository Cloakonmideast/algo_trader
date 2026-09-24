const IST_OFFSET_MS = 330 * 60 * 1000;
const DAY_MS = 24 * 60 * 60 * 1000;

function shifted(d: Date | string): Date {
  return new Date(new Date(d).getTime() + IST_OFFSET_MS);
}

/** Calendar date in IST, YYYY-MM-DD. */
export function istDate(d: Date | string): string {
  return shifted(d).toISOString().slice(0, 10);
}

/** Wall-clock time in IST, HH:mm:ss. */
export function istTime(d: Date | string): string {
  return shifted(d).toISOString().slice(11, 19);
}

export function todayIST(now: Date = new Date()): string {
  return istDate(now);
}

export function addDays(date: string, n: number): string {
  const d = new Date(`${date}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() + n);
  return d.toISOString().slice(0, 10);
}

export function isISODate(s: string | undefined): s is string {
  if (!s || !/^\d{4}-\d{2}-\d{2}$/.test(s)) return false;
  const t = Date.parse(`${s}T00:00:00Z`);
  return !Number.isNaN(t) && new Date(t).toISOString().slice(0, 10) === s;
}

/**
 * Zerodha expires access tokens early each morning, so a token counts as
 * current when it was created at or after the most recent 06:00 IST.
 */
export function tokenValidToday(createdAt: string | null, now: Date = new Date()): boolean {
  if (!createdAt) return false;
  const created = Date.parse(createdAt);
  if (Number.isNaN(created)) return false;
  let cutoff = Date.parse(`${todayIST(now)}T06:00:00+05:30`);
  if (now.getTime() < cutoff) cutoff -= DAY_MS;
  return created >= cutoff;
}

export type Health = "ok" | "stale" | "down";

export function engineHealth(lastSeen: string | null, now: Date = new Date()): Health {
  if (!lastSeen) return "down";
  const age = now.getTime() - Date.parse(lastSeen);
  if (Number.isNaN(age)) return "down";
  if (age < 3 * 60 * 1000) return "ok";
  if (age < 10 * 60 * 1000) return "stale";
  return "down";
}

export function ago(iso: string, now: Date = new Date()): string {
  const s = Math.max(0, Math.round((now.getTime() - Date.parse(iso)) / 1000));
  if (s < 60) return "just now";
  if (s < 3600) return `${Math.floor(s / 60)} min ago`;
  if (s < 86400) return `${Math.floor(s / 3600)} h ago`;
  return `${Math.floor(s / 86400)} d ago`;
}
