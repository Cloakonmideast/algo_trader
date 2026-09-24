# Signals Website Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the `web/` Next.js site: dashboard, strategies/day view,
history, Excel export, passcode gate and the Kite login flow. It reads
Supabase, and falls back to built-in demo data when Supabase isn't configured
in `npm run dev`.

**Architecture:** Next.js 16 App Router on Vercel.
- Only server code reads Supabase, through a `SignalStore` interface with two
  implementations: `SupabaseStore` (production) and `DemoStore` (in-memory,
  development only).
- The pure logic (filters, sorting, summaries, IST dates, cookie signing, the
  Kite exchange, workbook building, paged export fetching) lives in
  `src/lib/*.ts` with vitest tests.
- Pages are server components. The only client components are the 30 s
  dashboard refresh, the export button and the nav links.

**Tech Stack:** Next.js 16.3 (App Router, `proxy.ts`), React 19, TypeScript,
Tailwind CSS 4, `@supabase/supabase-js` 2, `exceljs` 4, vitest.

**Spec:** `docs/superpowers/specs/2026-09-24-signals-dashboard-design.md`.
This plan covers the Website section, the `supabase/migrations/001_init.sql`
data model, and the Supabase/Vercel/Kite parts of `docs/SETUP.md`. The engine
changes come in a later plan.

## Global Constraints

- Next.js 16 conventions:
  - Request gating goes in `src/proxy.ts`, exporting `function proxy`. Never
    `middleware.ts`.
  - Page `searchParams` is a `Promise`.
  - `cookies()` is async.
  - Read `web/node_modules/next/dist/docs/` before using any other Next API.
- The Supabase service key is used only in server code. Never import
  `@/lib/store` from a client component.
- The browser must never receive the Kite `access_token`. `/api/status`
  returns only `tokenCreatedAt` and `tokenValidToday`.
- Times are shown in IST. `trade_date` is the IST calendar date.
- A token is valid when it was created at or after the most recent 06:00 IST.
- Engine health is `ok` when last seen under 3 min ago, `stale` under 10 min,
  and `down` otherwise.
- Export columns, in order: Date, Time (IST), Symbol, Strategy, Signal,
  Direction, Price, RSI Daily, RSI Weekly, RSI Monthly.
- Export cap: 100,000 rows. `pageSize` max: 1,000. `all=1` cap: 5,000.
- Env vars: `SUPABASE_URL`, `SUPABASE_SERVICE_KEY`, `SITE_PASSCODE`,
  `COOKIE_SECRET`, `KITE_API_KEY`, `KITE_API_SECRET`, `KITE_USER_ID`.
- All commands run from `web/` unless stated otherwise.
- Commit message trailer: `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## File Map

```
supabase/migrations/001_init.sql      tables + RLS
web/
  vitest.config.ts, .env.example, README.md
  src/proxy.ts                        passcode gate
  src/lib/types.ts                    Signal, SignalFilter, Status
  src/lib/catalog.ts                  known strategies/signals, directionOf, shortSignalName
  src/lib/time.ts                     IST dates, token validity, engine health, "ago"
  src/lib/auth.ts                     unlock cookie + passcode checks
  src/lib/filters.ts                  parse/serialize SignalFilter from URL params
  src/lib/filter-rows.ts              in-memory filter + sort (demo store, strategies page)
  src/lib/summary.ts                  counts per strategy/signal + sidebar model
  src/lib/kite.ts                     Kite login URL, checksum, token exchange
  src/lib/excel.ts                    workbook builder + file name
  src/lib/export-fetch.ts             paged fetch of all rows for export
  src/lib/store/types.ts              SignalStore interface, caps, TooManyRowsError
  src/lib/store/query.ts              PostgREST filter mapping (pure)
  src/lib/store/supabase.ts           SupabaseStore (server-only)
  src/lib/store/demo.ts               DemoStore + demo data
  src/lib/store/index.ts              getStore() / isDemo()
  src/app/layout.tsx, globals.css
  src/app/unlock/page.tsx
  src/app/(site)/layout.tsx           header + nav, force-dynamic
  src/app/(site)/page.tsx             dashboard
  src/app/(site)/strategies/page.tsx
  src/app/(site)/history/page.tsx
  src/app/api/unlock/route.ts
  src/app/api/signals/route.ts
  src/app/api/status/route.ts
  src/app/api/kite/login/route.ts
  src/app/api/kite/callback/route.ts
  src/components/*.tsx
docs/SETUP.md                         Supabase, Vercel, Kite console, GitHub
```

---

### Task 1: Scaffold the Next.js app

**Files:**
- Create: `web/` (via create-next-app), `web/vitest.config.ts`, `web/.env.example`
- Modify: `web/package.json` (scripts), `web/src/app/page.tsx`, `web/src/app/layout.tsx`, `web/src/app/globals.css`

**Interfaces:**
- Produces: a building Next app; `npm test` runs vitest over `src/**/*.test.ts`; `@/` → `web/src/`.

- [ ] **Step 1: Generate the app** (from the repo root)

```bash
npx --yes create-next-app@16.3.6 web --ts --tailwind --eslint --app --src-dir --import-alias "@/*" --use-npm --disable-git --yes
cd web
npm install @supabase/supabase-js@2 exceljs@4 server-only
npm install -D vitest
rm -f public/*.svg
```

- [ ] **Step 2: Add `web/vitest.config.ts`**

```ts
import { fileURLToPath } from "node:url";
import { defineConfig } from "vitest/config";

export default defineConfig({
  resolve: {
    alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) },
  },
  test: {
    environment: "node",
    include: ["src/**/*.test.ts"],
  },
});
```

Add to `web/package.json` scripts: `"test": "vitest run"`.

- [ ] **Step 3: Replace the starter files**

`web/src/app/globals.css`:

```css
@import "tailwindcss";

:root {
  color-scheme: light dark;
}

body {
  @apply bg-slate-50 text-slate-900 dark:bg-slate-950 dark:text-slate-100 antialiased;
  font-family: ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, sans-serif;
}
```

`web/src/app/layout.tsx`:

```tsx
import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "AlgoTrader Signals",
  description: "Personal signal dashboard",
  robots: { index: false, follow: false },
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
```

`web/src/app/page.tsx` (temporary; replaced in Task 9):

```tsx
export default function Page() {
  return <main className="p-6">AlgoTrader Signals</main>;
}
```

- [ ] **Step 4: Add `web/.env.example`**

```bash
# Supabase → Project Settings → API. Leave both empty to use demo data in `npm run dev`.
SUPABASE_URL=
SUPABASE_SERVICE_KEY=

# The passcode you type once per device, and a long random secret that signs the unlock cookie.
# Changing COOKIE_SECRET locks every device out.
SITE_PASSCODE=
COOKIE_SECRET=

# Kite Connect app (developers.kite.trade) and your Zerodha client ID, e.g. AB1234.
KITE_API_KEY=
KITE_API_SECRET=
KITE_USER_ID=
```

- [ ] **Step 5: Verify**

Run: `npm run lint && npm run build && npx vitest run --passWithNoTests`
Expected: lint clean; build succeeds; vitest reports no test files and exits 0.

- [ ] **Step 6: Commit**

```bash
git add web
git commit -m "feat(web): scaffold Next.js 16 app with vitest"
```

---

### Task 2: Database schema, shared types and signal catalog

**Files:**
- Create: `supabase/migrations/001_init.sql`, `web/src/lib/types.ts`, `web/src/lib/catalog.ts`
- Test: `web/src/lib/catalog.test.ts`

**Interfaces:**
- Produces:
  - `types.ts`: `Direction`, `Signal`, `SortKey`, `SignalFilter`, `EngineStatus`, `Status`
  - `catalog.ts`: `STRATEGIES: Record<string, readonly string[]>`,
    `STRATEGY_NAMES: string[]`, `directionOf(signal: string): Direction`,
    `shortSignalName(signal: string, strategy: string): string`

- [ ] **Step 1: Write `supabase/migrations/001_init.sql`**

```sql
-- Run once in the Supabase SQL editor.
create table if not exists signals (
  id          bigint generated always as identity primary key,
  fired_at    timestamptz   not null,
  trade_date  date          not null,          -- IST calendar date of fired_at
  token       bigint        not null,
  symbol      text          not null,
  strategy    text          not null,          -- family: 'RSI', 'IchimokuKinkoHyo'
  signal      text          not null,          -- exact variant
  direction   text          not null check (direction in ('long', 'short')),
  price       numeric(12,2) not null,
  rsi_daily   numeric(5,2),
  rsi_weekly  numeric(5,2),
  rsi_monthly numeric(5,2),
  created_at  timestamptz   not null default now(),
  unique (signal, token, trade_date)
);
create index if not exists signals_date_idx     on signals (trade_date desc, fired_at desc);
create index if not exists signals_strategy_idx on signals (strategy, trade_date);
create index if not exists signals_signal_idx   on signals (signal, trade_date);
create index if not exists signals_symbol_idx   on signals (symbol, trade_date);

create table if not exists kite_session (
  id           smallint    primary key default 1 check (id = 1),
  access_token text        not null,
  user_id      text        not null,
  created_at   timestamptz not null default now()
);

create table if not exists engine_status (
  id             smallint    primary key default 1 check (id = 1),
  last_seen      timestamptz not null,
  instruments    integer     not null,
  feed_connected boolean     not null,
  token_valid    boolean     not null
);

-- RLS on with no policies: the anon key can do nothing. Only the service key
-- (engine + website server) can read or write.
alter table signals       enable row level security;
alter table kite_session  enable row level security;
alter table engine_status enable row level security;
```

- [ ] **Step 2: Write `web/src/lib/types.ts`**

```ts
export type Direction = "long" | "short";

export interface Signal {
  id: number;
  firedAt: string; // ISO timestamp
  tradeDate: string; // YYYY-MM-DD, IST
  token: number;
  symbol: string;
  strategy: string; // family, e.g. "RSI"
  signal: string; // exact variant, e.g. "RSI (Daily Strong Bullish)"
  direction: Direction;
  price: number;
  rsiDaily: number | null;
  rsiWeekly: number | null;
  rsiMonthly: number | null;
}

export type SortKey = "time" | "symbol" | "signal";

export interface SignalFilter {
  from?: string; // inclusive trade_date
  to?: string; // inclusive trade_date
  strategies: string[]; // a row matches if its strategy OR its signal is selected
  signals: string[];
  direction?: Direction;
  symbol?: string; // uppercase prefix
  sort: SortKey;
  order: "asc" | "desc";
}

export interface EngineStatus {
  lastSeen: string;
  instruments: number;
  feedConnected: boolean;
  tokenValid: boolean;
}

export interface Status {
  tokenCreatedAt: string | null;
  tokenValidToday: boolean;
  engine: EngineStatus | null;
}
```

- [ ] **Step 3: Write the failing test `web/src/lib/catalog.test.ts`**

```ts
import { describe, expect, it } from "vitest";
import { STRATEGIES, STRATEGY_NAMES, directionOf, shortSignalName } from "./catalog";

describe("directionOf", () => {
  it.each([
    ["RSI (Daily Strong Bullish)", "long"],
    ["RSI (GFS)", "long"],
    ["RSI (Bearish Advanced GFS)", "short"],
    ["RSI (Weekly Strong Bearish)", "short"],
    ["IchimokuKinkoHyo (SELL) 1A", "short"],
    ["IchimokuKinkoHyo (SELL) Death Cross", "short"],
    ["IchimokuKinkoHyo (BUY) Gold Cross", "long"],
  ])("%s → %s", (signal, want) => {
    expect(directionOf(signal)).toBe(want);
  });
});

describe("shortSignalName", () => {
  it("drops the family prefix and a lone pair of parentheses", () => {
    expect(shortSignalName("RSI (Daily Strong Bullish)", "RSI")).toBe("Daily Strong Bullish");
  });
  it("keeps parentheses when text follows them", () => {
    expect(shortSignalName("IchimokuKinkoHyo (BUY) 1A", "IchimokuKinkoHyo")).toBe("(BUY) 1A");
  });
  it("returns unknown names unchanged", () => {
    expect(shortSignalName("Something else", "RSI")).toBe("Something else");
  });
});

describe("catalog", () => {
  it("lists the 12 RSI and 8 Ichimoku signals", () => {
    expect(STRATEGIES.RSI).toHaveLength(12);
    expect(STRATEGIES.IchimokuKinkoHyo).toHaveLength(8);
    expect(STRATEGY_NAMES).toEqual(["RSI", "IchimokuKinkoHyo"]);
  });
});
```

- [ ] **Step 4: Run it — expect FAIL** (`Cannot find module './catalog'`)

Run: `npx vitest run src/lib/catalog.test.ts`

- [ ] **Step 5: Write `web/src/lib/catalog.ts`**

```ts
import type { Direction } from "./types";

// Mirrors the names the Go engine emits (strategy.RSISignalNames and the
// Ichimoku variants in internal/strategy/breakout.go). Signals not listed here
// still display; this list only drives the sidebar and filter checkboxes.
export const STRATEGIES: Record<string, readonly string[]> = {
  RSI: [
    "RSI (Weekly Strong Bullish)",
    "RSI (Weekly Bullish Range-Shift)",
    "RSI (Weekly Strong Bearish)",
    "RSI (Weekly Bearish Range-Shift)",
    "RSI (Daily Strong Bullish)",
    "RSI (Daily Bullish Range-Shift)",
    "RSI (Daily Strong Bearish)",
    "RSI (Daily Bearish Range-Shift)",
    "RSI (Advanced GFS)",
    "RSI (GFS)",
    "RSI (Bearish Advanced GFS)",
    "RSI (Bearish GFS)",
  ],
  IchimokuKinkoHyo: [
    "IchimokuKinkoHyo (BUY)",
    "IchimokuKinkoHyo (BUY) 1A",
    "IchimokuKinkoHyo (BUY) Gold Cross",
    "IchimokuKinkoHyo (BUY) Gold Cross A",
    "IchimokuKinkoHyo (SELL)",
    "IchimokuKinkoHyo (SELL) 1A",
    "IchimokuKinkoHyo (SELL) Death Cross",
    "IchimokuKinkoHyo (SELL) Death Cross A",
  ],
};

export const STRATEGY_NAMES = Object.keys(STRATEGIES);

// Same vocabulary as the engine's backtest.InferDirection.
const SHORT_MARKERS = ["SELL", "BEARISH", "DEATH", "SHORT"];

export function directionOf(signal: string): Direction {
  const upper = signal.toUpperCase();
  return SHORT_MARKERS.some((m) => upper.includes(m)) ? "short" : "long";
}

/** "RSI (Daily Strong Bullish)" → "Daily Strong Bullish"; "IchimokuKinkoHyo (BUY) 1A" → "(BUY) 1A". */
export function shortSignalName(signal: string, strategy: string): string {
  const rest = signal.startsWith(strategy + " ") ? signal.slice(strategy.length + 1) : signal;
  const lone = /^\(([^()]*)\)$/.exec(rest);
  return lone ? lone[1] : rest;
}
```

- [ ] **Step 6: Run it — expect PASS**

Run: `npx vitest run src/lib/catalog.test.ts`

- [ ] **Step 7: Commit**

```bash
git add supabase web/src/lib/types.ts web/src/lib/catalog.ts web/src/lib/catalog.test.ts
git commit -m "feat(web): signal schema, shared types and signal catalog"
```

---

### Task 3: IST time helpers

**Files:**
- Create: `web/src/lib/time.ts`
- Test: `web/src/lib/time.test.ts`

**Interfaces:**
- Produces:
  - `istDate(d: Date | string): string` (YYYY-MM-DD)
  - `istTime(d: Date | string): string` (HH:mm:ss)
  - `todayIST(now?: Date): string`
  - `addDays(date: string, n: number): string`
  - `isISODate(s: string | undefined): s is string`
  - `tokenValidToday(createdAt: string | null, now?: Date): boolean`
  - `engineHealth(lastSeen: string | null, now?: Date): "ok" | "stale" | "down"`
  - `ago(iso: string, now?: Date): string`

- [ ] **Step 1: Write the failing test `web/src/lib/time.test.ts`**

```ts
import { describe, expect, it } from "vitest";
import { addDays, ago, engineHealth, isISODate, istDate, istTime, todayIST, tokenValidToday } from "./time";

describe("IST conversion", () => {
  it("shifts UTC into IST", () => {
    expect(istDate("2026-09-24T20:00:00Z")).toBe("2026-09-25");
    expect(istTime("2026-09-24T03:45:00Z")).toBe("09:15:00");
    expect(todayIST(new Date("2026-09-24T18:29:59Z"))).toBe("2026-09-24");
    expect(todayIST(new Date("2026-09-24T18:30:00Z"))).toBe("2026-09-25");
  });
});

describe("addDays / isISODate", () => {
  it("crosses month and year boundaries", () => {
    expect(addDays("2026-02-28", 1)).toBe("2026-03-01");
    expect(addDays("2026-01-01", -1)).toBe("2025-12-31");
  });
  it("accepts only real YYYY-MM-DD dates", () => {
    expect(isISODate("2026-09-24")).toBe(true);
    expect(isISODate("2026-02-30")).toBe(false);
    expect(isISODate("24-09-2026")).toBe(false);
    expect(isISODate("")).toBe(false);
    expect(isISODate(undefined)).toBe(false);
  });
});

describe("tokenValidToday", () => {
  const at1030 = new Date("2026-09-24T05:00:00Z"); // 10:30 IST
  it("accepts a token created after 06:00 IST today", () => {
    expect(tokenValidToday("2026-09-24T01:00:00Z", at1030)).toBe(true); // 06:30 IST
  });
  it("rejects a token created before 06:00 IST today", () => {
    expect(tokenValidToday("2026-09-24T00:29:00Z", at1030)).toBe(false); // 05:59 IST
  });
  it("before 06:00 IST, yesterday's token is still current", () => {
    const at0330 = new Date("2026-09-23T22:00:00Z"); // 03:30 IST on the 24th
    expect(tokenValidToday("2026-09-23T03:00:00Z", at0330)).toBe(true); // 08:30 IST on the 23rd
  });
  it("rejects missing or unparseable values", () => {
    expect(tokenValidToday(null, at1030)).toBe(false);
    expect(tokenValidToday("nonsense", at1030)).toBe(false);
  });
});

describe("engineHealth", () => {
  const now = new Date("2026-09-24T05:00:00Z");
  it("classifies by age", () => {
    expect(engineHealth("2026-09-24T04:58:00Z", now)).toBe("ok");
    expect(engineHealth("2026-09-24T04:55:00Z", now)).toBe("stale");
    expect(engineHealth("2026-09-24T04:40:00Z", now)).toBe("down");
    expect(engineHealth(null, now)).toBe("down");
  });
});

describe("ago", () => {
  const now = new Date("2026-09-24T05:00:00Z");
  it("renders coarse relative times", () => {
    expect(ago("2026-09-24T04:59:30Z", now)).toBe("just now");
    expect(ago("2026-09-24T04:55:00Z", now)).toBe("5 min ago");
    expect(ago("2026-09-24T02:00:00Z", now)).toBe("3 h ago");
    expect(ago("2026-09-22T05:00:00Z", now)).toBe("2 d ago");
  });
});
```

- [ ] **Step 2: Run — expect FAIL** (module not found)

Run: `npx vitest run src/lib/time.test.ts`

- [ ] **Step 3: Write `web/src/lib/time.ts`**

```ts
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
```

- [ ] **Step 4: Run — expect PASS**

Run: `npx vitest run src/lib/time.test.ts`

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/time.ts web/src/lib/time.test.ts
git commit -m "feat(web): IST date, token validity and engine health helpers"
```

---

### Task 4: Passcode gate

**Files:**
- Create: `web/src/lib/auth.ts`, `web/src/proxy.ts`, `web/src/app/api/unlock/route.ts`, `web/src/app/unlock/page.tsx`
- Test: `web/src/lib/auth.test.ts`

**Interfaces:**
- Produces:
  - `UNLOCK_COOKIE = "unlock"`
  - `UNLOCK_MAX_AGE` (seconds, 1 year)
  - `unlockToken(secret: string): string`
  - `isValidUnlock(value: string | undefined, secret: string | undefined): boolean`
  - `passcodeMatches(input: string, expected: string | undefined): boolean`
  - `safeNext(next: string | null | undefined): string`

- [ ] **Step 1: Write the failing test `web/src/lib/auth.test.ts`**

```ts
import { describe, expect, it } from "vitest";
import { isValidUnlock, passcodeMatches, safeNext, unlockToken } from "./auth";

describe("unlock cookie", () => {
  it("is stable per secret and versioned", () => {
    expect(unlockToken("s1")).toBe(unlockToken("s1"));
    expect(unlockToken("s1")).toMatch(/^v1\./);
    expect(unlockToken("s1")).not.toBe(unlockToken("s2"));
  });
  it("validates only the token for the current secret", () => {
    expect(isValidUnlock(unlockToken("s1"), "s1")).toBe(true);
    expect(isValidUnlock(unlockToken("s1"), "s2")).toBe(false);
    expect(isValidUnlock(unlockToken("s1") + "x", "s1")).toBe(false);
    expect(isValidUnlock(undefined, "s1")).toBe(false);
    expect(isValidUnlock(unlockToken("s1"), undefined)).toBe(false);
  });
});

describe("passcodeMatches", () => {
  it("compares exactly", () => {
    expect(passcodeMatches("open sesame", "open sesame")).toBe(true);
    expect(passcodeMatches("open sesamE", "open sesame")).toBe(false);
  });
  it("never matches when no passcode is configured", () => {
    expect(passcodeMatches("", undefined)).toBe(false);
    expect(passcodeMatches("", "")).toBe(false);
  });
});

describe("safeNext", () => {
  it("keeps same-site paths", () => {
    expect(safeNext("/history?from=2026-09-01")).toBe("/history?from=2026-09-01");
  });
  it("rejects anything that could leave the site", () => {
    expect(safeNext("//evil.com")).toBe("/");
    expect(safeNext("/\\evil.com")).toBe("/");
    expect(safeNext("https://evil.com")).toBe("/");
    expect(safeNext(null)).toBe("/");
  });
});
```

- [ ] **Step 2: Run — expect FAIL**

Run: `npx vitest run src/lib/auth.test.ts`

- [ ] **Step 3: Write `web/src/lib/auth.ts`**

```ts
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
```

- [ ] **Step 4: Run — expect PASS**

Run: `npx vitest run src/lib/auth.test.ts`

- [ ] **Step 5: Write `web/src/proxy.ts`**

```ts
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
```

- [ ] **Step 6: Write `web/src/app/api/unlock/route.ts`**

```ts
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
```

- [ ] **Step 7: Write `web/src/app/unlock/page.tsx`**

```tsx
import { safeNext } from "@/lib/auth";

type SP = Promise<Record<string, string | string[] | undefined>>;
const first = (v: string | string[] | undefined) => (Array.isArray(v) ? v[0] : v);

export default async function UnlockPage({ searchParams }: { searchParams: SP }) {
  const sp = await searchParams;
  const next = safeNext(first(sp.next));
  const error = first(sp.error);

  return (
    <main className="grid min-h-dvh place-items-center p-4">
      <form
        method="post"
        action="/api/unlock"
        className="w-full max-w-sm space-y-4 rounded-xl border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900"
      >
        <h1 className="text-lg font-semibold">AlgoTrader Signals</h1>
        <p className="text-sm text-slate-600 dark:text-slate-400">
          Enter the passcode to unlock this device. It stays unlocked for a year.
        </p>
        <input type="hidden" name="next" value={next} />
        <input
          type="password"
          name="passcode"
          required
          autoFocus
          autoComplete="current-password"
          placeholder="Passcode"
          className="w-full rounded-md border border-slate-300 bg-transparent px-3 py-2 dark:border-slate-700"
        />
        {error === "1" && <p className="text-sm text-red-600">Wrong passcode.</p>}
        {error === "config" && (
          <p className="text-sm text-red-600">The site has no COOKIE_SECRET configured.</p>
        )}
        <button className="w-full rounded-md bg-slate-900 px-3 py-2 font-medium text-white dark:bg-slate-100 dark:text-slate-900">
          Unlock
        </button>
      </form>
    </main>
  );
}
```

- [ ] **Step 8: Verify the gate manually**

Create `web/.env.local` with `SITE_PASSCODE=test` and `COOKIE_SECRET=dev-secret-change-me`, then run `npm run dev`. Then:
- `curl -s -o /dev/null -w "%{http_code} %{redirect_url}\n" http://localhost:3000/` → `307 …/unlock?next=%2F`
- `curl -s -w "%{http_code}\n" http://localhost:3000/api/status` → `401`
- `curl -s -o /dev/null -w "%{http_code}\n" -c jar -d passcode=test -d next=/ http://localhost:3000/api/unlock` → `303`
- `curl -s -o /dev/null -w "%{http_code}\n" -b jar http://localhost:3000/` → `200`

- [ ] **Step 9: Commit**

```bash
git add web/src/lib/auth.ts web/src/lib/auth.test.ts web/src/proxy.ts web/src/app/api/unlock web/src/app/unlock
git commit -m "feat(web): per-device passcode gate"
```

---

### Task 5: Filters, in-memory filtering and summaries

**Files:**
- Create: `web/src/lib/filters.ts`, `web/src/lib/filter-rows.ts`, `web/src/lib/summary.ts`
- Test: `web/src/lib/filters.test.ts`, `web/src/lib/filter-rows.test.ts`, `web/src/lib/summary.test.ts`

**Interfaces:**
- Consumes: `SignalFilter`, `Signal` (Task 2); `isISODate` (Task 3); `STRATEGIES`, `STRATEGY_NAMES` (Task 2).
- Produces:
  - `filters.ts`:
    - `type SearchParamsLike = URLSearchParams | Record<string, string | string[] | undefined>`
    - `first(v: string | string[] | undefined): string | undefined`
    - `baseFilter(overrides?: Partial<SignalFilter>): SignalFilter`
    - `parseFilter(p: SearchParamsLike, defaults?: Partial<SignalFilter>): SignalFilter`
    - `filterToParams(f: SignalFilter): URLSearchParams`
  - `filter-rows.ts`:
    - `matchesFilter(row: Signal, f: SignalFilter): boolean`
    - `applyFilter(rows: Signal[], f: SignalFilter): Signal[]` (filtered and sorted)
  - `summary.ts`:
    - `interface SignalCount { signal; strategy; long; short; total }`
    - `interface StrategyCount { strategy; long; short; total; signals: SignalCount[] }`
    - `summarize(rows: Signal[]): StrategyCount[]`
    - `withCatalog(counts: StrategyCount[]): StrategyCount[]`

- [ ] **Step 1: Write the failing tests**

`web/src/lib/filters.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { baseFilter, filterToParams, parseFilter } from "./filters";

describe("parseFilter", () => {
  it("reads every field", () => {
    const f = parseFilter(
      new URLSearchParams(
        "from=2026-09-01&to=2026-09-24&strategy=RSI&signal=IchimokuKinkoHyo+(BUY)+1A&direction=short&symbol=+rel&sort=symbol&order=asc",
      ),
    );
    expect(f).toEqual({
      from: "2026-09-01",
      to: "2026-09-24",
      strategies: ["RSI"],
      signals: ["IchimokuKinkoHyo (BUY) 1A"],
      direction: "short",
      symbol: "REL",
      sort: "symbol",
      order: "asc",
    });
  });

  it("falls back to defaults for missing or invalid values", () => {
    const f = parseFilter({ from: "garbage", direction: "sideways", sort: "price" }, { from: "2026-09-18", to: "2026-09-24" });
    expect(f).toEqual(baseFilter({ from: "2026-09-18", to: "2026-09-24" }));
  });

  it("swaps a reversed range", () => {
    const f = parseFilter({ from: "2026-09-24", to: "2026-09-01" });
    expect([f.from, f.to]).toEqual(["2026-09-01", "2026-09-24"]);
  });

  it("strips characters that never appear in NSE symbols", () => {
    expect(parseFilter({ symbol: "m&m%_" }).symbol).toBe("M&M");
  });

  it("accepts repeated keys from a Next.js searchParams record", () => {
    expect(parseFilter({ strategy: ["RSI", "IchimokuKinkoHyo"] }).strategies).toEqual(["RSI", "IchimokuKinkoHyo"]);
  });
});

describe("filterToParams", () => {
  it("round-trips through parseFilter", () => {
    const f = baseFilter({
      from: "2026-09-01",
      to: "2026-09-24",
      strategies: ["RSI"],
      signals: ["IchimokuKinkoHyo (SELL) 1A"],
      direction: "long",
      symbol: "TCS",
      sort: "signal",
      order: "asc",
    });
    expect(parseFilter(filterToParams(f))).toEqual(f);
  });

  it("omits defaults", () => {
    expect(filterToParams(baseFilter()).toString()).toBe("");
  });
});
```

`web/src/lib/filter-rows.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { applyFilter, matchesFilter } from "./filter-rows";
import { baseFilter } from "./filters";
import type { Signal } from "./types";

function row(p: Partial<Signal>): Signal {
  return {
    id: 1,
    firedAt: "2026-09-24T04:00:00Z",
    tradeDate: "2026-09-24",
    token: 1,
    symbol: "TCS",
    strategy: "RSI",
    signal: "RSI (GFS)",
    direction: "long",
    price: 100,
    rsiDaily: null,
    rsiWeekly: null,
    rsiMonthly: null,
    ...p,
  };
}

const rows = [
  row({ id: 1, symbol: "TCS", firedAt: "2026-09-24T04:00:00Z" }),
  row({ id: 2, symbol: "INFY", signal: "RSI (Bearish GFS)", direction: "short", firedAt: "2026-09-24T06:00:00Z" }),
  row({ id: 3, symbol: "TATASTEEL", strategy: "IchimokuKinkoHyo", signal: "IchimokuKinkoHyo (BUY) 1A", firedAt: "2026-09-23T05:00:00Z", tradeDate: "2026-09-23" }),
];

describe("matchesFilter", () => {
  it("bounds by trade date inclusively", () => {
    const f = baseFilter({ from: "2026-09-24", to: "2026-09-24" });
    expect(rows.filter((r) => matchesFilter(r, f)).map((r) => r.id)).toEqual([1, 2]);
  });
  it("matches a selected strategy OR a selected signal", () => {
    const f = baseFilter({ strategies: ["IchimokuKinkoHyo"], signals: ["RSI (Bearish GFS)"] });
    expect(rows.filter((r) => matchesFilter(r, f)).map((r) => r.id)).toEqual([2, 3]);
  });
  it("filters by direction and symbol prefix", () => {
    expect(rows.filter((r) => matchesFilter(r, baseFilter({ direction: "short" }))).map((r) => r.id)).toEqual([2]);
    expect(rows.filter((r) => matchesFilter(r, baseFilter({ symbol: "TA" }))).map((r) => r.id)).toEqual([3]);
  });
});

describe("applyFilter sorting", () => {
  it("defaults to newest first", () => {
    expect(applyFilter(rows, baseFilter()).map((r) => r.id)).toEqual([2, 1, 3]);
  });
  it("sorts by symbol ascending", () => {
    expect(applyFilter(rows, baseFilter({ sort: "symbol", order: "asc" })).map((r) => r.symbol)).toEqual(["INFY", "TATASTEEL", "TCS"]);
  });
  it("sorts by signal descending", () => {
    expect(applyFilter(rows, baseFilter({ sort: "signal", order: "desc" })).map((r) => r.id)).toEqual([1, 2, 3]);
  });
});
```

`web/src/lib/summary.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { summarize, withCatalog } from "./summary";
import type { Signal } from "./types";

const base: Omit<Signal, "id" | "signal" | "strategy" | "direction"> = {
  firedAt: "2026-09-24T04:00:00Z",
  tradeDate: "2026-09-24",
  token: 1,
  symbol: "TCS",
  price: 1,
  rsiDaily: null,
  rsiWeekly: null,
  rsiMonthly: null,
};

const rows: Signal[] = [
  { ...base, id: 1, strategy: "RSI", signal: "RSI (GFS)", direction: "long" },
  { ...base, id: 2, strategy: "RSI", signal: "RSI (GFS)", direction: "long" },
  { ...base, id: 3, strategy: "RSI", signal: "RSI (Bearish GFS)", direction: "short" },
  { ...base, id: 4, strategy: "IchimokuKinkoHyo", signal: "IchimokuKinkoHyo (SELL)", direction: "short" },
];

describe("summarize", () => {
  it("counts per strategy and signal, busiest signal first", () => {
    expect(summarize(rows)).toEqual([
      {
        strategy: "RSI",
        long: 2,
        short: 1,
        total: 3,
        signals: [
          { strategy: "RSI", signal: "RSI (GFS)", long: 2, short: 0, total: 2 },
          { strategy: "RSI", signal: "RSI (Bearish GFS)", long: 0, short: 1, total: 1 },
        ],
      },
      {
        strategy: "IchimokuKinkoHyo",
        long: 0,
        short: 1,
        total: 1,
        signals: [{ strategy: "IchimokuKinkoHyo", signal: "IchimokuKinkoHyo (SELL)", long: 0, short: 1, total: 1 }],
      },
    ]);
  });
});

describe("withCatalog", () => {
  it("lists every catalog signal in catalog order, including zero counts", () => {
    const model = withCatalog(summarize(rows));
    const rsi = model.find((s) => s.strategy === "RSI")!;
    expect(rsi.signals).toHaveLength(12);
    expect(rsi.signals[0]).toEqual({ strategy: "RSI", signal: "RSI (Weekly Strong Bullish)", long: 0, short: 0, total: 0 });
    expect(rsi.signals.find((s) => s.signal === "RSI (GFS)")!.total).toBe(2);
    expect(rsi.total).toBe(3);
  });
  it("keeps signals and strategies the catalog doesn't know", () => {
    const extra: Signal = { ...base, id: 9, strategy: "NewThing", signal: "NewThing (X)", direction: "long" };
    const model = withCatalog(summarize([...rows, extra]));
    expect(model.map((s) => s.strategy)).toEqual(["RSI", "IchimokuKinkoHyo", "NewThing"]);
    expect(model[2].signals.map((s) => s.signal)).toEqual(["NewThing (X)"]);
  });
});
```

- [ ] **Step 2: Run — expect FAIL**

Run: `npx vitest run src/lib/filters.test.ts src/lib/filter-rows.test.ts src/lib/summary.test.ts`

- [ ] **Step 3: Write `web/src/lib/filters.ts`**

```ts
import { isISODate } from "./time";
import type { SignalFilter } from "./types";

export type SearchParamsLike = URLSearchParams | Record<string, string | string[] | undefined>;

export function first(v: string | string[] | undefined): string | undefined {
  return Array.isArray(v) ? v[0] : v;
}

function all(p: SearchParamsLike, key: string): string[] {
  if (p instanceof URLSearchParams) return p.getAll(key);
  const v = p[key];
  return v === undefined ? [] : Array.isArray(v) ? v : [v];
}

export function baseFilter(overrides: Partial<SignalFilter> = {}): SignalFilter {
  return { strategies: [], signals: [], sort: "time", order: "desc", ...overrides };
}

export function parseFilter(p: SearchParamsLike, defaults: Partial<SignalFilter> = {}): SignalFilter {
  const get = (k: string) => all(p, k)[0];
  let from = isISODate(get("from")) ? get("from") : defaults.from;
  let to = isISODate(get("to")) ? get("to") : defaults.to;
  if (from && to && from > to) [from, to] = [to, from];

  const direction = get("direction");
  const sort = get("sort");
  const symbol = (get("symbol") ?? "").toUpperCase().replace(/[^A-Z0-9&-]/g, "");

  return {
    ...baseFilter(),
    from,
    to,
    strategies: all(p, "strategy").filter(Boolean),
    signals: all(p, "signal").filter(Boolean),
    direction: direction === "long" || direction === "short" ? direction : undefined,
    symbol: symbol || undefined,
    sort: sort === "symbol" || sort === "signal" ? sort : "time",
    order: get("order") === "asc" ? "asc" : "desc",
  };
}

export function filterToParams(f: SignalFilter): URLSearchParams {
  const p = new URLSearchParams();
  if (f.from) p.set("from", f.from);
  if (f.to) p.set("to", f.to);
  for (const s of f.strategies) p.append("strategy", s);
  for (const s of f.signals) p.append("signal", s);
  if (f.direction) p.set("direction", f.direction);
  if (f.symbol) p.set("symbol", f.symbol);
  if (f.sort !== "time") p.set("sort", f.sort);
  if (f.order !== "desc") p.set("order", f.order);
  return p;
}
```

Note: `parseFilter` returns `from: undefined` / `to: undefined` / `direction:
undefined` / `symbol: undefined` keys, and `toEqual` treats undefined-valued
keys as equal to missing ones, so the tests above hold.

- [ ] **Step 4: Write `web/src/lib/filter-rows.ts`**

```ts
import type { Signal, SignalFilter } from "./types";

export function matchesFilter(row: Signal, f: SignalFilter): boolean {
  if (f.from && row.tradeDate < f.from) return false;
  if (f.to && row.tradeDate > f.to) return false;
  if ((f.strategies.length || f.signals.length) && !f.strategies.includes(row.strategy) && !f.signals.includes(row.signal)) {
    return false;
  }
  if (f.direction && row.direction !== f.direction) return false;
  if (f.symbol && !row.symbol.startsWith(f.symbol)) return false;
  return true;
}

const newestFirst = (a: Signal, b: Signal) => b.firedAt.localeCompare(a.firedAt) || b.id - a.id;

function comparator(f: SignalFilter): (a: Signal, b: Signal) => number {
  const dir = f.order === "asc" ? 1 : -1;
  switch (f.sort) {
    case "symbol":
      return (a, b) => dir * a.symbol.localeCompare(b.symbol) || newestFirst(a, b);
    case "signal":
      return (a, b) => dir * a.signal.localeCompare(b.signal) || newestFirst(a, b);
    default:
      return (a, b) => dir * (a.firedAt.localeCompare(b.firedAt) || a.id - b.id);
  }
}

export function applyFilter(rows: Signal[], f: SignalFilter): Signal[] {
  return rows.filter((r) => matchesFilter(r, f)).sort(comparator(f));
}
```

- [ ] **Step 5: Write `web/src/lib/summary.ts`**

```ts
import { STRATEGIES, STRATEGY_NAMES } from "./catalog";
import type { Signal } from "./types";

export interface SignalCount {
  strategy: string;
  signal: string;
  long: number;
  short: number;
  total: number;
}

export interface StrategyCount {
  strategy: string;
  long: number;
  short: number;
  total: number;
  signals: SignalCount[];
}

const strategyRank = (s: string) => {
  const i = STRATEGY_NAMES.indexOf(s);
  return i === -1 ? Number.MAX_SAFE_INTEGER : i;
};

const byStrategy = (a: { strategy: string }, b: { strategy: string }) =>
  strategyRank(a.strategy) - strategyRank(b.strategy) || a.strategy.localeCompare(b.strategy);

export function summarize(rows: Signal[]): StrategyCount[] {
  const strategies = new Map<string, StrategyCount>();
  const signals = new Map<string, SignalCount>();

  for (const r of rows) {
    let s = strategies.get(r.strategy);
    if (!s) strategies.set(r.strategy, (s = { strategy: r.strategy, long: 0, short: 0, total: 0, signals: [] }));
    let g = signals.get(r.signal);
    if (!g) {
      g = { strategy: r.strategy, signal: r.signal, long: 0, short: 0, total: 0 };
      signals.set(r.signal, g);
      s.signals.push(g);
    }
    s[r.direction]++;
    s.total++;
    g[r.direction]++;
    g.total++;
  }

  const out = [...strategies.values()].sort(byStrategy);
  for (const s of out) s.signals.sort((a, b) => b.total - a.total || a.signal.localeCompare(b.signal));
  return out;
}

/** Sidebar model: every catalog signal in catalog order (zero counts included), then anything unknown. */
export function withCatalog(counts: StrategyCount[]): StrategyCount[] {
  const seen = new Map(counts.map((c) => [c.strategy, c]));
  const names = [...STRATEGY_NAMES, ...counts.map((c) => c.strategy).filter((s) => !STRATEGIES[s])];

  return names.map((strategy) => {
    const c = seen.get(strategy);
    const bySignal = new Map((c?.signals ?? []).map((g) => [g.signal, g]));
    const known = (STRATEGIES[strategy] ?? []).map(
      (signal) => bySignal.get(signal) ?? { strategy, signal, long: 0, short: 0, total: 0 },
    );
    const extra = (c?.signals ?? []).filter((g) => !(STRATEGIES[strategy] ?? []).includes(g.signal));
    return {
      strategy,
      long: c?.long ?? 0,
      short: c?.short ?? 0,
      total: c?.total ?? 0,
      signals: [...known, ...extra],
    };
  });
}
```

- [ ] **Step 6: Run — expect PASS**

Run: `npx vitest run src/lib/filters.test.ts src/lib/filter-rows.test.ts src/lib/summary.test.ts`

- [ ] **Step 7: Commit**

```bash
git add web/src/lib/filters.ts web/src/lib/filter-rows.ts web/src/lib/summary.ts web/src/lib/*.test.ts
git commit -m "feat(web): signal filters, in-memory filtering and summaries"
```

---

### Task 6: Data stores and JSON API

**Files:**
- Create: `web/src/lib/store/types.ts`, `web/src/lib/store/query.ts`, `web/src/lib/store/supabase.ts`, `web/src/lib/store/demo.ts`, `web/src/lib/store/index.ts`, `web/src/app/api/signals/route.ts`, `web/src/app/api/status/route.ts`
- Test: `web/src/lib/store/query.test.ts`, `web/src/lib/store/demo.test.ts`

**Interfaces:**
- Consumes: Task 2–5 exports.
- Produces:
  - `store/types.ts`:
    ```ts
    interface SignalStore {
      list(f: SignalFilter, offset: number, limit: number): Promise<{ rows: Signal[]; total: number }>;
      listAll(f: SignalFilter, cap: number): Promise<Signal[]>; // throws TooManyRowsError
      status(): Promise<Status>;
      saveKiteSession(accessToken: string, userId: string): Promise<void>;
    }
    ```
    Also `EXPORT_CAP = 100_000`, `ALL_CAP = 5_000`, `MAX_PAGE_SIZE = 1_000`,
    and `class TooManyRowsError { total; cap }`.
  - `store/index.ts`: `getStore(): SignalStore`, `isDemo(): boolean`
  - `store/demo.ts`: `demoRows(now: Date, days?: number): Signal[]`, `class DemoStore`
  - `GET /api/signals` → `{ rows, total, page, pageSize }`, or `{ rows, total }` with `all=1`; 413 `{ error }` when over the cap
  - `GET /api/status` → `Status`

- [ ] **Step 1: Write the failing tests**

`web/src/lib/store/query.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { baseFilter } from "@/lib/filters";
import { applyFilterToQuery, pgQuote, selectionFilter, toSignal, type FilterableQuery } from "./query";

class FakeQuery implements FilterableQuery {
  calls: string[] = [];
  gte(c: string, v: string) { this.calls.push(`gte ${c} ${v}`); return this; }
  lte(c: string, v: string) { this.calls.push(`lte ${c} ${v}`); return this; }
  eq(c: string, v: string) { this.calls.push(`eq ${c} ${v}`); return this; }
  or(s: string) { this.calls.push(`or ${s}`); return this; }
  like(c: string, p: string) { this.calls.push(`like ${c} ${p}`); return this; }
  order(c: string, o: { ascending: boolean }) { this.calls.push(`order ${c} ${o.ascending ? "asc" : "desc"}`); return this; }
}

describe("pgQuote / selectionFilter", () => {
  it("quotes values containing spaces and parentheses", () => {
    expect(pgQuote('RSI (GFS)')).toBe('"RSI (GFS)"');
    expect(pgQuote('a"b\\c')).toBe('"a\\"b\\\\c"');
  });
  it("ORs strategies and signals", () => {
    expect(selectionFilter(baseFilter({ strategies: ["RSI"], signals: ["IchimokuKinkoHyo (BUY) 1A"] }))).toBe(
      'strategy.in.("RSI"),signal.in.("IchimokuKinkoHyo (BUY) 1A")',
    );
    expect(selectionFilter(baseFilter())).toBeNull();
  });
});

describe("applyFilterToQuery", () => {
  it("maps every filter field", () => {
    const q = applyFilterToQuery(
      new FakeQuery(),
      baseFilter({ from: "2026-09-01", to: "2026-09-24", strategies: ["RSI"], direction: "long", symbol: "TC", sort: "symbol", order: "asc" }),
    );
    expect(q.calls).toEqual([
      "gte trade_date 2026-09-01",
      "lte trade_date 2026-09-24",
      'or strategy.in.("RSI")',
      "eq direction long",
      "like symbol TC%",
      "order symbol asc",
      "order fired_at desc",
      "order id desc",
    ]);
  });
  it("sorts by time with an id tie-break in the same direction", () => {
    expect(applyFilterToQuery(new FakeQuery(), baseFilter()).calls).toEqual(["order fired_at desc", "order id desc"]);
    expect(applyFilterToQuery(new FakeQuery(), baseFilter({ order: "asc" })).calls).toEqual(["order fired_at asc", "order id asc"]);
  });
});

describe("toSignal", () => {
  it("maps snake_case rows and coerces numerics", () => {
    expect(
      toSignal({
        id: 7, fired_at: "2026-09-24T04:00:00+00:00", trade_date: "2026-09-24", token: 408065, symbol: "INFY",
        strategy: "RSI", signal: "RSI (GFS)", direction: "long", price: "1523.40", rsi_daily: 41.2, rsi_weekly: null, rsi_monthly: "66.10",
      }),
    ).toEqual({
      id: 7, firedAt: "2026-09-24T04:00:00+00:00", tradeDate: "2026-09-24", token: 408065, symbol: "INFY",
      strategy: "RSI", signal: "RSI (GFS)", direction: "long", price: 1523.4, rsiDaily: 41.2, rsiWeekly: null, rsiMonthly: 66.1,
    });
  });
});
```

`web/src/lib/store/demo.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { baseFilter } from "@/lib/filters";
import { tokenValidToday } from "@/lib/time";
import { DemoStore, demoRows } from "./demo";
import { TooManyRowsError } from "./types";

const now = new Date("2026-09-24T06:30:00Z"); // 12:00 IST, a Thursday

describe("demoRows", () => {
  it("is deterministic, weekday-only and never in the future", () => {
    const a = demoRows(now);
    expect(demoRows(now)).toEqual(a);
    expect(a.length).toBeGreaterThan(100);
    for (const r of a) {
      const dow = new Date(`${r.tradeDate}T00:00:00Z`).getUTCDay();
      expect(dow).not.toBe(0);
      expect(dow).not.toBe(6);
      expect(Date.parse(r.firedAt)).toBeLessThanOrEqual(now.getTime());
    }
  });
});

describe("DemoStore", () => {
  it("pages and counts", async () => {
    const store = new DemoStore(now);
    const { rows, total } = await store.list(baseFilter(), 0, 10);
    expect(rows).toHaveLength(10);
    expect(total).toBeGreaterThan(10);
  });
  it("enforces the cap in listAll", async () => {
    await expect(new DemoStore(now).listAll(baseFilter(), 5)).rejects.toBeInstanceOf(TooManyRowsError);
  });
  it("remembers a saved Kite session", async () => {
    const store = new DemoStore(now);
    expect((await store.status()).tokenValidToday).toBe(false);
    await store.saveKiteSession("tok", "AB1234");
    const status = await store.status();
    expect(tokenValidToday(status.tokenCreatedAt)).toBe(true);
    expect(status.tokenValidToday).toBe(true);
  });
});
```

- [ ] **Step 2: Run — expect FAIL**

Run: `npx vitest run src/lib/store`

- [ ] **Step 3: Write `web/src/lib/store/types.ts`**

```ts
import type { Signal, SignalFilter, Status } from "@/lib/types";

export const EXPORT_CAP = 100_000;
export const ALL_CAP = 5_000;
export const MAX_PAGE_SIZE = 1_000;

export class TooManyRowsError extends Error {
  constructor(
    public readonly total: number,
    public readonly cap: number,
  ) {
    super(`${total.toLocaleString("en-US")} signals match — narrow the range to ${cap.toLocaleString("en-US")} or fewer.`);
    this.name = "TooManyRowsError";
  }
}

export interface SignalStore {
  list(f: SignalFilter, offset: number, limit: number): Promise<{ rows: Signal[]; total: number }>;
  listAll(f: SignalFilter, cap: number): Promise<Signal[]>;
  status(): Promise<Status>;
  saveKiteSession(accessToken: string, userId: string): Promise<void>;
}
```

- [ ] **Step 4: Write `web/src/lib/store/query.ts`**

```ts
import type { Direction, Signal, SignalFilter } from "@/lib/types";

export const SIGNAL_COLUMNS =
  "id,fired_at,trade_date,token,symbol,strategy,signal,direction,price,rsi_daily,rsi_weekly,rsi_monthly";

export interface SignalRow {
  id: number;
  fired_at: string;
  trade_date: string;
  token: number;
  symbol: string;
  strategy: string;
  signal: string;
  direction: Direction;
  price: number | string;
  rsi_daily: number | string | null;
  rsi_weekly: number | string | null;
  rsi_monthly: number | string | null;
}

const num = (v: number | string | null): number | null => (v === null ? null : Number(v));

export function toSignal(r: SignalRow): Signal {
  return {
    id: r.id,
    firedAt: r.fired_at,
    tradeDate: r.trade_date,
    token: r.token,
    symbol: r.symbol,
    strategy: r.strategy,
    signal: r.signal,
    direction: r.direction,
    price: Number(r.price),
    rsiDaily: num(r.rsi_daily),
    rsiWeekly: num(r.rsi_weekly),
    rsiMonthly: num(r.rsi_monthly),
  };
}

/** The subset of the PostgREST builder this module drives, so it can be tested with a fake. */
export interface FilterableQuery {
  gte(column: string, value: string): FilterableQuery;
  lte(column: string, value: string): FilterableQuery;
  eq(column: string, value: string): FilterableQuery;
  or(filters: string): FilterableQuery;
  like(column: string, pattern: string): FilterableQuery;
  order(column: string, options: { ascending: boolean }): FilterableQuery;
}

/** Double-quote a value for a PostgREST `in.(...)` list; names contain spaces and parentheses. */
export function pgQuote(value: string): string {
  return `"${value.replace(/\\/g, "\\\\").replace(/"/g, '\\"')}"`;
}

export function selectionFilter(f: SignalFilter): string | null {
  const parts: string[] = [];
  if (f.strategies.length) parts.push(`strategy.in.(${f.strategies.map(pgQuote).join(",")})`);
  if (f.signals.length) parts.push(`signal.in.(${f.signals.map(pgQuote).join(",")})`);
  return parts.length ? parts.join(",") : null;
}

/** Applies the filter and ordering. Every builder method returns the same builder, so the result is still a T. */
export function applyFilterToQuery<T extends FilterableQuery>(query: T, f: SignalFilter): T {
  let q: FilterableQuery = query;
  if (f.from) q = q.gte("trade_date", f.from);
  if (f.to) q = q.lte("trade_date", f.to);
  const selection = selectionFilter(f);
  if (selection) q = q.or(selection);
  if (f.direction) q = q.eq("direction", f.direction);
  if (f.symbol) q = q.like("symbol", `${f.symbol}%`); // parseFilter guarantees [A-Z0-9&-]

  const asc = f.order === "asc";
  if (f.sort === "symbol" || f.sort === "signal") {
    q = q.order(f.sort, { ascending: asc }).order("fired_at", { ascending: false }).order("id", { ascending: false });
  } else {
    q = q.order("fired_at", { ascending: asc }).order("id", { ascending: asc });
  }
  return q as T;
}
```

- [ ] **Step 5: Write `web/src/lib/store/supabase.ts`**

```ts
import "server-only";
import { createClient, type SupabaseClient } from "@supabase/supabase-js";
import { tokenValidToday } from "@/lib/time";
import type { Signal, SignalFilter, Status } from "@/lib/types";
import { SIGNAL_COLUMNS, applyFilterToQuery, toSignal, type FilterableQuery, type SignalRow } from "./query";
import { MAX_PAGE_SIZE, TooManyRowsError, type SignalStore } from "./types";

export class SupabaseStore implements SignalStore {
  private db: SupabaseClient;

  constructor(url: string, serviceKey: string) {
    this.db = createClient(url, serviceKey, { auth: { persistSession: false, autoRefreshToken: false } });
  }

  async list(f: SignalFilter, offset: number, limit: number): Promise<{ rows: Signal[]; total: number }> {
    const base = this.db.from("signals").select(SIGNAL_COLUMNS, { count: "exact" });
    // Every PostgREST builder method returns the same builder, so it can be driven through the
    // narrow FilterableQuery interface and cast back to keep .range().
    const query = applyFilterToQuery(base as unknown as FilterableQuery, f) as unknown as typeof base;
    const { data, count, error } = await query.range(offset, offset + limit - 1);
    if (error) throw new Error(`Supabase: ${error.message}`);
    return { rows: (data as unknown as SignalRow[]).map(toSignal), total: count ?? 0 };
  }

  async listAll(f: SignalFilter, cap: number): Promise<Signal[]> {
    const rows: Signal[] = [];
    for (let offset = 0; ; offset += MAX_PAGE_SIZE) {
      const page = await this.list(f, offset, MAX_PAGE_SIZE);
      if (page.total > cap) throw new TooManyRowsError(page.total, cap);
      rows.push(...page.rows);
      if (page.rows.length < MAX_PAGE_SIZE || rows.length >= page.total) return rows;
    }
  }

  async status(): Promise<Status> {
    const [kite, engine] = await Promise.all([
      // Never select access_token: the browser must not be able to learn it.
      this.db.from("kite_session").select("created_at").eq("id", 1).maybeSingle(),
      this.db.from("engine_status").select("last_seen,instruments,feed_connected,token_valid").eq("id", 1).maybeSingle(),
    ]);
    if (kite.error) throw new Error(`Supabase: ${kite.error.message}`);
    if (engine.error) throw new Error(`Supabase: ${engine.error.message}`);

    const createdAt = (kite.data?.created_at as string | undefined) ?? null;
    const e = engine.data;
    return {
      tokenCreatedAt: createdAt,
      tokenValidToday: tokenValidToday(createdAt),
      engine: e
        ? { lastSeen: e.last_seen, instruments: e.instruments, feedConnected: e.feed_connected, tokenValid: e.token_valid }
        : null,
    };
  }

  async saveKiteSession(accessToken: string, userId: string): Promise<void> {
    const { error } = await this.db
      .from("kite_session")
      .upsert({ id: 1, access_token: accessToken, user_id: userId, created_at: new Date().toISOString() });
    if (error) throw new Error(`Supabase: ${error.message}`);
  }
}
```

- [ ] **Step 6: Write `web/src/lib/store/demo.ts`**

```ts
import { STRATEGIES, directionOf } from "@/lib/catalog";
import { applyFilter } from "@/lib/filter-rows";
import { addDays, todayIST, tokenValidToday } from "@/lib/time";
import type { Signal, SignalFilter, Status } from "@/lib/types";
import { TooManyRowsError, type SignalStore } from "./types";

const SYMBOLS = [
  "RELIANCE", "TCS", "HDFCBANK", "INFY", "ICICIBANK", "SBIN", "BHARTIARTL", "ITC", "LT", "KOTAKBANK",
  "AXISBANK", "HINDUNILVR", "BAJFINANCE", "MARUTI", "SUNPHARMA", "TITAN", "ASIANPAINT", "ULTRACEMCO",
  "WIPRO", "NESTLEIND", "TATAMOTORS", "TATASTEEL", "POWERGRID", "NTPC", "ONGC", "ADANIENT", "JSWSTEEL",
  "HCLTECH", "TECHM", "COALINDIA",
];

function mulberry32(seed: number): () => number {
  return () => {
    seed = (seed + 0x6d2b79f5) | 0;
    let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

const round = (n: number, dp: number) => Math.round(n * 10 ** dp) / 10 ** dp;

/** Plausible, deterministic signals for the last `days` calendar days — weekdays, 09:15–15:30 IST, never after `now`. */
export function demoRows(now: Date, days = 30): Signal[] {
  const rand = mulberry32(42);
  const today = todayIST(now);
  const catalog = Object.entries(STRATEGIES).flatMap(([strategy, signals]) => signals.map((signal) => ({ strategy, signal })));
  const rows: Signal[] = [];
  let id = 1;

  for (let back = days - 1; back >= 0; back--) {
    const date = addDays(today, -back);
    const dow = new Date(`${date}T00:00:00Z`).getUTCDay();
    if (dow === 0 || dow === 6) continue;
    const open = Date.parse(`${date}T09:15:00+05:30`);
    const n = 20 + Math.floor(rand() * 40);

    for (let i = 0; i < n; i++) {
      const pick = catalog[Math.floor(rand() * catalog.length)];
      const fired = open + Math.floor(rand() * 375 * 60) * 1000; // within 09:15–15:30
      const symbolIdx = Math.floor(rand() * SYMBOLS.length);
      const price = round(100 + rand() * 2900, 2);
      const rsi = [round(20 + rand() * 60, 2), round(20 + rand() * 60, 2), round(20 + rand() * 60, 2)];
      if (fired > now.getTime()) continue;
      const isRSI = pick.strategy === "RSI";
      rows.push({
        id: id++,
        firedAt: new Date(fired).toISOString(),
        tradeDate: date,
        token: 100000 + symbolIdx,
        symbol: SYMBOLS[symbolIdx],
        strategy: pick.strategy,
        signal: pick.signal,
        direction: directionOf(pick.signal),
        price,
        rsiDaily: isRSI ? rsi[0] : null,
        rsiWeekly: isRSI ? rsi[1] : null,
        rsiMonthly: isRSI ? rsi[2] : null,
      });
    }
  }
  return rows;
}

/** In-memory store for `npm run dev` without Supabase. Never used in production. */
export class DemoStore implements SignalStore {
  private rows: Signal[];
  private kiteCreatedAt: string | null = null;

  constructor(now: Date = new Date()) {
    this.rows = demoRows(now);
  }

  async list(f: SignalFilter, offset: number, limit: number) {
    const all = applyFilter(this.rows, f);
    return { rows: all.slice(offset, offset + limit), total: all.length };
  }

  async listAll(f: SignalFilter, cap: number) {
    const all = applyFilter(this.rows, f);
    if (all.length > cap) throw new TooManyRowsError(all.length, cap);
    return all;
  }

  async status(): Promise<Status> {
    const valid = tokenValidToday(this.kiteCreatedAt);
    return {
      tokenCreatedAt: this.kiteCreatedAt,
      tokenValidToday: valid,
      engine: { lastSeen: new Date(Date.now() - 20_000).toISOString(), instruments: 500, feedConnected: true, tokenValid: valid },
    };
  }

  async saveKiteSession(): Promise<void> {
    this.kiteCreatedAt = new Date().toISOString();
  }
}
```

- [ ] **Step 7: Write `web/src/lib/store/index.ts`**

```ts
import "server-only";
import { DemoStore } from "./demo";
import { SupabaseStore } from "./supabase";
import type { SignalStore } from "./types";

// Cached on globalThis so every route in `next dev` shares one DemoStore.
const g = globalThis as unknown as { __signalStore?: SignalStore };

export function isDemo(): boolean {
  return !(process.env.SUPABASE_URL && process.env.SUPABASE_SERVICE_KEY);
}

export function getStore(): SignalStore {
  if (g.__signalStore) return g.__signalStore;
  if (!isDemo()) {
    g.__signalStore = new SupabaseStore(process.env.SUPABASE_URL!, process.env.SUPABASE_SERVICE_KEY!);
  } else if (process.env.NODE_ENV !== "production") {
    g.__signalStore = new DemoStore();
  } else {
    throw new Error("SUPABASE_URL and SUPABASE_SERVICE_KEY must be set in production.");
  }
  return g.__signalStore;
}
```

- [ ] **Step 8: Write `web/src/app/api/signals/route.ts`**

```ts
import { NextResponse, type NextRequest } from "next/server";
import { parseFilter } from "@/lib/filters";
import { getStore } from "@/lib/store";
import { ALL_CAP, MAX_PAGE_SIZE, TooManyRowsError } from "@/lib/store/types";

const int = (v: string | null, fallback: number) => {
  const n = Number.parseInt(v ?? "", 10);
  return Number.isFinite(n) ? n : fallback;
};

export async function GET(request: NextRequest) {
  const p = request.nextUrl.searchParams;
  const filter = parseFilter(p);
  const store = getStore();

  try {
    if (p.get("all") === "1") {
      const rows = await store.listAll(filter, ALL_CAP);
      return NextResponse.json({ rows, total: rows.length });
    }
    const pageSize = Math.min(Math.max(int(p.get("pageSize"), 100), 1), MAX_PAGE_SIZE);
    const page = Math.max(int(p.get("page"), 1), 1);
    const { rows, total } = await store.list(filter, (page - 1) * pageSize, pageSize);
    return NextResponse.json({ rows, total, page, pageSize });
  } catch (e) {
    if (e instanceof TooManyRowsError) return NextResponse.json({ error: e.message }, { status: 413 });
    throw e;
  }
}
```

- [ ] **Step 9: Write `web/src/app/api/status/route.ts`**

```ts
import { NextResponse } from "next/server";
import { getStore } from "@/lib/store";

export async function GET() {
  return NextResponse.json(await getStore().status());
}
```

- [ ] **Step 10: Run tests, typecheck and lint — expect PASS**

Run: `npx vitest run && npx tsc --noEmit && npm run lint`

- [ ] **Step 11: Smoke-test the API** (dev server running with the cookie jar from Task 4)

- `curl -s -b jar "http://localhost:3000/api/signals?pageSize=2" | head -c 400` → JSON with 2 rows and `total` > 2
- `curl -s -b jar "http://localhost:3000/api/status"` → JSON without any `access_token` field

- [ ] **Step 12: Commit**

```bash
git add web/src/lib/store web/src/app/api/signals web/src/app/api/status
git commit -m "feat(web): Supabase and demo signal stores with JSON API"
```

---

### Task 7: Kite login flow

**Files:**
- Create: `web/src/lib/kite.ts`, `web/src/app/api/kite/login/route.ts`, `web/src/app/api/kite/callback/route.ts`
- Test: `web/src/lib/kite.test.ts`

**Interfaces:**
- Consumes: `getStore().saveKiteSession` (Task 6).
- Produces:
  - `kiteLoginUrl(apiKey: string): string`
  - `kiteChecksum(apiKey: string, requestToken: string, apiSecret: string): string`
  - `completeKiteLogin(input, env, deps): Promise<KiteResult>`, where:
    - `input: { requestToken: string | null; status: string | null }`
    - `env: KiteEnv`
    - `deps: { fetch: typeof fetch; save(accessToken: string, userId: string): Promise<void> }`
    - `KiteResult = { ok: true } | { ok: false; reason: string }`
  - The callback redirects to `/?kite=ok` or `/?kite=error&reason=…`.

- [ ] **Step 1: Write the failing test `web/src/lib/kite.test.ts`**

```ts
import { createHash } from "node:crypto";
import { describe, expect, it, vi } from "vitest";
import { completeKiteLogin, kiteChecksum, kiteLoginUrl } from "./kite";

const env = { apiKey: "key123", apiSecret: "sec456", userId: "AB1234" };

function kiteResponse(body: unknown, status = 200) {
  return vi.fn(async () => new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } }));
}

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
    const save = vi.fn(async () => {});
    const res = await completeKiteLogin({ requestToken: "rt", status: "success" }, env, { fetch, save });

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
    const save = vi.fn(async () => {});
    const res = await completeKiteLogin(
      { requestToken: "rt", status: "success" },
      env,
      { fetch: kiteResponse({ status: "success", data: { user_id: "ZZ9999", access_token: "tok" } }), save },
    );
    expect(res.ok).toBe(false);
    expect(save).not.toHaveBeenCalled();
  });

  it("matches the user ID case-insensitively", async () => {
    const res = await completeKiteLogin(
      { requestToken: "rt", status: "success" },
      { ...env, userId: "ab1234" },
      { fetch: kiteResponse({ status: "success", data: { user_id: "AB1234", access_token: "tok" } }), save: async () => {} },
    );
    expect(res).toEqual({ ok: true });
  });

  it("reports Zerodha's error message", async () => {
    const res = await completeKiteLogin(
      { requestToken: "rt", status: "success" },
      env,
      { fetch: kiteResponse({ status: "error", message: "Token is invalid or has expired." }, 403), save: async () => {} },
    );
    expect(res).toEqual({ ok: false, reason: "Token is invalid or has expired." });
  });

  it("does nothing when the login was cancelled", async () => {
    const fetch = vi.fn();
    const res = await completeKiteLogin({ requestToken: null, status: "cancelled" }, env, { fetch, save: async () => {} });
    expect(res.ok).toBe(false);
    expect(fetch).not.toHaveBeenCalled();
  });

  it("fails clearly when Kite isn't configured", async () => {
    const res = await completeKiteLogin({ requestToken: "rt", status: "success" }, {}, { fetch: vi.fn(), save: async () => {} });
    expect(res).toEqual({ ok: false, reason: "Kite is not configured on the server (KITE_API_KEY, KITE_API_SECRET, KITE_USER_ID)." });
  });

  it("survives network and save failures", async () => {
    const down = await completeKiteLogin(
      { requestToken: "rt", status: "success" },
      env,
      { fetch: vi.fn(async () => { throw new Error("ECONNRESET"); }), save: async () => {} },
    );
    expect(down).toEqual({ ok: false, reason: "Could not reach Zerodha." });

    const saveFails = await completeKiteLogin(
      { requestToken: "rt", status: "success" },
      env,
      {
        fetch: kiteResponse({ status: "success", data: { user_id: "AB1234", access_token: "tok" } }),
        save: async () => { throw new Error("db down"); },
      },
    );
    expect(saveFails).toEqual({ ok: false, reason: "Logged in, but the token could not be saved: db down" });
  });
});
```

- [ ] **Step 2: Run — expect FAIL**

Run: `npx vitest run src/lib/kite.test.ts`

- [ ] **Step 3: Write `web/src/lib/kite.ts`**

```ts
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
    return { ok: false, reason: `Logged in, but the token could not be saved: ${e instanceof Error ? e.message : String(e)}` };
  }
  return { ok: true };
}
```

- [ ] **Step 4: Run — expect PASS**

Run: `npx vitest run src/lib/kite.test.ts`

- [ ] **Step 5: Write the routes**

`web/src/app/api/kite/login/route.ts`:

```ts
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
```

`web/src/app/api/kite/callback/route.ts`:

```ts
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
```

- [ ] **Step 6: Typecheck, lint and commit**

Run: `npx tsc --noEmit && npm run lint`

```bash
git add web/src/lib/kite.ts web/src/lib/kite.test.ts web/src/app/api/kite
git commit -m "feat(web): one-click Kite login that stores the daily token"
```

---

### Task 8: Excel export

**Files:**
- Create: `web/src/lib/excel.ts`, `web/src/lib/export-fetch.ts`, `web/src/components/ExportButton.tsx`
- Test: `web/src/lib/excel.test.ts`, `web/src/lib/export-fetch.test.ts`

**Interfaces:**
- Consumes: `summarize` (Task 5), `istTime` (Task 3), `filterToParams` (Task 5), `EXPORT_CAP`/`MAX_PAGE_SIZE` values (Task 6; duplicated as constants here, since client code must not import the store).
- Produces:
  - `excel.ts`:
    - `EXCEL_HEADERS: string[]`
    - `buildWorkbook(rows: Signal[]): ExcelJS.Workbook`
    - `workbookBlob(wb): Promise<Blob>`
    - `exportFileName(f: SignalFilter): string`
  - `export-fetch.ts`: `fetchAllSignals(filter, fetchFn?, onProgress?): Promise<Signal[]>`, which throws `Error` with a readable message
  - `<ExportButton filter={SignalFilter} label?="Export to Excel" />`

- [ ] **Step 1: Write the failing tests**

`web/src/lib/excel.test.ts`:

```ts
import ExcelJS from "exceljs";
import { describe, expect, it } from "vitest";
import { EXCEL_HEADERS, buildWorkbook, exportFileName } from "./excel";
import { baseFilter } from "./filters";
import type { Signal } from "./types";

const rows: Signal[] = [
  {
    id: 1, firedAt: "2026-09-24T04:05:06Z", tradeDate: "2026-09-24", token: 1, symbol: "INFY", strategy: "RSI",
    signal: "RSI (GFS)", direction: "long", price: 1523.4, rsiDaily: 41.25, rsiWeekly: 63, rsiMonthly: 66.1,
  },
  {
    id: 2, firedAt: "2026-09-24T05:00:00Z", tradeDate: "2026-09-24", token: 2, symbol: "TCS", strategy: "IchimokuKinkoHyo",
    signal: "IchimokuKinkoHyo (SELL) 1A", direction: "short", price: 3999, rsiDaily: null, rsiWeekly: null, rsiMonthly: null,
  },
];

async function roundTrip(wb: ExcelJS.Workbook) {
  const loaded = new ExcelJS.Workbook();
  await loaded.xlsx.load(await wb.xlsx.writeBuffer());
  return loaded;
}

describe("buildWorkbook", () => {
  it("writes a Signals sheet with the agreed columns", async () => {
    const wb = await roundTrip(buildWorkbook(rows));
    const ws = wb.getWorksheet("Signals")!;
    expect((ws.getRow(1).values as unknown[]).slice(1)).toEqual(EXCEL_HEADERS);
    expect((ws.getRow(2).values as unknown[]).slice(1)).toEqual([
      "2026-09-24", "09:35:06", "INFY", "RSI", "RSI (GFS)", "long", 1523.4, 41.25, 63, 66.1,
    ]);
    expect(ws.getRow(3).getCell(8).value).toBeNull(); // no RSI for Ichimoku
    expect(ws.rowCount).toBe(3);
  });

  it("writes a Summary sheet with per-signal counts and a total", async () => {
    const wb = await roundTrip(buildWorkbook(rows));
    const ws = wb.getWorksheet("Summary")!;
    const values = [2, 3, 4].map((r) => (ws.getRow(r).values as unknown[]).slice(1));
    expect(values).toEqual([
      ["RSI", "RSI (GFS)", 1, 0, 1],
      ["IchimokuKinkoHyo", "IchimokuKinkoHyo (SELL) 1A", 0, 1, 1],
      ["All", undefined, 1, 1, 2],
    ]);
  });
});

describe("exportFileName", () => {
  it("names a single day", () => {
    expect(exportFileName(baseFilter({ from: "2026-09-24", to: "2026-09-24" }))).toBe("signals_2026-09-24.xlsx");
  });
  it("names a range with a single selected strategy or signal", () => {
    expect(exportFileName(baseFilter({ from: "2026-09-01", to: "2026-09-24", strategies: ["RSI"] }))).toBe(
      "signals_2026-09-01_to_2026-09-24_RSI.xlsx",
    );
    expect(exportFileName(baseFilter({ from: "2026-09-24", to: "2026-09-24", signals: ["IchimokuKinkoHyo (BUY) 1A"] }))).toBe(
      "signals_2026-09-24_IchimokuKinkoHyo-BUY-1A.xlsx",
    );
  });
  it("handles open ranges and multi-selections", () => {
    expect(exportFileName(baseFilter())).toBe("signals_all.xlsx");
    expect(exportFileName(baseFilter({ to: "2026-09-24", strategies: ["RSI", "IchimokuKinkoHyo"] }))).toBe(
      "signals_start_to_2026-09-24.xlsx",
    );
  });
});
```

`web/src/lib/export-fetch.test.ts`:

```ts
import { describe, expect, it, vi } from "vitest";
import { fetchAllSignals } from "./export-fetch";
import { baseFilter } from "./filters";

function pages(total: number, pageSize = 1000) {
  return vi.fn(async (url: string) => {
    const page = Number(new URL(url, "http://x").searchParams.get("page"));
    const start = (page - 1) * pageSize;
    const rows = Array.from({ length: Math.max(0, Math.min(pageSize, total - start)) }, (_, i) => ({ id: start + i + 1 }));
    return new Response(JSON.stringify({ rows, total, page, pageSize }), { status: 200 });
  });
}

describe("fetchAllSignals", () => {
  it("walks every page and keeps the filter", async () => {
    const f = pages(2500);
    const rows = await fetchAllSignals(baseFilter({ strategies: ["RSI"] }), f as unknown as typeof fetch);
    expect(rows).toHaveLength(2500);
    expect(f).toHaveBeenCalledTimes(3);
    expect(f.mock.calls[0][0]).toContain("strategy=RSI");
    expect(f.mock.calls[0][0]).toContain("pageSize=1000");
  });

  it("refuses more than 100,000 rows before downloading them", async () => {
    const f = pages(150_000);
    await expect(fetchAllSignals(baseFilter(), f as unknown as typeof fetch)).rejects.toThrow(/150,000/);
    expect(f).toHaveBeenCalledTimes(1);
  });

  it("surfaces the server's error message", async () => {
    const f = vi.fn(async () => new Response(JSON.stringify({ error: "Locked." }), { status: 401 }));
    await expect(fetchAllSignals(baseFilter(), f as unknown as typeof fetch)).rejects.toThrow("Locked.");
  });
});
```

- [ ] **Step 2: Run — expect FAIL**

Run: `npx vitest run src/lib/excel.test.ts src/lib/export-fetch.test.ts`

- [ ] **Step 3: Write `web/src/lib/excel.ts`**

```ts
import ExcelJS from "exceljs";
import { summarize } from "./summary";
import { istTime } from "./time";
import type { Signal, SignalFilter } from "./types";

const COLUMNS = [
  { header: "Date", key: "date", width: 12 },
  { header: "Time (IST)", key: "time", width: 11 },
  { header: "Symbol", key: "symbol", width: 16 },
  { header: "Strategy", key: "strategy", width: 18 },
  { header: "Signal", key: "signal", width: 38 },
  { header: "Direction", key: "direction", width: 10 },
  { header: "Price", key: "price", width: 11 },
  { header: "RSI Daily", key: "rsiDaily", width: 10 },
  { header: "RSI Weekly", key: "rsiWeekly", width: 11 },
  { header: "RSI Monthly", key: "rsiMonthly", width: 12 },
];

export const EXCEL_HEADERS = COLUMNS.map((c) => c.header);

export function buildWorkbook(rows: Signal[]): ExcelJS.Workbook {
  const wb = new ExcelJS.Workbook();
  wb.created = new Date();

  const ws = wb.addWorksheet("Signals", { views: [{ state: "frozen", ySplit: 1 }] });
  ws.columns = COLUMNS;
  for (const r of rows) {
    ws.addRow({
      date: r.tradeDate,
      time: istTime(r.firedAt),
      symbol: r.symbol,
      strategy: r.strategy,
      signal: r.signal,
      direction: r.direction,
      price: r.price,
      rsiDaily: r.rsiDaily ?? null,
      rsiWeekly: r.rsiWeekly ?? null,
      rsiMonthly: r.rsiMonthly ?? null,
    });
  }
  ws.getRow(1).font = { bold: true };
  ws.autoFilter = { from: { row: 1, column: 1 }, to: { row: 1, column: COLUMNS.length } };
  ws.getColumn("price").numFmt = "#,##0.00";
  for (const key of ["rsiDaily", "rsiWeekly", "rsiMonthly"]) ws.getColumn(key).numFmt = "0.00";

  const sum = wb.addWorksheet("Summary", { views: [{ state: "frozen", ySplit: 1 }] });
  sum.columns = [
    { header: "Strategy", key: "strategy", width: 18 },
    { header: "Signal", key: "signal", width: 38 },
    { header: "Long", key: "long", width: 8 },
    { header: "Short", key: "short", width: 8 },
    { header: "Total", key: "total", width: 8 },
  ];
  let long = 0;
  let short = 0;
  for (const s of summarize(rows)) {
    long += s.long;
    short += s.short;
    for (const g of s.signals) sum.addRow(g);
  }
  sum.addRow({ strategy: "All", long, short, total: rows.length }).font = { bold: true };
  sum.getRow(1).font = { bold: true };

  return wb;
}

export async function workbookBlob(wb: ExcelJS.Workbook): Promise<Blob> {
  const buf = await wb.xlsx.writeBuffer();
  return new Blob([buf], { type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" });
}

export function exportFileName(f: SignalFilter): string {
  const range = !f.from && !f.to ? "all" : f.from && f.from === f.to ? f.from : `${f.from ?? "start"}_to_${f.to ?? "today"}`;
  const only =
    f.signals.length === 1 && !f.strategies.length
      ? f.signals[0]
      : f.strategies.length === 1 && !f.signals.length
        ? f.strategies[0]
        : "";
  const slug = only.replace(/[^A-Za-z0-9]+/g, "-").replace(/^-+|-+$/g, "");
  return `signals_${range}${slug ? `_${slug}` : ""}.xlsx`;
}
```

In the Summary test, the "All" row's Signal cell is `undefined` because no
value is written there, so the expected array has `undefined` at index 1.

- [ ] **Step 4: Write `web/src/lib/export-fetch.ts`**

```ts
import { filterToParams } from "./filters";
import type { Signal, SignalFilter } from "./types";

// Mirrors EXPORT_CAP and MAX_PAGE_SIZE in lib/store/types.ts (client code can't import the store).
const EXPORT_CAP = 100_000;
const PAGE_SIZE = 1_000;

interface Page {
  rows: Signal[];
  total: number;
  error?: string;
}

async function getPage(filter: SignalFilter, page: number, fetchFn: typeof fetch): Promise<Page> {
  const params = filterToParams(filter);
  params.set("page", String(page));
  params.set("pageSize", String(PAGE_SIZE));
  const res = await fetchFn(`/api/signals?${params}`, { cache: "no-store" });
  const body = (await res.json().catch(() => ({}))) as Page;
  if (!res.ok) throw new Error(body.error ?? `Export failed (HTTP ${res.status}).`);
  return body;
}

/** Fetches every row matching the filter, one page at a time (Vercel caps each response at ~4.5 MB). */
export async function fetchAllSignals(
  filter: SignalFilter,
  fetchFn: typeof fetch = fetch,
  onProgress?: (done: number, total: number) => void,
): Promise<Signal[]> {
  const first = await getPage(filter, 1, fetchFn);
  if (first.total > EXPORT_CAP) {
    throw new Error(
      `${first.total.toLocaleString("en-US")} signals match — narrow the range to ${EXPORT_CAP.toLocaleString("en-US")} or fewer.`,
    );
  }
  const rows = [...first.rows];
  onProgress?.(rows.length, first.total);
  for (let page = 2; rows.length < first.total; page++) {
    const next = await getPage(filter, page, fetchFn);
    if (next.rows.length === 0) break;
    rows.push(...next.rows);
    onProgress?.(rows.length, first.total);
  }
  return rows;
}
```

- [ ] **Step 5: Write `web/src/components/ExportButton.tsx`**

```tsx
"use client";

import { useState } from "react";
import { fetchAllSignals } from "@/lib/export-fetch";
import type { SignalFilter } from "@/lib/types";

export function ExportButton({ filter, label = "Export to Excel" }: { filter: SignalFilter; label?: string }) {
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function run() {
    setError(null);
    setBusy("Fetching…");
    try {
      const rows = await fetchAllSignals(filter, fetch, (done, total) => setBusy(`Fetching ${done}/${total}…`));
      if (rows.length === 0) {
        setError("Nothing to export for this selection.");
        return;
      }
      setBusy("Building file…");
      const { buildWorkbook, workbookBlob, exportFileName } = await import("@/lib/excel");
      const blob = await workbookBlob(buildWorkbook(rows));
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = exportFileName(filter);
      a.click();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(null);
    }
  }

  return (
    <span className="inline-flex flex-col items-end gap-1">
      <button
        type="button"
        onClick={run}
        disabled={busy !== null}
        className="rounded-md border border-emerald-600 px-3 py-1.5 text-sm font-medium text-emerald-700 hover:bg-emerald-50 disabled:opacity-60 dark:text-emerald-400 dark:hover:bg-emerald-950"
      >
        {busy ?? label}
      </button>
      {error && <span className="max-w-xs text-right text-xs text-red-600">{error}</span>}
    </span>
  );
}
```

- [ ] **Step 6: Run — expect PASS**

Run: `npx vitest run && npx tsc --noEmit && npm run lint`

- [ ] **Step 7: Commit**

```bash
git add web/src/lib/excel.ts web/src/lib/excel.test.ts web/src/lib/export-fetch.ts web/src/lib/export-fetch.test.ts web/src/components/ExportButton.tsx
git commit -m "feat(web): Excel export with summary sheet and paged fetching"
```

---

### Task 9: Site shell and dashboard

**Files:**
- Delete: `web/src/app/page.tsx`
- Create: `web/src/app/(site)/layout.tsx`, `web/src/app/(site)/page.tsx`, `web/src/components/Header.tsx`, `web/src/components/NavLinks.tsx`, `web/src/components/SignalsTable.tsx`, `web/src/components/SummaryCards.tsx`, `web/src/components/LiveSignals.tsx`, `web/src/components/KiteFlash.tsx`

**Interfaces:**
- Consumes: `getStore`, `isDemo`, `ALL_CAP` (Task 6); `engineHealth`, `ago`, `istTime`, `todayIST` (Task 3); `summarize`, `baseFilter`, `first` (Task 5); `shortSignalName` (Task 2); `ExportButton` (Task 8).
- Produces:
  - `<SignalsTable rows showDate? sortLinks?: Partial<Record<SortKey, string>> activeSort?: SortKey order?: "asc"|"desc" emptyText? />`: no `"use client"`, so it's usable from server and client components.
  - `<SummaryCards rows />`
  - `<LiveSignals date initialRows />` (client component, refreshes every 30 s)

- [ ] **Step 1: Write `web/src/components/SignalsTable.tsx`**

```tsx
import { shortSignalName } from "@/lib/catalog";
import { istTime } from "@/lib/time";
import type { Signal, SortKey } from "@/lib/types";

const price = new Intl.NumberFormat("en-IN", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
const rsi = (v: number | null) => (v === null ? "—" : v.toFixed(1));

export function DirectionBadge({ direction }: { direction: Signal["direction"] }) {
  return direction === "long" ? (
    <span className="rounded bg-emerald-100 px-1.5 py-0.5 text-xs font-medium text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300">Long</span>
  ) : (
    <span className="rounded bg-rose-100 px-1.5 py-0.5 text-xs font-medium text-rose-800 dark:bg-rose-950 dark:text-rose-300">Short</span>
  );
}

export function SignalsTable({
  rows,
  showDate = false,
  sortLinks,
  activeSort,
  order,
  emptyText = "No signals.",
}: {
  rows: Signal[];
  showDate?: boolean;
  sortLinks?: Partial<Record<SortKey, string>>;
  activeSort?: SortKey;
  order?: "asc" | "desc";
  emptyText?: string;
}) {
  if (rows.length === 0) {
    return <p className="rounded-lg border border-dashed border-slate-300 p-6 text-center text-sm text-slate-500 dark:border-slate-700">{emptyText}</p>;
  }

  const head = (label: string, key?: SortKey) => {
    const href = key ? sortLinks?.[key] : undefined;
    if (!href) return label;
    const arrow = activeSort === key ? (order === "asc" ? " ↑" : " ↓") : "";
    return (
      <a href={href} className="hover:underline">
        {label}
        {arrow}
      </a>
    );
  };

  return (
    <div className="overflow-x-auto rounded-lg border border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
      <table className="min-w-full text-sm">
        <thead className="bg-slate-100 text-left text-xs uppercase tracking-wide text-slate-600 dark:bg-slate-800 dark:text-slate-300">
          <tr>
            {showDate && <th className="px-3 py-2">Date</th>}
            <th className="px-3 py-2">{head("Time", "time")}</th>
            <th className="px-3 py-2">{head("Symbol", "symbol")}</th>
            <th className="px-3 py-2">{head("Signal", "signal")}</th>
            <th className="px-3 py-2">Side</th>
            <th className="px-3 py-2 text-right">Price</th>
            <th className="px-3 py-2 text-right">RSI D / W / M</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-100 dark:divide-slate-800">
          {rows.map((r) => (
            <tr key={r.id} className="hover:bg-slate-50 dark:hover:bg-slate-800/50">
              {showDate && <td className="whitespace-nowrap px-3 py-2 tabular-nums">{r.tradeDate}</td>}
              <td className="whitespace-nowrap px-3 py-2 tabular-nums">{istTime(r.firedAt)}</td>
              <td className="whitespace-nowrap px-3 py-2 font-medium">{r.symbol}</td>
              <td className="whitespace-nowrap px-3 py-2">
                {shortSignalName(r.signal, r.strategy)}
                <span className="ml-2 text-xs text-slate-500">{r.strategy}</span>
              </td>
              <td className="px-3 py-2">
                <DirectionBadge direction={r.direction} />
              </td>
              <td className="whitespace-nowrap px-3 py-2 text-right tabular-nums">{price.format(r.price)}</td>
              <td className="whitespace-nowrap px-3 py-2 text-right tabular-nums text-slate-600 dark:text-slate-400">
                {r.rsiDaily === null && r.rsiWeekly === null && r.rsiMonthly === null
                  ? "—"
                  : `${rsi(r.rsiDaily)} / ${rsi(r.rsiWeekly)} / ${rsi(r.rsiMonthly)}`}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
```

- [ ] **Step 2: Write `web/src/components/SummaryCards.tsx`**

```tsx
import { summarize } from "@/lib/summary";
import type { Signal } from "@/lib/types";

export function SummaryCards({ rows }: { rows: Signal[] }) {
  const counts = summarize(rows);
  const long = rows.filter((r) => r.direction === "long").length;
  const cards = [{ strategy: "All signals", total: rows.length, long, short: rows.length - long }, ...counts];

  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
      {cards.map((c) => (
        <div key={c.strategy} className="rounded-lg border border-slate-200 bg-white p-3 dark:border-slate-800 dark:bg-slate-900">
          <div className="truncate text-xs text-slate-500">{c.strategy}</div>
          <div className="text-2xl font-semibold tabular-nums">{c.total}</div>
          <div className="text-xs tabular-nums">
            <span className="text-emerald-700 dark:text-emerald-400">{c.long} long</span>
            {" · "}
            <span className="text-rose-700 dark:text-rose-400">{c.short} short</span>
          </div>
        </div>
      ))}
    </div>
  );
}
```

- [ ] **Step 3: Write `web/src/components/LiveSignals.tsx`**

```tsx
"use client";

import { useEffect, useState } from "react";
import { baseFilter } from "@/lib/filters";
import { istTime } from "@/lib/time";
import type { Signal } from "@/lib/types";
import { ExportButton } from "./ExportButton";
import { SignalsTable } from "./SignalsTable";
import { SummaryCards } from "./SummaryCards";

const REFRESH_MS = 30_000;

export function LiveSignals({ date, initialRows }: { date: string; initialRows: Signal[] }) {
  const [rows, setRows] = useState(initialRows);
  const [updated, setUpdated] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    const refresh = async () => {
      try {
        const res = await fetch(`/api/signals?from=${date}&to=${date}&all=1`, { cache: "no-store" });
        if (!res.ok) return;
        const body = (await res.json()) as { rows: Signal[] };
        if (!cancelled) {
          setRows(body.rows);
          setUpdated(istTime(new Date()));
        }
      } catch {
        // Offline or redeploying; try again on the next tick.
      }
    };
    const id = setInterval(refresh, REFRESH_MS);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [date]);

  return (
    <div className="space-y-4">
      <SummaryCards rows={rows} />
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h2 className="text-lg font-semibold">Latest signals</h2>
          <p className="text-xs text-slate-500">{updated ? `Updated ${updated} IST` : "Refreshes every 30 seconds"}</p>
        </div>
        <ExportButton filter={baseFilter({ from: date, to: date })} label="Export today" />
      </div>
      <SignalsTable rows={rows} emptyText="No signals yet today." />
    </div>
  );
}
```

- [ ] **Step 4: Write `web/src/components/NavLinks.tsx`**

```tsx
"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

const LINKS = [
  { href: "/", label: "Dashboard" },
  { href: "/strategies", label: "Strategies" },
  { href: "/history", label: "History" },
];

export function NavLinks() {
  const pathname = usePathname();
  return (
    <nav className="flex gap-1">
      {LINKS.map((l) => {
        const active = l.href === "/" ? pathname === "/" : pathname.startsWith(l.href);
        return (
          <Link
            key={l.href}
            href={l.href}
            className={`rounded-md px-3 py-1.5 text-sm font-medium ${
              active ? "bg-slate-900 text-white dark:bg-slate-100 dark:text-slate-900" : "text-slate-600 hover:bg-slate-200 dark:text-slate-300 dark:hover:bg-slate-800"
            }`}
          >
            {l.label}
          </Link>
        );
      })}
    </nav>
  );
}
```

- [ ] **Step 5: Write `web/src/components/Header.tsx`**

```tsx
import { getStore, isDemo } from "@/lib/store";
import { ago, engineHealth } from "@/lib/time";
import { NavLinks } from "./NavLinks";

const DOT = { ok: "bg-emerald-500", stale: "bg-amber-500", down: "bg-rose-500" } as const;

export async function Header() {
  const status = await getStore().status();
  const health = engineHealth(status.engine?.lastSeen ?? null);

  return (
    <header className="border-b border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
      <div className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-3 px-4 py-3">
        <div className="flex items-center gap-4">
          <span className="font-semibold">AlgoTrader</span>
          <NavLinks />
        </div>
        <div className="flex flex-wrap items-center gap-3 text-sm">
          {isDemo() && <span className="rounded bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-800">Demo data</span>}
          <span className="flex items-center gap-1.5" title={status.engine ? `${status.engine.instruments} instruments` : undefined}>
            <span className={`h-2 w-2 rounded-full ${DOT[health]}`} />
            Engine {status.engine ? ago(status.engine.lastSeen) : "never seen"}
          </span>
          {status.tokenValidToday ? (
            <span className="text-emerald-700 dark:text-emerald-400">✓ Kite token valid</span>
          ) : (
            <a href="/api/kite/login" className="rounded-md bg-amber-500 px-3 py-1.5 font-medium text-white hover:bg-amber-600">
              Log in to Kite
            </a>
          )}
        </div>
      </div>
    </header>
  );
}
```

- [ ] **Step 6: Write `web/src/components/KiteFlash.tsx`**

```tsx
export function KiteFlash({ kite, reason }: { kite?: string; reason?: string }) {
  if (kite === "ok") {
    return <div className="rounded-md bg-emerald-100 px-4 py-2 text-sm text-emerald-900 dark:bg-emerald-950 dark:text-emerald-200">✓ Kite token saved. The engine picks it up within a minute.</div>;
  }
  if (kite === "error") {
    return <div className="rounded-md bg-rose-100 px-4 py-2 text-sm text-rose-900 dark:bg-rose-950 dark:text-rose-200">Kite login failed: {reason ?? "unknown error"}</div>;
  }
  return null;
}
```

- [ ] **Step 7: Write the route-group layout and dashboard; delete the placeholder**

Delete `web/src/app/page.tsx`.

`web/src/app/(site)/layout.tsx`:

```tsx
import { Header } from "@/components/Header";

// Every page reads live data; never prerender at build time.
export const dynamic = "force-dynamic";

export default function SiteLayout({ children }: { children: React.ReactNode }) {
  return (
    <>
      <Header />
      <main className="mx-auto max-w-6xl space-y-6 px-4 py-6">{children}</main>
    </>
  );
}
```

`web/src/app/(site)/page.tsx`:

```tsx
import { KiteFlash } from "@/components/KiteFlash";
import { LiveSignals } from "@/components/LiveSignals";
import { baseFilter, first } from "@/lib/filters";
import { getStore } from "@/lib/store";
import { ALL_CAP } from "@/lib/store/types";
import { todayIST } from "@/lib/time";

type SP = Promise<Record<string, string | string[] | undefined>>;

export default async function DashboardPage({ searchParams }: { searchParams: SP }) {
  const sp = await searchParams;
  const today = todayIST();
  const rows = await getStore().listAll(baseFilter({ from: today, to: today }), ALL_CAP);

  return (
    <>
      <KiteFlash kite={first(sp.kite)} reason={first(sp.reason)} />
      <h1 className="text-xl font-semibold">Today · {today}</h1>
      <LiveSignals date={today} initialRows={rows} />
    </>
  );
}
```

- [ ] **Step 8: Verify in the browser**

Run `npm run dev` with the Task 4 `.env.local` (no Supabase vars, so demo mode is on). Open http://localhost:3000, unlock with `test`, and check:
- The header shows "Demo data", a green engine dot ("just now"), and "Log in to Kite".
- Summary cards and today's table render. The table scrolls sideways at 375 px width.
- "Export today" downloads `signals_<today>.xlsx` with both sheets.

Then run: `npx tsc --noEmit && npm run lint && npm run build`

- [ ] **Step 9: Commit**

```bash
git add web/src/app web/src/components
git commit -m "feat(web): site header, status badges and live dashboard"
```

---

### Task 10: Strategies page

**Files:**
- Create: `web/src/app/(site)/strategies/page.tsx`, `web/src/components/StrategySidebar.tsx`, `web/src/components/DayNav.tsx`

**Interfaces:**
- Consumes: `withCatalog`, `summarize` (Task 5); `applyFilter` (Task 5); `shortSignalName` (Task 2); `addDays`, `isISODate`, `todayIST` (Task 3); `SignalsTable`, `ExportButton`.
- Produces: `/strategies?date=YYYY-MM-DD&strategy=…|signal=…`.

- [ ] **Step 1: Write `web/src/components/DayNav.tsx`**

```tsx
import { addDays } from "@/lib/time";

export function DayNav({ date, today, extra }: { date: string; today: string; extra: Record<string, string> }) {
  const href = (d: string) => `/strategies?${new URLSearchParams({ date: d, ...extra })}`;
  const next = addDays(date, 1);
  const btn = "rounded-md border border-slate-300 px-3 py-1.5 text-sm dark:border-slate-700";

  return (
    <div className="flex flex-wrap items-center gap-2">
      <a href={href(addDays(date, -1))} className={btn} aria-label="Previous day">◀</a>
      <form method="get" action="/strategies" className="flex items-center gap-2">
        {Object.entries(extra).map(([k, v]) => (
          <input key={k} type="hidden" name={k} value={v} />
        ))}
        <input type="date" name="date" defaultValue={date} max={today} className={`${btn} bg-transparent`} />
        <button className={btn}>Go</button>
      </form>
      {next <= today ? (
        <a href={href(next)} className={btn} aria-label="Next day">▶</a>
      ) : (
        <span className={`${btn} opacity-40`}>▶</span>
      )}
      {date !== today && (
        <a href={href(today)} className="text-sm text-slate-600 underline dark:text-slate-400">Today</a>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Write `web/src/components/StrategySidebar.tsx`**

```tsx
import { shortSignalName } from "@/lib/catalog";
import type { StrategyCount } from "@/lib/summary";

export function StrategySidebar({
  model,
  date,
  strategy,
  signal,
}: {
  model: StrategyCount[];
  date: string;
  strategy?: string;
  signal?: string;
}) {
  const href = (p: Record<string, string>) => `/strategies?${new URLSearchParams({ date, ...p })}`;
  const item = (active: boolean) =>
    `flex items-center justify-between gap-2 rounded-md px-2 py-1 text-sm ${
      active ? "bg-slate-900 text-white dark:bg-slate-100 dark:text-slate-900" : "hover:bg-slate-200 dark:hover:bg-slate-800"
    }`;
  const count = (n: number) => <span className={`tabular-nums ${n === 0 ? "opacity-40" : ""}`}>{n}</span>;

  return (
    <nav className="w-full shrink-0 space-y-4 md:w-72">
      <a href={href({})} className={item(!strategy && !signal)}>
        <span className="font-medium">All strategies</span>
        {count(model.reduce((n, s) => n + s.total, 0))}
      </a>
      {model.map((s) => (
        <div key={s.strategy} className="space-y-1">
          <a href={href({ strategy: s.strategy })} className={item(strategy === s.strategy && !signal)}>
            <span className="font-medium">{s.strategy}</span>
            {count(s.total)}
          </a>
          <ul className="ml-3 space-y-0.5 border-l border-slate-200 pl-2 dark:border-slate-800">
            {s.signals.map((g) => (
              <li key={g.signal}>
                <a href={href({ signal: g.signal })} className={item(signal === g.signal)}>
                  <span className="truncate">{shortSignalName(g.signal, s.strategy)}</span>
                  {count(g.total)}
                </a>
              </li>
            ))}
          </ul>
        </div>
      ))}
    </nav>
  );
}
```

- [ ] **Step 3: Write `web/src/app/(site)/strategies/page.tsx`**

```tsx
import { DayNav } from "@/components/DayNav";
import { ExportButton } from "@/components/ExportButton";
import { SignalsTable } from "@/components/SignalsTable";
import { StrategySidebar } from "@/components/StrategySidebar";
import { shortSignalName } from "@/lib/catalog";
import { applyFilter } from "@/lib/filter-rows";
import { baseFilter, first } from "@/lib/filters";
import { getStore } from "@/lib/store";
import { ALL_CAP } from "@/lib/store/types";
import { summarize, withCatalog } from "@/lib/summary";
import { isISODate, todayIST } from "@/lib/time";

type SP = Promise<Record<string, string | string[] | undefined>>;

export default async function StrategiesPage({ searchParams }: { searchParams: SP }) {
  const sp = await searchParams;
  const today = todayIST();
  const requested = first(sp.date);
  const date = isISODate(requested) && requested <= today ? requested : today;
  const signal = first(sp.signal) || undefined;
  const strategy = signal ? undefined : first(sp.strategy) || undefined;

  const dayRows = await getStore().listAll(baseFilter({ from: date, to: date }), ALL_CAP);
  const selection = baseFilter({
    from: date,
    to: date,
    strategies: strategy ? [strategy] : [],
    signals: signal ? [signal] : [],
  });
  const shown = applyFilter(dayRows, selection);

  const selected = dayRows.find((r) => r.signal === signal);
  const title = signal ? shortSignalName(signal, selected?.strategy ?? "") : (strategy ?? "All strategies");
  const extra: Record<string, string> = signal ? { signal } : strategy ? { strategy } : {};

  return (
    <div className="flex flex-col gap-6 md:flex-row">
      <StrategySidebar model={withCatalog(summarize(dayRows))} date={date} strategy={strategy} signal={signal} />
      <section className="min-w-0 flex-1 space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h1 className="text-xl font-semibold">{title}</h1>
            <p className="text-sm text-slate-500">
              {shown.length} signal{shown.length === 1 ? "" : "s"} on {date}
            </p>
          </div>
          <ExportButton filter={selection} />
        </div>
        <DayNav date={date} today={today} extra={extra} />
        <SignalsTable rows={shown} emptyText="No signals for this selection on this day." />
      </section>
    </div>
  );
}
```

- [ ] **Step 4: Verify in the browser**

On `/strategies`:
- The sidebar lists RSI (12 signals) and IchimokuKinkoHyo (8), with counts. Zero counts are dimmed.
- Clicking "RSI" filters to RSI signals. Clicking "Daily Strong Bullish" narrows to that signal.
- ◀ goes to the previous day and keeps the selection. ▶ is disabled on today.
- Export downloads `signals_<date>_RSI.xlsx` (or the signal's slug).

Then run: `npx tsc --noEmit && npm run lint`

- [ ] **Step 5: Commit**

```bash
git add web/src/app/\(site\)/strategies web/src/components/StrategySidebar.tsx web/src/components/DayNav.tsx
git commit -m "feat(web): strategies page with per-day signal drill-down"
```

---

### Task 11: History page

**Files:**
- Create: `web/src/app/(site)/history/page.tsx`, `web/src/components/HistoryFilters.tsx`, `web/src/components/Pager.tsx`

**Interfaces:**
- Consumes: `parseFilter`, `filterToParams` (Task 5); `STRATEGIES`, `shortSignalName` (Task 2); `getStore` (Task 6); `SignalsTable`, `ExportButton`.
- Produces: `/history?from&to&strategy*&signal*&direction&symbol&sort&order&page`.

- [ ] **Step 1: Write `web/src/components/HistoryFilters.tsx`**

```tsx
import { STRATEGIES, shortSignalName } from "@/lib/catalog";
import type { SignalFilter } from "@/lib/types";

const input = "rounded-md border border-slate-300 bg-transparent px-2 py-1.5 text-sm dark:border-slate-700";

export function HistoryFilters({ filter, today }: { filter: SignalFilter; today: string }) {
  return (
    <details open className="rounded-lg border border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
      <summary className="cursor-pointer select-none px-4 py-2 text-sm font-medium">Filters</summary>
      <form method="get" action="/history" className="space-y-4 border-t border-slate-200 p-4 dark:border-slate-800">
        <div className="flex flex-wrap gap-3">
          <label className="flex flex-col gap-1 text-xs text-slate-500">
            From
            <input type="date" name="from" defaultValue={filter.from} max={today} className={input} />
          </label>
          <label className="flex flex-col gap-1 text-xs text-slate-500">
            To
            <input type="date" name="to" defaultValue={filter.to} max={today} className={input} />
          </label>
          <label className="flex flex-col gap-1 text-xs text-slate-500">
            Direction
            <select name="direction" defaultValue={filter.direction ?? ""} className={input}>
              <option value="">Both</option>
              <option value="long">Long</option>
              <option value="short">Short</option>
            </select>
          </label>
          <label className="flex flex-col gap-1 text-xs text-slate-500">
            Symbol starts with
            <input type="text" name="symbol" defaultValue={filter.symbol} placeholder="e.g. TATA" className={`${input} uppercase`} />
          </label>
        </div>

        <div className="grid gap-4 sm:grid-cols-2">
          {Object.entries(STRATEGIES).map(([strategy, signals]) => (
            <fieldset key={strategy} className="space-y-1">
              <label className="flex items-center gap-2 text-sm font-medium">
                <input type="checkbox" name="strategy" value={strategy} defaultChecked={filter.strategies.includes(strategy)} />
                {strategy} (all)
              </label>
              <div className="ml-6 grid gap-0.5">
                {signals.map((signal) => (
                  <label key={signal} className="flex items-center gap-2 text-sm">
                    <input type="checkbox" name="signal" value={signal} defaultChecked={filter.signals.includes(signal)} />
                    {shortSignalName(signal, strategy)}
                  </label>
                ))}
              </div>
            </fieldset>
          ))}
        </div>

        <input type="hidden" name="sort" value={filter.sort} />
        <input type="hidden" name="order" value={filter.order} />
        <div className="flex gap-2">
          <button className="rounded-md bg-slate-900 px-4 py-1.5 text-sm font-medium text-white dark:bg-slate-100 dark:text-slate-900">Apply</button>
          <a href="/history" className="rounded-md border border-slate-300 px-4 py-1.5 text-sm dark:border-slate-700">Reset</a>
        </div>
      </form>
    </details>
  );
}
```

- [ ] **Step 2: Write `web/src/components/Pager.tsx`**

```tsx
export function Pager({ page, pages, href }: { page: number; pages: number; href: (page: number) => string }) {
  if (pages <= 1) return null;
  const btn = "rounded-md border border-slate-300 px-3 py-1.5 text-sm dark:border-slate-700";
  return (
    <div className="flex items-center justify-center gap-3">
      {page > 1 ? <a href={href(page - 1)} className={btn}>← Newer</a> : <span className={`${btn} opacity-40`}>← Newer</span>}
      <span className="text-sm tabular-nums text-slate-500">
        Page {page} of {pages}
      </span>
      {page < pages ? <a href={href(page + 1)} className={btn}>Older →</a> : <span className={`${btn} opacity-40`}>Older →</span>}
    </div>
  );
}
```

"Newer" and "Older" match the default newest-first sort. With other sorts they
still step through the pages in order.

- [ ] **Step 3: Write `web/src/app/(site)/history/page.tsx`**

```tsx
import { ExportButton } from "@/components/ExportButton";
import { HistoryFilters } from "@/components/HistoryFilters";
import { Pager } from "@/components/Pager";
import { SignalsTable } from "@/components/SignalsTable";
import { filterToParams, first, parseFilter } from "@/lib/filters";
import { getStore } from "@/lib/store";
import { addDays, todayIST } from "@/lib/time";
import type { SignalFilter, SortKey } from "@/lib/types";

type SP = Promise<Record<string, string | string[] | undefined>>;
const PAGE_SIZE = 100;

export default async function HistoryPage({ searchParams }: { searchParams: SP }) {
  const sp = await searchParams;
  const today = todayIST();
  const filter = parseFilter(sp, { from: addDays(today, -6), to: today });
  const page = Math.max(1, Number.parseInt(first(sp.page) ?? "1", 10) || 1);

  const { rows, total } = await getStore().list(filter, (page - 1) * PAGE_SIZE, PAGE_SIZE);
  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  const withParams = (f: SignalFilter, p?: number) => {
    const params = filterToParams(f);
    if (p && p > 1) params.set("page", String(p));
    return `/history?${params}`;
  };
  const sortLinks = Object.fromEntries(
    (["time", "symbol", "signal"] as SortKey[]).map((key) => [
      key,
      withParams({ ...filter, sort: key, order: filter.sort === key && filter.order === "desc" ? "asc" : "desc" }),
    ]),
  );

  return (
    <>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold">History</h1>
          <p className="text-sm text-slate-500">
            {total.toLocaleString("en-US")} signal{total === 1 ? "" : "s"} · {filter.from ?? "start"} → {filter.to ?? "today"}
          </p>
        </div>
        <ExportButton filter={filter} />
      </div>
      <HistoryFilters filter={filter} today={today} />
      <SignalsTable
        rows={rows}
        showDate
        sortLinks={sortLinks}
        activeSort={filter.sort}
        order={filter.order}
        emptyText="No signals match these filters."
      />
      <Pager page={page} pages={pages} href={(p) => withParams(filter, p)} />
    </>
  );
}
```

`Pager` receives a function prop. That's valid only because `Pager` is a
server component, with no `"use client"`. Keep it that way.

- [ ] **Step 4: Verify in the browser**

On `/history`:
- The default range is the last 7 days, and the total count shows.
- Selecting "RSI (all)" plus one Ichimoku signal, then Apply, shows the union.
- Direction = Short shows only shorts. Symbol "TA" shows TATA* names.
- Clicking the "Symbol" header sorts A→Z; clicking again sorts Z→A.
- Paging works and keeps the filters.
- Export downloads every matching row (not just the page), named `signals_<from>_to_<to>….xlsx`.

Then run: `npx tsc --noEmit && npm run lint && npm run build`

- [ ] **Step 5: Commit**

```bash
git add web/src/app/\(site\)/history web/src/components/HistoryFilters.tsx web/src/components/Pager.tsx
git commit -m "feat(web): history page with filters, sorting, paging and export"
```

---

### Task 12: Setup docs and final verification

**Files:**
- Create: `web/README.md` (replace the create-next-app one), `docs/SETUP.md`
- Modify: `CLAUDE.md` (repo root)

- [ ] **Step 1: Write `web/README.md`**

````markdown
# AlgoTrader Signals — website

Next.js 16 site for viewing and exporting the engine's signals. Deployed on
Vercel with root directory `web/`.

## Local development

```bash
cd web
npm install
cp .env.example .env.local   # set SITE_PASSCODE and COOKIE_SECRET at least
npm run dev                  # http://localhost:3000
```

With `SUPABASE_URL` and `SUPABASE_SERVICE_KEY` empty, the site uses built-in
demo data (a "Demo data" badge shows in the header). Set both to read your
real Supabase project. In production they are required.

## Commands

| Command | What it does |
|---|---|
| `npm run dev` | Dev server |
| `npm test` | Unit tests (vitest) |
| `npx vitest run src/lib/kite.test.ts` | One test file |
| `npx tsc --noEmit` | Typecheck |
| `npm run lint` | ESLint |
| `npm run build` | Production build |

## How it fits together

- `src/proxy.ts` gates every route behind the per-device passcode cookie.
- Pages are server components that read through `getStore()`
  (`src/lib/store`). The browser never talks to Supabase.
- `/api/kite/login` → Zerodha → `/api/kite/callback`, which exchanges the
  request token and saves it to `kite_session` for the engine to pick up.
- Excel files are built in the browser (`src/lib/excel.ts`) from paged
  `/api/signals` calls.
````

- [ ] **Step 2: Write `docs/SETUP.md`**

````markdown
# Setup guide

You do these steps once. The engine server (Oracle Cloud) section is added
with the engine update.

## 1. GitHub

1. Create a **private** repository on GitHub.
2. From the repo folder:
   ```bash
   git remote add origin https://github.com/<you>/<repo>.git
   git push -u origin main
   ```
   `.gitignore` already keeps out `.env`, the database, binaries and logs.

## 2. Supabase

1. Create a free project at https://supabase.com (region: Mumbai, `ap-south-1`).
2. **SQL Editor → New query**, paste `supabase/migrations/001_init.sql`, then Run.
3. **Project Settings → API**: copy the **Project URL** and the
   **service_role** key. The service_role key is a secret: it goes only in
   Vercel and the engine's `.env`, never in browser code or git.
4. **Table Editor**: you should see `signals`, `kite_session` and
   `engine_status`, each marked "RLS enabled".

## 3. Vercel

1. **Add New → Project**, import the GitHub repo.
2. **Root Directory**: `web`. Framework: Next.js (auto-detected).
3. **Environment Variables** (Production):

   | Name | Value |
   |---|---|
   | `SUPABASE_URL` | Project URL from step 2 |
   | `SUPABASE_SERVICE_KEY` | service_role key from step 2 |
   | `SITE_PASSCODE` | The passcode you'll type once per device |
   | `COOKIE_SECRET` | A long random string, e.g. the output of `openssl rand -base64 32` |
   | `KITE_API_KEY` | From developers.kite.trade |
   | `KITE_API_SECRET` | From developers.kite.trade |
   | `KITE_USER_ID` | Your Zerodha client ID, e.g. `AB1234` |

4. Deploy. Note your URL, e.g. `https://algotrader-xyz.vercel.app`.
5. Every push to `main` redeploys automatically.

## 4. Kite developer console

1. https://developers.kite.trade → your app → **Redirect URL**:
   `https://<your-site>.vercel.app/api/kite/callback`
2. Save.

## 5. Check it works

1. Open the site and enter the passcode. You land on the dashboard, with no
   "Demo data" badge.
2. Click **Log in to Kite**, log in on Zerodha, and you return with
   "✓ Kite token saved". The header now shows "✓ Kite token valid".
3. In Supabase, `kite_session` has one row (`created_at` is now).
4. The engine status stays "never seen" until the engine update is deployed.

## Daily routine

Each trading morning: open the site on any device → **Log in to Kite** →
Zerodha password + TOTP. That's it; one login covers every device and the
engine.
````

- [ ] **Step 3: Add a web section to the root `CLAUDE.md`**

Append:

```markdown
## Website (`web/`)

Next.js 16 on Vercel, which reads Supabase server-side only. See `web/README.md`.

- `cd web && npm test` (vitest), `npx tsc --noEmit`, `npm run lint`, `npm run build`. A single file: `npx vitest run src/lib/kite.test.ts`.
- Next 16: gating lives in `src/proxy.ts` (not `middleware.ts`), page `searchParams` is a Promise, and `cookies()` is async. Check `web/node_modules/next/dist/docs/` before using other Next APIs.
- Without `SUPABASE_URL`/`SUPABASE_SERVICE_KEY`, `npm run dev` serves demo data (`src/lib/store/demo.ts`).
- Signal names in `src/lib/catalog.ts` mirror the Go engine's. Update both together, and keep `directionOf` in step with `backtest.InferDirection`.
- The Kite `access_token` must never reach the browser. `SupabaseStore.status()` deliberately selects only `created_at`.
- Schema: `supabase/migrations/001_init.sql`. Setup steps: `docs/SETUP.md`.
```

- [ ] **Step 4: Full verification**

Run (in `web/`): `npm test && npx tsc --noEmit && npm run lint && npm run build`
Expected: all tests pass, no type or lint errors, and the build lists `/`,
`/strategies`, `/history` and `/unlock` plus the API routes, with the
`(site)` pages marked dynamic (ƒ).

Then walk through every page once more in `npm run dev` (demo mode), including
a phone-width viewport.

- [ ] **Step 5: Commit**

```bash
git add web/README.md docs/SETUP.md CLAUDE.md
git commit -m "docs: website README, setup guide and CLAUDE.md web section"
```

---

## Self-review notes

- **Spec coverage:** dashboard (T9), strategies/day view (T10), history
  (T11), Excel with summary sheet (T8), passcode per device (T4), Kite
  button, callback and user-ID check (T7), status header with engine health
  (T9), schema and RLS (T2), token never exposed (T6 `status()`), 30 s refresh
  (T9), export cap and paging (T6/T8), SETUP for GitHub/Supabase/Vercel/Kite
  (T12). Engine-side items are deferred to the engine plan by design.
- **Consistency:** counts in error messages use `toLocaleString("en-US")`
  ("150,000") in both `export-fetch.ts` and `TooManyRowsError`.
  `FilterableQuery` is non-generic, so the fake in `query.test.ts` and the
  cast in `supabase.ts` agree.
