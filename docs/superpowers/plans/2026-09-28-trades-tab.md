# Trades Tab Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Record buys and sells: Buy/Sell buttons on every signal row, and a new Trades tab listing every recorded trade, where one can also be added by searching for a stock.

**Architecture:** A new Supabase table `trades` (migration `002_trades.sql`) is read and written only by the Next.js server: pages read through the existing `getStore()` store, and two new API routes (`POST /api/trades`, `DELETE /api/trades/[id]`) write. Pure helpers in `src/lib/trades.ts` hold every validation rule so they are unit-tested; the UI is two small client components (`TradeButtons`, `AddTradeForm`) plus a server-rendered Trades page. No engine change.

**Tech Stack:** Next.js 16 (App Router, `src/proxy.ts` gate), React 19, TypeScript, Tailwind 4, `@supabase/supabase-js` (PostgREST), vitest 5 (node environment). Go only for the one-off stock-list generator.

**Spec:** `docs/superpowers/specs/2026-09-28-trades-tab-design.md`

## Global Constraints

- **Never commit or push.** Leave every change in the working tree; the user commits and pushes themselves.
- Symbol rule everywhere (SQL check, validation, bundled list): `^[A-Z0-9&-]{1,30}$`, uppercase.
- Price: absent/null, or a finite number `> 0` and `< 100000000` (₹10 crore), rounded to 2 decimals.
- Trade date: `YYYY-MM-DD`, IST calendar, never after today (IST); defaults to today (IST).
- A trade recorded from a signal copies that signal's `symbol`, `price`, `trade_date` and name (`signal`); the browser only sends `{ signalId, side }`.
- `unique (signal_id, side)`: at most one buy and one sell per signal; a repeat click returns the existing trade (HTTP 200, not an error).
- Trades list: newest first (`trade_date desc, id desc`), `TRADES_CAP = 1000`. Marks lookups: `MARKS_BATCH = 200` signal ids per request.
- UI copy: buttons **Buy** / **Sell**, recorded state **Bought ✓** / **Sold ✓**, nav label **Trades**, manual source **Manual**.
- The browser never talks to Supabase; the service key and the Kite `access_token` never reach it. New write routes reject a cross-site `Origin` with 403.
- Next 16: page `searchParams` and route `params` are Promises; the gate is `src/proxy.ts` (unchanged: it already covers `/trades` and `/api/trades*`).
- Web checks: `cd web && npm test`, `npx tsc --noEmit`, `npm run lint`, `npm run build`. Go on Windows needs `export PATH="/c/msys64/mingw64/bin:$PATH"` first.

## Review Focus

1. **Double-clicking Buy** records exactly one trade and ends on **Bought ✓**. Pinned by the repeat-POST route test (Task 3) and a double-click browser check (Task 5).
2. **Symbols with `&` or `-`** (M&M, BAJAJ-AUTO) survive the add form, the filter box and query strings. Pinned by `normalizeSymbol`/`parseTradeFilter` tests (Task 1), the `like` query test (Task 2) and a browser check adding and filtering M&M (Task 6).
3. **`002_trades.sql` not run yet**: Dashboard, Strategies and History still load; a Buy click shows Supabase's error; the Trades page shows the error page naming `002_trades.sql`. Pinned by `tradeMarksOrEmpty` tests (Task 2), the store-failure route test (Task 3) and a `TRADES_MISSING=1` browser check (Task 7).
4. **Signal deleted after the page loaded** (e.g. the out-of-hours cleanup SQL): Buy shows "That signal no longer exists." Pinned by the 404 route test (Task 3).
5. **Just after midnight IST** (still the previous day in UTC): a manual trade dated today (IST) is accepted and tomorrow is refused. Pinned by `parseTradeInput` date tests (Task 1) and a fake-timer route test (Task 3).

---

## File Structure

| File | Responsibility |
|---|---|
| `supabase/migrations/002_trades.sql` (new) | The `trades` table, indexes, RLS |
| `web/src/lib/types.ts` | + `Side`, `Trade`, `TradeFilter`, `TradeMarks`, `TradeInput`, `NewTrade` |
| `web/src/lib/trades.ts` (new) | Pure rules: symbol, input and filter parsing, marks shaping, batching, labels |
| `web/src/lib/origin.ts` (new) | Same-origin check for write routes |
| `web/src/lib/store/trade-query.ts` (new) | Trade row mapping and query ordering/filtering |
| `web/src/lib/store/marks.ts` (new) | `tradeMarksOrEmpty`: marks that never break a page |
| `web/src/lib/store/types.ts`, `supabase.ts` | Store interface and Supabase implementation of the trade methods |
| `web/src/app/api/trades/route.ts` (new) | `POST` create |
| `web/src/app/api/trades/[id]/route.ts` (new) | `DELETE` remove |
| `web/src/components/TradeButtons.tsx` (new) | Buy/Sell toggle on a signal row |
| `web/src/components/SignalsTable.tsx`, `LiveSignals.tsx`, three signal pages | Show the buttons with their marks |
| `scripts/gen_web_symbols.go` (new), `web/src/lib/nse-symbols.json` (generated) | Stock suggestions from NSE's `EQUITY_L.csv` |
| `web/src/components/AddTradeForm.tsx`, `TradesTable.tsx`, `RemoveTradeButton.tsx`, `web/src/app/(site)/trades/page.tsx` (new) | The Trades tab |
| `web/src/components/NavLinks.tsx`, `web/src/app/error.tsx` | Nav entry; missing-table hint |
| `docs/SETUP.md`, `web/README.md`, `CLAUDE.md` | Migration step and code map |

---

### Task 1: Trades table, shared types and input rules

**Files:**
- Create: `supabase/migrations/002_trades.sql`
- Modify: `web/src/lib/types.ts` (append)
- Create: `web/src/lib/trades.ts`, `web/src/lib/trades.test.ts`
- Create: `web/src/lib/origin.ts`, `web/src/lib/origin.test.ts`
- Modify: `docs/SETUP.md` (§2.2 and "Updating the site later"), `web/README.md` ("How it fits together")

**Interfaces:**
- Consumes: `first`, `SearchParamsLike` from `web/src/lib/filters.ts`; `isISODate` from `web/src/lib/time.ts`; `shortSignalName` from `web/src/lib/catalog.ts`; `Signal` from `web/src/lib/types.ts`.
- Produces (used by Tasks 2–6):
  - types `Side`, `Trade`, `TradeFilter`, `TradeMarks`, `TradeInput`, `NewTrade` (exact shapes in Step 3);
  - `SYMBOL_RE: RegExp`, `MAX_PRICE = 100_000_000`;
  - `normalizeSymbol(s: unknown): string | null`;
  - `parseTradeInput(body: unknown, today: string): { ok: true; value: TradeInput } | { ok: false; error: string }`;
  - `parseTradeFilter(p: SearchParamsLike): TradeFilter`;
  - `tradeMarksFrom(rows: { id: number; signalId: number | null; side: Side }[]): TradeMarks`;
  - `chunk<T>(items: T[], size: number): T[][]`;
  - `signalSource(signal: string | null): { family: string; name: string } | null`;
  - `tradeFromSignal(signal: Signal, side: Side): NewTrade`;
  - `manualTrade(input: Extract<TradeInput, { kind: "manual" }>): NewTrade`;
  - `isSameOrigin(origin: string | null, host: string | null): boolean` (in `origin.ts`).

- [ ] **Step 1: Write the migration**

Create `supabase/migrations/002_trades.sql`:

```sql
-- Run once in the Supabase SQL editor, after 001_init.sql. Adds the Trades tab's table.
create table if not exists trades (
  id          bigint generated always as identity primary key,
  created_at  timestamptz   not null default now(),
  trade_date  date          not null,                  -- IST day of the trade
  symbol      text          not null check (symbol ~ '^[A-Z0-9&-]{1,30}$'),
  side        text          not null check (side in ('buy', 'sell')),
  price       numeric(12,2) check (price > 0),         -- optional on manual trades
  signal_id   bigint        references signals(id) on delete set null,
  signal      text,                                    -- signal name when recorded; null = manual
  unique (signal_id, side)                             -- one buy and one sell per signal
);
create index if not exists trades_date_idx   on trades (trade_date desc, id desc);
create index if not exists trades_signal_idx on trades (signal_id);

-- RLS on with no policies: only the service key (the website server) can read or write.
alter table trades enable row level security;
```

- [ ] **Step 2: Write the failing tests**

Create `web/src/lib/trades.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import {
  MAX_PRICE,
  chunk,
  manualTrade,
  normalizeSymbol,
  parseTradeFilter,
  parseTradeInput,
  signalSource,
  tradeFromSignal,
  tradeMarksFrom,
} from "./trades";
import type { Signal } from "./types";

const TODAY = "2026-09-28";

describe("normalizeSymbol", () => {
  it.each([
    [" reliance ", "RELIANCE"],
    ["m&m", "M&M"],
    ["BAJAJ-AUTO", "BAJAJ-AUTO"],
    ["20MICRONS", "20MICRONS"],
  ])("%s → %s", (raw, want) => {
    expect(normalizeSymbol(raw)).toBe(want);
  });
  it.each(["", "   ", "RELIANCE INDUSTRIES", "TCS;DROP", "A".repeat(31), 42, null, undefined])("rejects %j", (raw) => {
    expect(normalizeSymbol(raw)).toBeNull();
  });
});

describe("parseTradeInput", () => {
  it("reads a signal trade and ignores every other field", () => {
    expect(parseTradeInput({ signalId: 17, side: "buy", symbol: "HACK", price: 1 }, TODAY)).toEqual({
      ok: true,
      value: { kind: "signal", signalId: 17, side: "buy" },
    });
  });
  it.each([0, -3, 1.5, "17", null])("rejects signalId %j", (signalId) => {
    expect(parseTradeInput({ signalId, side: "sell" }, TODAY)).toMatchObject({ ok: false });
  });
  it("reads a manual trade, defaulting price to none and date to today", () => {
    expect(parseTradeInput({ symbol: "m&m", side: "sell" }, TODAY)).toEqual({
      ok: true,
      value: { kind: "manual", symbol: "M&M", side: "sell", price: null, tradeDate: TODAY },
    });
  });
  it("rounds the price to paise and keeps a past date", () => {
    expect(parseTradeInput({ symbol: "TCS", side: "buy", price: 3521.456, tradeDate: "2026-09-01" }, TODAY)).toEqual({
      ok: true,
      value: { kind: "manual", symbol: "TCS", side: "buy", price: 3521.46, tradeDate: "2026-09-01" },
    });
  });
  it("treats an empty price or date as not given", () => {
    expect(parseTradeInput({ symbol: "TCS", side: "buy", price: null, tradeDate: "" }, TODAY)).toMatchObject({
      ok: true,
      value: { price: null, tradeDate: TODAY },
    });
  });
  it("accepts today and refuses tomorrow", () => {
    expect(parseTradeInput({ symbol: "TCS", side: "buy", tradeDate: TODAY }, TODAY)).toMatchObject({ ok: true });
    expect(parseTradeInput({ symbol: "TCS", side: "buy", tradeDate: "2026-09-29" }, TODAY)).toMatchObject({ ok: false });
  });
  it.each([
    [{ side: "buy" }, "symbol"],
    [{ symbol: "TCS" }, "side"],
    [{ symbol: "TCS", side: "hold" }, "side"],
    [{ symbol: "TCS", side: "buy", price: 0 }, "Price"],
    [{ symbol: "TCS", side: "buy", price: -5 }, "Price"],
    [{ symbol: "TCS", side: "buy", price: MAX_PRICE }, "Price"],
    [{ symbol: "TCS", side: "buy", price: "100" }, "Price"],
    [{ symbol: "TCS", side: "buy", tradeDate: "2026-09-29" }, "future"],
    [{ symbol: "TCS", side: "buy", tradeDate: "28-09-2026" }, "YYYY-MM-DD"],
    [{ symbol: "TCS", side: "buy", tradeDate: "2026-02-30" }, "YYYY-MM-DD"],
  ])("rejects %j with a message about %s", (body, fragment) => {
    const r = parseTradeInput(body, TODAY);
    expect(r.ok).toBe(false);
    if (!r.ok) expect(r.error).toContain(fragment);
  });
  it.each([null, "text", 42, []])("rejects the non-object body %j", (body) => {
    expect(parseTradeInput(body, TODAY)).toMatchObject({ ok: false });
  });
});

describe("parseTradeFilter", () => {
  it("reads the side and a cleaned symbol prefix", () => {
    expect(parseTradeFilter({ side: "sell", symbol: " m&m " })).toEqual({ side: "sell", symbol: "M&M" });
  });
  it("drops unknown sides and empty symbols", () => {
    expect(parseTradeFilter({ side: "hold", symbol: "" })).toEqual({ side: undefined, symbol: undefined });
    expect(parseTradeFilter(new URLSearchParams("side=buy"))).toEqual({ side: "buy", symbol: undefined });
  });
});

describe("tradeMarksFrom", () => {
  it("maps each signal to its buy and sell trade ids, skipping manual trades", () => {
    expect(
      tradeMarksFrom([
        { id: 1, signalId: 10, side: "buy" },
        { id: 2, signalId: 10, side: "sell" },
        { id: 3, signalId: 11, side: "sell" },
        { id: 4, signalId: null, side: "buy" },
      ]),
    ).toEqual({ 10: { buy: 1, sell: 2 }, 11: { sell: 3 } });
  });
});

describe("chunk", () => {
  it("splits into batches and keeps the remainder", () => {
    expect(chunk([1, 2, 3, 4, 5], 2)).toEqual([[1, 2], [3, 4], [5]]);
    expect(chunk([], 200)).toEqual([]);
  });
});

describe("signalSource", () => {
  it("names the signal a trade came from", () => {
    expect(signalSource("IchimokuKinkoHyo (Future BUY)")).toEqual({ family: "IchimokuKinkoHyo", name: "Future BUY" });
    expect(signalSource("IchimokuKinkoHyo (BUY) 1A")).toEqual({ family: "IchimokuKinkoHyo", name: "(BUY) 1A" });
    expect(signalSource("RSI (Daily Strong Bullish)")).toEqual({ family: "RSI", name: "Daily Strong Bullish" });
    expect(signalSource(null)).toBeNull();
  });
});

describe("tradeFromSignal / manualTrade", () => {
  it("copies the signal's stock, price, day and name", () => {
    const signal: Signal = {
      id: 9,
      firedAt: "2026-09-28T04:00:00Z",
      tradeDate: "2026-09-28",
      token: 258817,
      symbol: "SCHAEFFLER",
      strategy: "IchimokuKinkoHyo",
      signal: "IchimokuKinkoHyo (Future BUY)",
      direction: "long",
      price: 4217.6,
      rsiDaily: 67.4,
      rsiWeekly: 56.3,
      rsiMonthly: 55.8,
    };
    expect(tradeFromSignal(signal, "buy")).toEqual({
      tradeDate: "2026-09-28",
      symbol: "SCHAEFFLER",
      side: "buy",
      price: 4217.6,
      signalId: 9,
      signal: "IchimokuKinkoHyo (Future BUY)",
    });
  });
  it("marks a manual trade as having no signal", () => {
    expect(manualTrade({ kind: "manual", symbol: "M&M", side: "sell", price: null, tradeDate: TODAY })).toEqual({
      tradeDate: TODAY,
      symbol: "M&M",
      side: "sell",
      price: null,
      signalId: null,
      signal: null,
    });
  });
});
```

Create `web/src/lib/origin.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { isSameOrigin } from "./origin";

describe("isSameOrigin", () => {
  it("accepts the site's own origin", () => {
    expect(isSameOrigin("https://web-one-nu-38.vercel.app", "web-one-nu-38.vercel.app")).toBe(true);
    expect(isSameOrigin("http://localhost:3000", "localhost:3000")).toBe(true);
  });
  it("refuses other sites, other ports and opaque origins", () => {
    expect(isSameOrigin("https://evil.example", "web-one-nu-38.vercel.app")).toBe(false);
    expect(isSameOrigin("http://localhost:4000", "localhost:3000")).toBe(false);
    expect(isSameOrigin("null", "localhost:3000")).toBe(false);
    expect(isSameOrigin("https://evil.example", null)).toBe(false);
  });
  it("leaves a request without an Origin header to the cookie check", () => {
    expect(isSameOrigin(null, "localhost:3000")).toBe(true);
  });
});
```

- [ ] **Step 3: Run the tests to see them fail**

Run: `cd web && npx vitest run src/lib/trades.test.ts src/lib/origin.test.ts`
Expected: FAIL, `Failed to resolve import "./trades"` / `"./origin"`.

- [ ] **Step 4: Add the types**

Append to `web/src/lib/types.ts`:

```ts
export type Side = "buy" | "sell";

export interface Trade {
  id: number;
  createdAt: string; // ISO timestamp when it was recorded
  tradeDate: string; // YYYY-MM-DD, IST
  symbol: string;
  side: Side;
  price: number | null;
  signalId: number | null; // the signal it was recorded from; null = manual, or that signal was deleted
  signal: string | null; // that signal's name, e.g. "IchimokuKinkoHyo (Future BUY)"; null = manual
}

export interface TradeFilter {
  side?: Side;
  symbol?: string; // uppercase prefix
}

/** For each signal on screen, the ids of its recorded buy and/or sell trade. */
export type TradeMarks = Record<number, { buy?: number; sell?: number }>;

/** What a POST /api/trades body asks for, once validated. */
export type TradeInput =
  | { kind: "signal"; signalId: number; side: Side }
  | { kind: "manual"; symbol: string; side: Side; price: number | null; tradeDate: string };

/** A row to insert into the trades table. */
export interface NewTrade {
  tradeDate: string;
  symbol: string;
  side: Side;
  price: number | null;
  signalId: number | null;
  signal: string | null;
}
```

- [ ] **Step 5: Implement the rules**

Create `web/src/lib/trades.ts`:

```ts
import { shortSignalName } from "./catalog";
import { first, type SearchParamsLike } from "./filters";
import { isISODate } from "./time";
import type { NewTrade, Side, Signal, TradeFilter, TradeInput, TradeMarks } from "./types";

/** NSE trading symbols: capitals, digits, "&" and "-" (e.g. M&M, BAJAJ-AUTO). Same rule as the SQL check. */
export const SYMBOL_RE = /^[A-Z0-9&-]{1,30}$/;

/** ₹10 crore. The column holds more, but a price this large is a typo. */
export const MAX_PRICE = 100_000_000;

export function normalizeSymbol(s: unknown): string | null {
  if (typeof s !== "string") return null;
  const v = s.trim().toUpperCase();
  return SYMBOL_RE.test(v) ? v : null;
}

function isSide(v: unknown): v is Side {
  return v === "buy" || v === "sell";
}

const blank = (v: unknown) => v === undefined || v === null || v === "";

/** Validates a POST /api/trades body. `today` is the IST date, YYYY-MM-DD. */
export function parseTradeInput(
  body: unknown,
  today: string,
): { ok: true; value: TradeInput } | { ok: false; error: string } {
  if (typeof body !== "object" || body === null) return { ok: false, error: "Send the trade as a JSON object." };
  const b = body as Record<string, unknown>;
  if (!isSide(b.side)) return { ok: false, error: 'The side must be "buy" or "sell".' };

  if (b.signalId !== undefined) {
    const id = b.signalId;
    if (typeof id !== "number" || !Number.isSafeInteger(id) || id <= 0) {
      return { ok: false, error: "signalId must be a positive whole number." };
    }
    return { ok: true, value: { kind: "signal", signalId: id, side: b.side } };
  }

  const symbol = normalizeSymbol(b.symbol);
  if (!symbol) return { ok: false, error: "Enter a valid NSE symbol, e.g. RELIANCE or M&M." };

  let price: number | null = null;
  if (!blank(b.price)) {
    const p = b.price;
    if (typeof p !== "number" || !Number.isFinite(p) || p <= 0 || p >= MAX_PRICE) {
      return { ok: false, error: "Price must be a number above 0 and below ₹10 crore." };
    }
    price = Math.round(p * 100) / 100;
  }

  let tradeDate = today;
  if (!blank(b.tradeDate)) {
    if (typeof b.tradeDate !== "string" || !isISODate(b.tradeDate)) {
      return { ok: false, error: "The date must be a YYYY-MM-DD date." };
    }
    if (b.tradeDate > today) return { ok: false, error: "The date can't be in the future." };
    tradeDate = b.tradeDate;
  }
  return { ok: true, value: { kind: "manual", symbol, side: b.side, price, tradeDate } };
}

/** The Trades page's `side` and `symbol` query parameters; symbol is an uppercase prefix. */
export function parseTradeFilter(p: SearchParamsLike): TradeFilter {
  const get = (k: string) => (p instanceof URLSearchParams ? (p.get(k) ?? undefined) : first(p[k]));
  const side = get("side");
  const symbol = (get("symbol") ?? "").toUpperCase().replace(/[^A-Z0-9&-]/g, "");
  return { side: isSide(side) ? side : undefined, symbol: symbol || undefined };
}

export function tradeMarksFrom(rows: { id: number; signalId: number | null; side: Side }[]): TradeMarks {
  const marks: TradeMarks = {};
  for (const r of rows) {
    if (r.signalId === null) continue;
    (marks[r.signalId] ??= {})[r.side] = r.id;
  }
  return marks;
}

export function chunk<T>(items: T[], size: number): T[][] {
  const out: T[][] = [];
  for (let i = 0; i < items.length; i += size) out.push(items.slice(i, i + size));
  return out;
}

/** "IchimokuKinkoHyo (Future BUY)" → { family: "IchimokuKinkoHyo", name: "Future BUY" }. */
export function signalSource(signal: string | null): { family: string; name: string } | null {
  if (!signal) return null;
  const family = signal.split(" ")[0];
  return { family, name: shortSignalName(signal, family) };
}

/** A trade recorded from a signal takes the signal's stock, price, day and name. */
export function tradeFromSignal(signal: Signal, side: Side): NewTrade {
  return {
    tradeDate: signal.tradeDate,
    symbol: signal.symbol,
    side,
    price: signal.price,
    signalId: signal.id,
    signal: signal.signal,
  };
}

export function manualTrade(input: Extract<TradeInput, { kind: "manual" }>): NewTrade {
  return {
    tradeDate: input.tradeDate,
    symbol: input.symbol,
    side: input.side,
    price: input.price,
    signalId: null,
    signal: null,
  };
}
```

Create `web/src/lib/origin.ts`:

```ts
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
```

- [ ] **Step 6: Run the tests to see them pass**

Run: `cd web && npx vitest run src/lib/trades.test.ts src/lib/origin.test.ts`
Expected: PASS, all tests.

- [ ] **Step 7: Document the migration**

In `docs/SETUP.md` §2.2, replace steps 3–4:

```markdown
3. Click **Run**. You should see "Success. No rows returned".
4. Do the same with `supabase/migrations/002_trades.sql` (the Trades tab).
5. Left sidebar → **Table Editor**. You should see `signals`, `kite_session`,
   `engine_status` and `trades`, each without a red "RLS disabled" warning.
```

In `docs/SETUP.md` "Updating the site later", after the paragraph ending "under your project → **Deployments**.", add:

```markdown
If an update adds a file to `supabase/migrations/` (for example
`002_trades.sql` for the Trades tab), run it once in Supabase → **SQL Editor**
before pushing. The site shows "Couldn't load signals" on the pages that need
the new table until you do.
```

In `web/README.md` "How it fits together", add a bullet:

```markdown
- Trades (the `/trades` tab and the Buy/Sell buttons on signal rows) live in
  the `trades` table (`supabase/migrations/002_trades.sql`). Pages read them
  through the store; `POST /api/trades` and `DELETE /api/trades/[id]` write.
```

- [ ] **Step 8: Typecheck**

Run: `cd web && npx tsc --noEmit`
Expected: no output. Do not commit.

---

### Task 2: Store support for trades

**Files:**
- Create: `web/src/lib/store/trade-query.ts`, `web/src/lib/store/trade-query.test.ts`
- Create: `web/src/lib/store/marks.ts`, `web/src/lib/store/marks.test.ts`
- Modify: `web/src/lib/store/types.ts`, `web/src/lib/store/supabase.ts`

**Interfaces:**
- Consumes: `FilterableQuery`, `SIGNAL_COLUMNS`, `SignalRow`, `toSignal` from `web/src/lib/store/query.ts`; `chunk`, `tradeMarksFrom` (Task 1); types from Task 1.
- Produces:
  - `TRADES_CAP = 1_000`, `MARKS_BATCH = 200` in `store/types.ts`;
  - `SignalStore` methods: `getSignal(id: number): Promise<Signal | null>`, `addTrade(t: NewTrade): Promise<{ trade: Trade; created: boolean }>`, `deleteTrade(id: number): Promise<void>`, `listTrades(f: TradeFilter, cap: number): Promise<{ rows: Trade[]; total: number }>`, `tradeMarks(signalIds: number[]): Promise<TradeMarks>`;
  - `TRADE_COLUMNS`, `TradeRow`, `toTrade(r: TradeRow): Trade`, `applyTradeFilter<T extends FilterableQuery>(q: T, f: TradeFilter): T` in `trade-query.ts`;
  - `tradeMarksOrEmpty(store: Pick<SignalStore, "tradeMarks">, signalIds: number[]): Promise<TradeMarks>` in `marks.ts`.

- [ ] **Step 1: Write the failing tests**

Create `web/src/lib/store/trade-query.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import type { FilterableQuery } from "./query";
import { applyTradeFilter, toTrade } from "./trade-query";

class FakeQuery implements FilterableQuery {
  calls: string[] = [];
  gte(c: string, v: string) {
    this.calls.push(`gte ${c} ${v}`);
    return this;
  }
  lte(c: string, v: string) {
    this.calls.push(`lte ${c} ${v}`);
    return this;
  }
  eq(c: string, v: string) {
    this.calls.push(`eq ${c} ${v}`);
    return this;
  }
  or(s: string) {
    this.calls.push(`or ${s}`);
    return this;
  }
  like(c: string, p: string) {
    this.calls.push(`like ${c} ${p}`);
    return this;
  }
  order(c: string, o: { ascending: boolean }) {
    this.calls.push(`order ${c} ${o.ascending ? "asc" : "desc"}`);
    return this;
  }
}

describe("applyTradeFilter", () => {
  it("filters by side and symbol prefix, newest first", () => {
    expect(applyTradeFilter(new FakeQuery(), { side: "sell", symbol: "M&M" }).calls).toEqual([
      "eq side sell",
      "like symbol M&M%",
      "order trade_date desc",
      "order id desc",
    ]);
  });
  it("lists everything newest first without a filter", () => {
    expect(applyTradeFilter(new FakeQuery(), {}).calls).toEqual(["order trade_date desc", "order id desc"]);
  });
});

describe("toTrade", () => {
  it("maps snake_case rows and coerces the price", () => {
    expect(
      toTrade({
        id: 3,
        created_at: "2026-09-28T05:00:00+00:00",
        trade_date: "2026-09-28",
        symbol: "TCS",
        side: "buy",
        price: "3521.40",
        signal_id: 12,
        signal: "RSI (GFS)",
      }),
    ).toEqual({
      id: 3,
      createdAt: "2026-09-28T05:00:00+00:00",
      tradeDate: "2026-09-28",
      symbol: "TCS",
      side: "buy",
      price: 3521.4,
      signalId: 12,
      signal: "RSI (GFS)",
    });
  });
  it("keeps a missing price as null", () => {
    const t = toTrade({
      id: 4,
      created_at: "2026-09-28T05:00:00+00:00",
      trade_date: "2026-09-28",
      symbol: "TCS",
      side: "sell",
      price: null,
      signal_id: null,
      signal: null,
    });
    expect(t.price).toBeNull();
  });
});
```

Create `web/src/lib/store/marks.test.ts`:

```ts
import { describe, expect, it, vi } from "vitest";
import { tradeMarksOrEmpty } from "./marks";

describe("tradeMarksOrEmpty", () => {
  it("passes the store's marks through", async () => {
    const store = { tradeMarks: async () => ({ 5: { buy: 1 } }) };
    expect(await tradeMarksOrEmpty(store, [5])).toEqual({ 5: { buy: 1 } });
  });
  it("returns no marks, and logs, when the lookup fails (e.g. 002_trades.sql not run)", async () => {
    const log = vi.spyOn(console, "error").mockImplementation(() => {});
    const store = {
      tradeMarks: async () => {
        throw new Error('Supabase: relation "trades" does not exist');
      },
    };
    expect(await tradeMarksOrEmpty(store, [5])).toEqual({});
    expect(log).toHaveBeenCalled();
    log.mockRestore();
  });
  it("skips the lookup when no signals are shown", async () => {
    const tradeMarks = vi.fn();
    expect(await tradeMarksOrEmpty({ tradeMarks }, [])).toEqual({});
    expect(tradeMarks).not.toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `cd web && npx vitest run src/lib/store/trade-query.test.ts src/lib/store/marks.test.ts`
Expected: FAIL, `Failed to resolve import "./trade-query"` / `"./marks"`.

- [ ] **Step 3: Implement the query helpers and the marks wrapper**

Create `web/src/lib/store/trade-query.ts`:

```ts
import type { Side, Trade, TradeFilter } from "@/lib/types";
import type { FilterableQuery } from "./query";

export const TRADE_COLUMNS = "id,created_at,trade_date,symbol,side,price,signal_id,signal";

export interface TradeRow {
  id: number;
  created_at: string;
  trade_date: string;
  symbol: string;
  side: Side;
  price: number | string | null;
  signal_id: number | null;
  signal: string | null;
}

export function toTrade(r: TradeRow): Trade {
  return {
    id: r.id,
    createdAt: r.created_at,
    tradeDate: r.trade_date,
    symbol: r.symbol,
    side: r.side,
    price: r.price === null ? null : Number(r.price),
    signalId: r.signal_id,
    signal: r.signal,
  };
}

/** Filters by side and symbol prefix and orders newest first. Every builder method returns the same builder. */
export function applyTradeFilter<T extends FilterableQuery>(query: T, f: TradeFilter): T {
  let q: FilterableQuery = query;
  if (f.side) q = q.eq("side", f.side);
  if (f.symbol) q = q.like("symbol", `${f.symbol}%`); // parseTradeFilter guarantees [A-Z0-9&-]
  return q.order("trade_date", { ascending: false }).order("id", { ascending: false }) as T;
}
```

Create `web/src/lib/store/marks.ts`:

```ts
import type { TradeMarks } from "@/lib/types";
import type { SignalStore } from "./types";

/**
 * Buy/Sell marks for the signals on a page. A failure (for example before
 * 002_trades.sql has been run) must not take the signal pages down, so it is
 * logged and the page shows every row unmarked.
 */
export async function tradeMarksOrEmpty(
  store: Pick<SignalStore, "tradeMarks">,
  signalIds: number[],
): Promise<TradeMarks> {
  if (signalIds.length === 0) return {};
  try {
    return await store.tradeMarks(signalIds);
  } catch (e) {
    console.error("[trades] could not load Buy/Sell marks:", e);
    return {};
  }
}
```

- [ ] **Step 4: Extend the store interface**

Replace `web/src/lib/store/types.ts` with:

```ts
import type { NewTrade, Signal, SignalFilter, Status, Trade, TradeFilter, TradeMarks } from "@/lib/types";

export const EXPORT_CAP = 100_000;
export const ALL_CAP = 5_000;
export const MAX_PAGE_SIZE = 1_000;
/** The Trades tab shows at most this many trades, newest first. */
export const TRADES_CAP = 1_000;
/** Signal ids per trades lookup, so request URLs stay short. */
export const MARKS_BATCH = 200;

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

  getSignal(id: number): Promise<Signal | null>;
  /** Inserts a trade. A signal can be bought once and sold once: a repeat returns the existing trade with created=false. */
  addTrade(t: NewTrade): Promise<{ trade: Trade; created: boolean }>;
  deleteTrade(id: number): Promise<void>;
  listTrades(f: TradeFilter, cap: number): Promise<{ rows: Trade[]; total: number }>;
  tradeMarks(signalIds: number[]): Promise<TradeMarks>;
}
```

- [ ] **Step 5: Implement the trade methods on `SupabaseStore`**

In `web/src/lib/store/supabase.ts`, replace the imports with:

```ts
import "server-only";
import { createClient, type SupabaseClient } from "@supabase/supabase-js";
import { tokenValidToday } from "@/lib/time";
import { chunk, tradeMarksFrom } from "@/lib/trades";
import type { NewTrade, Side, Signal, SignalFilter, Status, Trade, TradeFilter, TradeMarks } from "@/lib/types";
import { SIGNAL_COLUMNS, applyFilterToQuery, toSignal, type FilterableQuery, type SignalRow } from "./query";
import { TRADE_COLUMNS, applyTradeFilter, toTrade, type TradeRow } from "./trade-query";
import { MARKS_BATCH, MAX_PAGE_SIZE, TooManyRowsError, type SignalStore } from "./types";
```

and add these methods inside the class, after `saveKiteSession`:

```ts
  async getSignal(id: number): Promise<Signal | null> {
    const { data, error } = await this.db.from("signals").select(SIGNAL_COLUMNS).eq("id", id).maybeSingle();
    if (error) throw new Error(`Supabase: ${error.message}`);
    return data ? toSignal(data as unknown as SignalRow) : null;
  }

  async addTrade(t: NewTrade): Promise<{ trade: Trade; created: boolean }> {
    const row = {
      trade_date: t.tradeDate,
      symbol: t.symbol,
      side: t.side,
      price: t.price,
      signal_id: t.signalId,
      signal: t.signal,
    };
    if (t.signalId === null) {
      const { data, error } = await this.db.from("trades").insert(row).select(TRADE_COLUMNS).single();
      if (error) throw new Error(`Supabase: ${error.message}`);
      return { trade: toTrade(data as unknown as TradeRow), created: true };
    }

    // One buy and one sell per signal: a repeat click must not error, so skip the
    // duplicate and hand back the trade that is already there.
    const { data, error } = await this.db
      .from("trades")
      .upsert(row, { onConflict: "signal_id,side", ignoreDuplicates: true })
      .select(TRADE_COLUMNS);
    if (error) throw new Error(`Supabase: ${error.message}`);
    const inserted = data as unknown as TradeRow[];
    if (inserted.length > 0) return { trade: toTrade(inserted[0]), created: true };

    const existing = await this.db
      .from("trades")
      .select(TRADE_COLUMNS)
      .eq("signal_id", t.signalId)
      .eq("side", t.side)
      .single();
    if (existing.error) throw new Error(`Supabase: ${existing.error.message}`);
    return { trade: toTrade(existing.data as unknown as TradeRow), created: false };
  }

  async deleteTrade(id: number): Promise<void> {
    const { error } = await this.db.from("trades").delete().eq("id", id);
    if (error) throw new Error(`Supabase: ${error.message}`);
  }

  async listTrades(f: TradeFilter, cap: number): Promise<{ rows: Trade[]; total: number }> {
    const base = this.db.from("trades").select(TRADE_COLUMNS, { count: "exact" });
    const query = applyTradeFilter(base as unknown as FilterableQuery, f) as unknown as typeof base;
    const { data, count, error } = await query.range(0, cap - 1);
    if (error) throw new Error(`Supabase: ${error.message}`);
    return { rows: (data as unknown as TradeRow[]).map(toTrade), total: count ?? 0 };
  }

  async tradeMarks(signalIds: number[]): Promise<TradeMarks> {
    const rows: { id: number; signalId: number | null; side: Side }[] = [];
    for (const ids of chunk([...new Set(signalIds)], MARKS_BATCH)) {
      const { data, error } = await this.db.from("trades").select("id,signal_id,side").in("signal_id", ids);
      if (error) throw new Error(`Supabase: ${error.message}`);
      for (const r of data as { id: number; signal_id: number | null; side: Side }[]) {
        rows.push({ id: r.id, signalId: r.signal_id, side: r.side });
      }
    }
    return tradeMarksFrom(rows);
  }
```

- [ ] **Step 6: Run the tests and the typecheck**

Run: `cd web && npx vitest run src/lib/store && npx tsc --noEmit`
Expected: PASS for `query.test.ts`, `trade-query.test.ts`, `marks.test.ts`; tsc prints nothing. Do not commit.

---

### Task 3: API routes

**Files:**
- Create: `web/src/app/api/trades/route.ts`, `web/src/app/api/trades/route.test.ts`
- Create: `web/src/app/api/trades/[id]/route.ts`, `web/src/app/api/trades/[id]/route.test.ts`

**Interfaces:**
- Consumes: `getStore()` from `@/lib/store` (methods from Task 2); `parseTradeInput`, `tradeFromSignal`, `manualTrade` (Task 1); `isSameOrigin` (Task 1); `todayIST` from `@/lib/time`.
- Produces: `POST /api/trades` → `201 { trade }` (created) or `200 { trade }` (already recorded), `400|403|404|500 { error }`; `DELETE /api/trades/{id}` → `204`, `400|403|500 { error }`. The client components in Tasks 5–6 read `body.trade.id` and `body.error`.

- [ ] **Step 1: Write the failing tests**

Create `web/src/app/api/trades/route.test.ts`:

```ts
import { NextRequest } from "next/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { addDays, todayIST } from "@/lib/time";
import type { Signal, Trade } from "@/lib/types";

const store = vi.hoisted(() => ({ getSignal: vi.fn(), addTrade: vi.fn() }));
vi.mock("@/lib/store", () => ({ getStore: () => store }));

import { POST } from "./route";

const signal: Signal = {
  id: 9,
  firedAt: "2026-09-28T04:00:00Z",
  tradeDate: "2026-09-28",
  token: 258817,
  symbol: "SCHAEFFLER",
  strategy: "IchimokuKinkoHyo",
  signal: "IchimokuKinkoHyo (Future BUY)",
  direction: "long",
  price: 4217.6,
  rsiDaily: 67.4,
  rsiWeekly: 56.3,
  rsiMonthly: 55.8,
};
const trade: Trade = {
  id: 1,
  createdAt: "2026-09-28T05:00:00Z",
  tradeDate: "2026-09-28",
  symbol: "SCHAEFFLER",
  side: "buy",
  price: 4217.6,
  signalId: 9,
  signal: "IchimokuKinkoHyo (Future BUY)",
};

function post(body: unknown, origin: string | null = "http://localhost:3000") {
  const headers: Record<string, string> = { host: "localhost:3000", "content-type": "application/json" };
  if (origin) headers.origin = origin;
  const payload = typeof body === "string" ? body : JSON.stringify(body);
  return POST(new NextRequest("http://localhost:3000/api/trades", { method: "POST", headers, body: payload }));
}

beforeEach(() => {
  store.getSignal.mockReset();
  store.addTrade.mockReset();
});
afterEach(() => vi.useRealTimers());

describe("POST /api/trades", () => {
  it("records a trade from a signal with the signal's details: 201", async () => {
    store.getSignal.mockResolvedValue(signal);
    store.addTrade.mockResolvedValue({ trade, created: true });
    const res = await post({ signalId: 9, side: "buy", price: 1, symbol: "HACK" });
    expect(res.status).toBe(201);
    expect(await res.json()).toEqual({ trade });
    expect(store.addTrade).toHaveBeenCalledWith({
      tradeDate: "2026-09-28",
      symbol: "SCHAEFFLER",
      side: "buy",
      price: 4217.6,
      signalId: 9,
      signal: "IchimokuKinkoHyo (Future BUY)",
    });
  });
  it("answers a repeat click with the existing trade: 200", async () => {
    store.getSignal.mockResolvedValue(signal);
    store.addTrade.mockResolvedValue({ trade, created: false });
    const res = await post({ signalId: 9, side: "buy" });
    expect(res.status).toBe(200);
    expect(await res.json()).toEqual({ trade });
  });
  it("says so when the signal no longer exists: 404", async () => {
    store.getSignal.mockResolvedValue(null);
    const res = await post({ signalId: 9, side: "buy" });
    expect(res.status).toBe(404);
    expect((await res.json()).error).toContain("no longer exists");
    expect(store.addTrade).not.toHaveBeenCalled();
  });
  it("records a manual trade dated today (IST): 201", async () => {
    store.addTrade.mockResolvedValue({ trade, created: true });
    const res = await post({ symbol: "m&m", side: "sell" });
    expect(res.status).toBe(201);
    expect(store.addTrade).toHaveBeenCalledWith({
      tradeDate: todayIST(),
      symbol: "M&M",
      side: "sell",
      price: null,
      signalId: null,
      signal: null,
    });
  });
  it("uses the IST day just after midnight, when UTC is still on the previous day", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-09-28T19:00:00Z")); // 00:30 IST on the 29th
    store.addTrade.mockResolvedValue({ trade, created: true });
    expect((await post({ symbol: "TCS", side: "buy", tradeDate: "2026-09-29" })).status).toBe(201);
    expect((await post({ symbol: "TCS", side: "buy", tradeDate: "2026-09-30" })).status).toBe(400);
  });
  it("refuses a future date: 400", async () => {
    const res = await post({ symbol: "TCS", side: "buy", tradeDate: addDays(todayIST(), 1) });
    expect(res.status).toBe(400);
    expect((await res.json()).error).toContain("future");
    expect(store.addTrade).not.toHaveBeenCalled();
  });
  it("refuses a body that isn't JSON: 400", async () => {
    expect((await post("not json")).status).toBe(400);
  });
  it("refuses a cross-site request: 403", async () => {
    const res = await post({ symbol: "TCS", side: "buy" }, "https://evil.example");
    expect(res.status).toBe(403);
    expect(store.addTrade).not.toHaveBeenCalled();
  });
  it("passes a Supabase failure on: 500 with its message", async () => {
    store.addTrade.mockRejectedValue(new Error('Supabase: relation "trades" does not exist'));
    const res = await post({ symbol: "TCS", side: "buy" });
    expect(res.status).toBe(500);
    expect((await res.json()).error).toContain('relation "trades" does not exist');
  });
});
```

Create `web/src/app/api/trades/[id]/route.test.ts`:

```ts
import { NextRequest } from "next/server";
import { beforeEach, describe, expect, it, vi } from "vitest";

const store = vi.hoisted(() => ({ deleteTrade: vi.fn() }));
vi.mock("@/lib/store", () => ({ getStore: () => store }));

import { DELETE } from "./route";

function del(id: string, origin = "http://localhost:3000") {
  const req = new NextRequest(`http://localhost:3000/api/trades/${id}`, {
    method: "DELETE",
    headers: { host: "localhost:3000", origin },
  });
  return DELETE(req, { params: Promise.resolve({ id }) });
}

beforeEach(() => store.deleteTrade.mockReset());

describe("DELETE /api/trades/[id]", () => {
  it("deletes the trade: 204", async () => {
    store.deleteTrade.mockResolvedValue(undefined);
    const res = await del("7");
    expect(res.status).toBe(204);
    expect(store.deleteTrade).toHaveBeenCalledWith(7);
  });
  it.each(["abc", "0", "1.5", "-2", "99999999999999999999"])("refuses the id %s: 400", async (id) => {
    expect((await del(id)).status).toBe(400);
    expect(store.deleteTrade).not.toHaveBeenCalled();
  });
  it("refuses a cross-site request: 403", async () => {
    expect((await del("7", "https://evil.example")).status).toBe(403);
    expect(store.deleteTrade).not.toHaveBeenCalled();
  });
  it("passes a Supabase failure on: 500", async () => {
    store.deleteTrade.mockRejectedValue(new Error("Supabase: down"));
    const res = await del("7");
    expect(res.status).toBe(500);
    expect((await res.json()).error).toBe("Supabase: down");
  });
});
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `cd web && npx vitest run src/app/api/trades`
Expected: FAIL, `Failed to resolve import "./route"` in both files.

- [ ] **Step 3: Implement `POST /api/trades`**

Create `web/src/app/api/trades/route.ts`:

```ts
import { NextResponse, type NextRequest } from "next/server";
import { isSameOrigin } from "@/lib/origin";
import { getStore } from "@/lib/store";
import { todayIST } from "@/lib/time";
import { manualTrade, parseTradeInput, tradeFromSignal } from "@/lib/trades";
import type { NewTrade } from "@/lib/types";

const fail = (status: number, error: string) => NextResponse.json({ error }, { status });

export async function POST(request: NextRequest) {
  if (!isSameOrigin(request.headers.get("origin"), request.headers.get("host"))) {
    return fail(403, "Cross-site request refused.");
  }
  const body = await request.json().catch(() => null);
  const parsed = parseTradeInput(body, todayIST());
  if (!parsed.ok) return fail(400, parsed.error);

  try {
    const store = getStore();
    const input = parsed.value;
    let row: NewTrade;
    if (input.kind === "signal") {
      // The browser only names the signal; its stock, price and day come from the database.
      const signal = await store.getSignal(input.signalId);
      if (!signal) return fail(404, "That signal no longer exists.");
      row = tradeFromSignal(signal, input.side);
    } else {
      row = manualTrade(input);
    }
    const { trade, created } = await store.addTrade(row);
    return NextResponse.json({ trade }, { status: created ? 201 : 200 });
  } catch (e) {
    return fail(500, e instanceof Error ? e.message : String(e));
  }
}
```

- [ ] **Step 4: Implement `DELETE /api/trades/[id]`**

Create `web/src/app/api/trades/[id]/route.ts`:

```ts
import { NextResponse, type NextRequest } from "next/server";
import { isSameOrigin } from "@/lib/origin";
import { getStore } from "@/lib/store";

export async function DELETE(request: NextRequest, { params }: { params: Promise<{ id: string }> }) {
  if (!isSameOrigin(request.headers.get("origin"), request.headers.get("host"))) {
    return NextResponse.json({ error: "Cross-site request refused." }, { status: 403 });
  }
  const { id } = await params;
  const tradeId = /^\d+$/.test(id) ? Number(id) : NaN;
  if (!Number.isSafeInteger(tradeId) || tradeId <= 0) {
    return NextResponse.json({ error: "Unknown trade." }, { status: 400 });
  }
  try {
    await getStore().deleteTrade(tradeId); // deleting a missing id is not an error
  } catch (e) {
    return NextResponse.json({ error: e instanceof Error ? e.message : String(e) }, { status: 500 });
  }
  return new NextResponse(null, { status: 204 });
}
```

- [ ] **Step 5: Run the tests to see them pass**

Run: `cd web && npx vitest run src/app/api/trades && npx tsc --noEmit`
Expected: PASS, both files; tsc prints nothing. Do not commit.

---

### Task 4: Browser test harness (throwaway, outside the repo)

A fake Supabase that the real site and the engine's real Supabase client can both talk to, so Tasks 5–7 can check the UI in a browser. Nothing here is kept.

`$SCRATCH` below is this session's scratchpad: `C:\Users\naman\AppData\Local\Temp\claude\c--vsc-algotrader\e92b713b-a0a5-47ad-ad12-8307e4d2c97d\scratchpad` (Git Bash: `/c/Users/naman/AppData/Local/Temp/claude/c--vsc-algotrader/e92b713b-a0a5-47ad-ad12-8307e4d2c97d/scratchpad`).

**Files:**
- Replace: `$SCRATCH/fake-postgrest.cjs`
- Create then delete: `internal/zz_e2e_tmp/main.go` (inside the repo only so it can import the engine's internal packages)

**Interfaces:**
- Produces: a PostgREST look-alike on `http://localhost:54322` (key `fake_secret`) with `signals`, `kite_session`, `engine_status`, `trades`; `TRADES_MISSING=1` makes `trades` answer like a project without `002_trades.sql`.

- [ ] **Step 1: Write the fake**

Replace `$SCRATCH/fake-postgrest.cjs` with:

```js
// Throwaway in-memory fake of the PostgREST endpoints the engine and website use,
// for browser tests without a real Supabase project. TRADES_MISSING=1 makes
// /trades answer like a project where 002_trades.sql hasn't been run.
const http = require("http");
const KEY = "fake_secret";
const TRADES_MISSING = process.env.TRADES_MISSING === "1";
const signals = [];
const trades = [];
let nextSignalId = 1;
let nextTradeId = 1;
const now = () => new Date().toISOString();
const log = (...a) => console.log(now().slice(11, 19), ...a);

// A PostgREST value list: "1,2" or "\"RSI (GFS)\",\"x\"".
function parseList(inner) {
  const out = [];
  const re = /"((?:[^"\\]|\\.)*)"|([^,]+)/g;
  let m;
  while ((m = re.exec(inner))) out.push(m[1] !== undefined ? m[1].replace(/\\(.)/g, "$1") : m[2]);
  return out;
}

function applyQuery(rows, q) {
  let out = rows.slice();
  for (const [key, raw] of q.entries()) {
    if (["select", "order", "offset", "limit", "on_conflict", "or"].includes(key)) continue;
    const dot = raw.indexOf(".");
    const op = raw.slice(0, dot);
    const val = raw.slice(dot + 1);
    out = out.filter((r) => {
      const v = String(r[key]);
      if (op === "eq") return v === val;
      if (op === "gte") return v >= val;
      if (op === "lte") return v <= val;
      if (op === "like") return v.startsWith(val.replace(/[%*]$/, ""));
      if (op === "in") return parseList(val.replace(/^\(|\)$/g, "")).includes(v);
      return true;
    });
  }
  const or = q.get("or");
  if (or) {
    const clauses = [...or.matchAll(/(strategy|signal)\.in\.\(((?:"(?:[^"\\]|\\.)*"|[^()"])*)\)/g)].map((m) => [m[1], parseList(m[2])]);
    out = out.filter((r) => clauses.some(([col, vals]) => vals.includes(r[col])));
  }
  const order = (q.get("order") || "").split(",").filter(Boolean).map((o) => o.split("."));
  out.sort((a, b) => {
    for (const [col, dir] of order) {
      if (a[col] < b[col]) return dir === "asc" ? -1 : 1;
      if (a[col] > b[col]) return dir === "asc" ? 1 : -1;
    }
    return 0;
  });
  return out;
}

function reply(req, res, status, rows, total, offset = 0) {
  const headers = { "Content-Type": "application/json" };
  if (total !== undefined) headers["Content-Range"] = rows.length ? `${offset}-${offset + rows.length - 1}/${total}` : `*/${total}`;
  if ((req.headers["accept"] || "").includes("vnd.pgrst.object")) {
    if (rows.length !== 1) {
      res.writeHead(406, headers);
      return res.end(JSON.stringify({ code: "PGRST116", message: "JSON object requested, multiple (or no) rows returned" }));
    }
    res.writeHead(status, headers);
    return res.end(JSON.stringify(rows[0]));
  }
  res.writeHead(status, headers);
  res.end(JSON.stringify(rows));
}

function page(req, res, rows, q) {
  const offset = Number(q.get("offset") || 0);
  const limit = Number(q.get("limit") || rows.length);
  const counted = (req.headers["prefer"] || "").includes("count=exact");
  reply(req, res, 200, rows.slice(offset, offset + limit), counted ? rows.length : undefined, offset);
}

http
  .createServer((req, res) => {
    let body = "";
    req.on("data", (c) => (body += c));
    req.on("end", () => {
      const url = new URL(req.url, "http://x");
      const q = url.searchParams;
      const prefer = req.headers["prefer"] || "";
      const json = (status, data) => {
        res.writeHead(status, { "Content-Type": "application/json" });
        res.end(data === undefined ? "" : JSON.stringify(data));
      };
      if (req.headers["apikey"] !== KEY) return json(401, { message: "Invalid API key" });

      if (url.pathname === "/rest/v1/signals") {
        if (req.method === "POST") {
          const row = JSON.parse(body);
          if (signals.some((s) => s.signal === row.signal && s.token === row.token && s.trade_date === row.trade_date)) {
            return json(201, []);
          }
          const saved = { id: nextSignalId++, ...row };
          signals.push(saved);
          log("signal saved:", row.signal, row.symbol);
          return json(201, [saved]);
        }
        return page(req, res, applyQuery(signals, q), q);
      }
      if (url.pathname === "/rest/v1/kite_session") return reply(req, res, 200, [{ created_at: now() }]);
      if (url.pathname === "/rest/v1/engine_status") {
        if (req.method === "POST") return json(201);
        return reply(req, res, 200, [{ last_seen: now(), instruments: 500, feed_connected: true, token_valid: true }]);
      }
      if (url.pathname === "/rest/v1/trades") {
        if (TRADES_MISSING) {
          return json(404, { code: "PGRST205", message: "Could not find the table 'public.trades' in the schema cache" });
        }
        if (req.method === "POST") {
          const incoming = [].concat(JSON.parse(body));
          const saved = [];
          for (const row of incoming) {
            const dup = row.signal_id != null && trades.some((t) => t.signal_id === row.signal_id && t.side === row.side);
            if (dup) {
              if (prefer.includes("resolution=ignore-duplicates")) continue;
              return json(409, { code: "23505", message: 'duplicate key value violates unique constraint "trades_signal_id_side_key"' });
            }
            const t = { id: nextTradeId++, created_at: now(), ...row };
            trades.push(t);
            saved.push(t);
            log("trade saved:", t.side, t.symbol, "signal", t.signal_id);
          }
          if (!prefer.includes("return=representation")) return json(201);
          return reply(req, res, 201, saved);
        }
        if (req.method === "DELETE") {
          const gone = new Set(applyQuery(trades, q).map((t) => t.id));
          for (let i = trades.length - 1; i >= 0; i--) if (gone.has(trades[i].id)) trades.splice(i, 1);
          log("trades deleted:", [...gone]);
          return json(204);
        }
        return page(req, res, applyQuery(trades, q), q);
      }
      log("unexpected", req.method, req.url);
      json(404, { message: "not found" });
    });
  })
  .listen(54322, () => log(`fake postgrest on :54322${TRADES_MISSING ? " (no trades table)" : ""}`));
```

- [ ] **Step 2: Write the throwaway feeder**

Create `internal/zz_e2e_tmp/main.go` (delete in Task 7):

```go
// Throwaway: saves three test signals through the engine's real Supabase client
// into the fake at :54322. Deleted after the browser checks.
package main

import (
	"context"
	"fmt"
	"time"

	"algotrader/internal/domain"
	"algotrader/internal/transport/supabase"
)

func main() {
	c := supabase.New("http://localhost:54322", "fake_secret")
	rsi := func(d, w, m float64) *domain.RSIReadings {
		return &domain.RSIReadings{Daily: d, Weekly: w, WeeklyOK: true, Monthly: m, MonthlyOK: true}
	}
	for _, s := range []domain.OrderSignal{
		{InstrumentToken: 258817, TradingSymbol: "SCHAEFFLER", Strategy: "IchimokuKinkoHyo", StrategyName: "IchimokuKinkoHyo (Future BUY)", Price: 4217.6, RSI: rsi(67.4, 56.3, 55.8)},
		{InstrumentToken: 1793, TradingSymbol: "AARTIIND", Strategy: "IchimokuKinkoHyo", StrategyName: "IchimokuKinkoHyo (Future SELL)", Price: 468.7, RSI: rsi(33.9, 48.4, 51.7)},
		{InstrumentToken: 519937, TradingSymbol: "M&M", Strategy: "RSI", StrategyName: "RSI (Advanced GFS)", Price: 3120.5, RSI: rsi(61.0, 64.2, 66.8)},
	} {
		s.SignalTime = time.Now()
		s.Direction = domain.DirectionOf(s.StrategyName)
		ok, err := c.InsertSignal(context.Background(), s)
		fmt.Println(s.TradingSymbol, s.StrategyName, "inserted:", ok, "err:", err)
	}
}
```

- [ ] **Step 3: Start the fake, seed it, and check the trades endpoints by hand**

Run (Git Bash):

```bash
S=/c/Users/naman/AppData/Local/Temp/claude/c--vsc-algotrader/e92b713b-a0a5-47ad-ad12-8307e4d2c97d/scratchpad
cd $S && (node fake-postgrest.cjs > fake-postgrest.log 2>&1 &) ; sleep 1
cd /c/vsc/algotrader && export PATH="/c/msys64/mingw64/bin:$PATH" && go run ./internal/zz_e2e_tmp
H='-H apikey:fake_secret -H Content-Type:application/json'
curl -s $H -H "Prefer: resolution=ignore-duplicates,return=representation" -d '{"trade_date":"2026-09-28","symbol":"M&M","side":"buy","price":3120.5,"signal_id":3,"signal":"RSI (Advanced GFS)"}' "http://localhost:54322/rest/v1/trades?on_conflict=signal_id,side&select=*"; echo
curl -s $H -H "Prefer: resolution=ignore-duplicates,return=representation" -d '{"trade_date":"2026-09-28","symbol":"M&M","side":"buy","price":3120.5,"signal_id":3,"signal":"RSI (Advanced GFS)"}' "http://localhost:54322/rest/v1/trades?on_conflict=signal_id,side&select=*"; echo
curl -s $H "http://localhost:54322/rest/v1/trades?select=id,signal_id,side&signal_id=in.(1,2,3)"; echo
curl -s -X DELETE $H "http://localhost:54322/rest/v1/trades?id=eq.1" -o /dev/null -w "%{http_code}\n"
curl -s $H "http://localhost:54322/rest/v1/trades?select=*"; echo
```

Expected: the feeder prints three `inserted: true err: <nil>` lines; the first POST prints a one-row array, the second `[]`; the `in` query prints one row with `signal_id` 3; the DELETE prints `204`; the last GET prints `[]`.

---

### Task 5: Buy/Sell buttons on every signal row

**Files:**
- Create: `web/src/components/TradeButtons.tsx`
- Modify: `web/src/components/SignalsTable.tsx`, `web/src/components/LiveSignals.tsx`
- Modify: `web/src/app/(site)/page.tsx`, `web/src/app/(site)/strategies/page.tsx`, `web/src/app/(site)/history/page.tsx`

**Interfaces:**
- Consumes: `POST /api/trades` and `DELETE /api/trades/{id}` (Task 3); `tradeMarksOrEmpty` (Task 2); `TradeMarks`, `Side`, `Trade` (Task 1).
- Produces: `TradeButtons({ signalId: number; initial?: TradeMarks[number] })`; `SignalsTable` prop `marks?: TradeMarks`; `LiveSignals` prop `marks: TradeMarks`.

- [ ] **Step 1: Write `TradeButtons`**

Create `web/src/components/TradeButtons.tsx`:

```tsx
"use client";

import { useState } from "react";
import type { Side, Trade, TradeMarks } from "@/lib/types";

const LABEL = { buy: ["Buy", "Bought ✓"], sell: ["Sell", "Sold ✓"] } as const;
const STYLE = {
  buy: {
    off: "border-emerald-600 text-emerald-700 hover:bg-emerald-50 dark:text-emerald-400 dark:hover:bg-emerald-950",
    on: "border-emerald-600 bg-emerald-600 text-white",
  },
  sell: {
    off: "border-rose-600 text-rose-700 hover:bg-rose-50 dark:text-rose-400 dark:hover:bg-rose-950",
    on: "border-rose-600 bg-rose-600 text-white",
  },
} as const;

/**
 * Records a buy or sell of this signal's stock, or removes it when clicked again.
 * State starts from the marks the page loaded; later changes are kept here, so the
 * Dashboard's 30-second refresh doesn't reset them.
 */
export function TradeButtons({ signalId, initial }: { signalId: number; initial?: TradeMarks[number] }) {
  const [marks, setMarks] = useState<TradeMarks[number]>(initial ?? {});
  const [busy, setBusy] = useState<Side | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function toggle(side: Side) {
    setBusy(side);
    setError(null);
    try {
      const tradeId = marks[side];
      if (tradeId === undefined) {
        const res = await fetch("/api/trades", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ signalId, side }),
        });
        const body = (await res.json().catch(() => ({}))) as { trade?: Trade; error?: string };
        if (!res.ok || !body.trade) throw new Error(body.error ?? `Couldn't save (HTTP ${res.status}).`);
        const id = body.trade.id;
        setMarks((m) => ({ ...m, [side]: id }));
      } else {
        const res = await fetch(`/api/trades/${tradeId}`, { method: "DELETE" });
        if (!res.ok) {
          const body = (await res.json().catch(() => ({}))) as { error?: string };
          throw new Error(body.error ?? `Couldn't remove (HTTP ${res.status}).`);
        }
        setMarks((m) => {
          const next = { ...m };
          delete next[side];
          return next;
        });
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="flex flex-col items-start gap-1">
      <div className="flex gap-1">
        {(["buy", "sell"] as const).map((side) => {
          const on = marks[side] !== undefined;
          return (
            <button
              key={side}
              type="button"
              onClick={() => toggle(side)}
              disabled={busy !== null}
              aria-pressed={on}
              title={on ? "Recorded. Click to remove it." : `Record a ${side} of this stock`}
              className={`w-[4.75rem] rounded border px-2 py-0.5 text-xs font-medium disabled:opacity-60 ${STYLE[side][on ? "on" : "off"]}`}
            >
              {busy === side ? "…" : LABEL[side][on ? 1 : 0]}
            </button>
          );
        })}
      </div>
      {error && <span className="max-w-[12rem] text-xs text-red-600">{error}</span>}
    </div>
  );
}
```

- [ ] **Step 2: Add the Trade column to `SignalsTable`**

In `web/src/components/SignalsTable.tsx`:

- change the type import to `import type { Signal, SortKey, TradeMarks } from "@/lib/types";` and add `import { TradeButtons } from "./TradeButtons";`;
- add `marks,` to the destructured props and `marks?: TradeMarks;` to the props type (after `emptyText?: string;`);
- after `<th className="px-3 py-2 text-right">RSI D / W / M</th>` add `<th className="px-3 py-2">Trade</th>`;
- after the RSI `<td>…</td>` of each row add:

```tsx
              <td className="px-3 py-2">
                <TradeButtons signalId={r.id} initial={marks?.[r.id]} />
              </td>
```

- [ ] **Step 3: Pass marks through the Dashboard**

In `web/src/components/LiveSignals.tsx`: import `TradeMarks` (`import type { Signal, TradeMarks } from "@/lib/types";`), change the signature to

```tsx
export function LiveSignals({ date, initialRows, marks }: { date: string; initialRows: Signal[]; marks: TradeMarks }) {
```

and render `<SignalsTable rows={rows} marks={marks} emptyText="No signals yet today." />`.

Replace `web/src/app/(site)/page.tsx` with:

```tsx
import { KiteFlash } from "@/components/KiteFlash";
import { LiveSignals } from "@/components/LiveSignals";
import { baseFilter, first } from "@/lib/filters";
import { getStore } from "@/lib/store";
import { tradeMarksOrEmpty } from "@/lib/store/marks";
import { ALL_CAP } from "@/lib/store/types";
import { todayIST } from "@/lib/time";

type SP = Promise<Record<string, string | string[] | undefined>>;

export default async function DashboardPage({ searchParams }: { searchParams: SP }) {
  const sp = await searchParams;
  const today = todayIST();
  const store = getStore();
  const rows = await store.listAll(baseFilter({ from: today, to: today }), ALL_CAP);
  const marks = await tradeMarksOrEmpty(store, rows.map((r) => r.id));

  return (
    <>
      <KiteFlash kite={first(sp.kite)} reason={first(sp.reason)} />
      <h1 className="text-xl font-semibold">Today · {today}</h1>
      <LiveSignals date={today} initialRows={rows} marks={marks} />
    </>
  );
}
```

- [ ] **Step 4: Pass marks through Strategies and History**

In `web/src/app/(site)/strategies/page.tsx`: add `import { tradeMarksOrEmpty } from "@/lib/store/marks";`; after `const shown = applyFilter(dayRows, selection);` add

```tsx
  const marks = await tradeMarksOrEmpty(getStore(), shown.map((r) => r.id));
```

and render `<SignalsTable rows={shown} marks={marks} emptyText="No signals for this selection on this day." />`.

In `web/src/app/(site)/history/page.tsx`: add `import { tradeMarksOrEmpty } from "@/lib/store/marks";`; after the `if (page !== requested) …` line add

```tsx
  const marks = await tradeMarksOrEmpty(store, rows.map((r) => r.id));
```

and add `marks={marks}` to the `<SignalsTable … />` props.

- [ ] **Step 5: Typecheck, lint, test**

Run: `cd web && npx tsc --noEmit && npm run lint && npm test`
Expected: tsc and lint print no problems; all vitest files pass.

- [ ] **Step 6: Check the buttons in a browser**

With the Task 4 fake running, start the site against it:

```bash
cd /c/vsc/algotrader/web && SUPABASE_URL=http://localhost:54322 SUPABASE_SERVICE_KEY=fake_secret SITE_PASSCODE=e2e-pass COOKIE_SECRET=e2e-cookie-secret-0123456789abcdef KITE_API_KEY=x KITE_API_SECRET=x KITE_USER_ID=AB1234 npx next dev -p 3100
```

(run in the background). With Playwright (1366×900): open `http://localhost:3100/`, unlock with `e2e-pass`, then check:
1. Every row has **Buy** and **Sell**.
2. Click **Buy** on SCHAEFFLER → it turns into **Bought ✓**; the fake's log shows `trade saved: buy SCHAEFFLER signal 1`.
3. Reload → still **Bought ✓**.
4. Double-click **Sell** on AARTIIND → **Sold ✓**; the fake's log shows exactly one `trade saved: sell AARTIIND` line.
5. Click **Bought ✓** on SCHAEFFLER → back to **Buy**; the log shows `trades deleted`.
6. `/strategies` and `/history` show the same states (AARTIIND **Sold ✓**) and their buttons work.

Do not commit.

---

### Task 6: The Trades tab

**Files:**
- Create: `scripts/gen_web_symbols.go`, `web/src/lib/nse-symbols.json` (generated), `web/src/lib/nse-symbols.test.ts`
- Create: `web/src/components/AddTradeForm.tsx`, `web/src/components/TradesTable.tsx`, `web/src/components/RemoveTradeButton.tsx`
- Create: `web/src/app/(site)/trades/page.tsx`
- Modify: `web/src/components/NavLinks.tsx`, `web/src/app/error.tsx`, `CLAUDE.md`

**Interfaces:**
- Consumes: `SYMBOL_RE`, `parseTradeFilter`, `signalSource` (Task 1); `TRADES_CAP`, `listTrades` (Task 2); `POST`/`DELETE` routes (Task 3).
- Produces: route `/trades` (`?side=buy|sell&symbol=PREFIX`); `AddTradeForm({ today: string })`; `TradesTable({ rows: Trade[] })`; `RemoveTradeButton({ id: number; label: string })`.

- [ ] **Step 1: Write the stock-list generator**

Create `scripts/gen_web_symbols.go`:

```go
//go:build ignore

// Regenerates web/src/lib/nse-symbols.json, the Trades tab's stock suggestions,
// from NSE's official list of listed equities. Run from the repo root:
//
//	go run ./scripts/gen_web_symbols.go
package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	equityListURL = "https://archives.nseindia.com/content/equities/EQUITY_L.csv"
	outPath       = "web/src/lib/nse-symbols.json"
)

// symbolRE is the website's symbol rule (web/src/lib/trades.ts SYMBOL_RE).
var symbolRE = regexp.MustCompile(`^[A-Z0-9&-]{1,30}$`)

func main() {
	req, err := http.NewRequest(http.MethodGet, equityListURL, nil)
	if err != nil {
		fail("build request: %v", err)
	}
	// NSE refuses requests without a browser-like User-Agent.
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		fail("download %s: %v", equityListURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fail("download %s: HTTP %d", equityListURL, resp.StatusCode)
	}

	records, err := csv.NewReader(resp.Body).ReadAll()
	if err != nil {
		fail("parse CSV: %v", err)
	}
	if len(records) < 2 || strings.TrimSpace(records[0][0]) != "SYMBOL" {
		fail("unexpected CSV header %v", records[0])
	}

	seen := map[string]bool{}
	var out [][2]string
	for _, r := range records[1:] {
		if len(r) < 2 {
			continue
		}
		symbol := strings.ToUpper(strings.TrimSpace(r[0]))
		name := strings.TrimSpace(r[1])
		if !symbolRE.MatchString(symbol) || name == "" || seen[symbol] {
			continue
		}
		seen[symbol] = true
		out = append(out, [2]string{symbol, name})
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) // keep "M&M" readable instead of "M\u0026M"
	if err := enc.Encode(out); err != nil {
		fail("encode: %v", err)
	}
	if err := os.WriteFile(outPath, buf.Bytes(), 0o644); err != nil {
		fail("write %s: %v", outPath, err)
	}
	fmt.Printf("wrote %d symbols to %s\n", len(out), outPath)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "gen_web_symbols: "+format+"\n", args...)
	os.Exit(1)
}
```

- [ ] **Step 2: Write the list's test, see it fail, generate, see it pass**

Create `web/src/lib/nse-symbols.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import symbols from "./nse-symbols.json";
import { SYMBOL_RE } from "./trades";

describe("nse-symbols.json", () => {
  it("lists NSE equities as [symbol, company], sorted, unique, every symbol valid", () => {
    expect(symbols.length).toBeGreaterThan(2000);
    let prev = "";
    for (const [symbol, name] of symbols) {
      expect(symbol).toMatch(SYMBOL_RE);
      expect(name.length).toBeGreaterThan(0);
      expect(symbol > prev).toBe(true); // sorted and unique
      prev = symbol;
    }
    expect(symbols.find(([s]) => s === "M&M")?.[1]).toMatch(/Mahindra/);
  });
});
```

Run: `cd web && npx vitest run src/lib/nse-symbols.test.ts`
Expected: FAIL, `Failed to resolve import "./nse-symbols.json"`.

Run: `cd /c/vsc/algotrader && export PATH="/c/msys64/mingw64/bin:$PATH" && go run ./scripts/gen_web_symbols.go`
Expected: `wrote 26xx symbols to web/src/lib/nse-symbols.json`.

Run: `cd web && npx vitest run src/lib/nse-symbols.test.ts`
Expected: PASS.

- [ ] **Step 3: Write the list and its Remove button**

Create `web/src/components/RemoveTradeButton.tsx`:

```tsx
"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

export function RemoveTradeButton({ id, label }: { id: number; label: string }) {
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function remove() {
    if (!window.confirm(`Remove ${label}?`)) return;
    setBusy(true);
    setError(null);
    try {
      const res = await fetch(`/api/trades/${id}`, { method: "DELETE" });
      if (!res.ok) {
        const body = (await res.json().catch(() => ({}))) as { error?: string };
        throw new Error(body.error ?? `Couldn't remove (HTTP ${res.status}).`);
      }
      router.refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <span className="inline-flex flex-col items-end gap-1">
      <button
        type="button"
        onClick={remove}
        disabled={busy}
        className="rounded border border-slate-300 px-2 py-0.5 text-xs text-slate-600 hover:bg-slate-100 disabled:opacity-60 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
      >
        {busy ? "…" : "Remove"}
      </button>
      {error && <span className="max-w-[12rem] text-right text-xs text-red-600">{error}</span>}
    </span>
  );
}
```

Create `web/src/components/TradesTable.tsx`:

```tsx
import { signalSource } from "@/lib/trades";
import type { Trade } from "@/lib/types";
import { RemoveTradeButton } from "./RemoveTradeButton";

const price = new Intl.NumberFormat("en-IN", { minimumFractionDigits: 2, maximumFractionDigits: 2 });

function SideBadge({ side }: { side: Trade["side"] }) {
  return side === "buy" ? (
    <span className="rounded bg-emerald-100 px-1.5 py-0.5 text-xs font-medium text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300">
      Bought
    </span>
  ) : (
    <span className="rounded bg-rose-100 px-1.5 py-0.5 text-xs font-medium text-rose-800 dark:bg-rose-950 dark:text-rose-300">
      Sold
    </span>
  );
}

export function TradesTable({ rows }: { rows: Trade[] }) {
  if (rows.length === 0) {
    return (
      <p className="rounded-lg border border-dashed border-slate-300 p-6 text-center text-sm text-slate-500 dark:border-slate-700">
        No trades yet. Use Buy or Sell on a signal, or add one above.
      </p>
    );
  }

  return (
    <div className="overflow-x-auto rounded-lg border border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
      <table className="min-w-full text-sm">
        <thead className="bg-slate-100 text-left text-xs uppercase tracking-wide text-slate-600 dark:bg-slate-800 dark:text-slate-300">
          <tr>
            <th className="px-3 py-2">Date</th>
            <th className="px-3 py-2">Stock</th>
            <th className="px-3 py-2">Side</th>
            <th className="px-3 py-2 text-right">Price</th>
            <th className="px-3 py-2">From</th>
            <th className="px-3 py-2" />
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-100 dark:divide-slate-800">
          {rows.map((t) => {
            const src = signalSource(t.signal);
            return (
              <tr key={t.id} className="hover:bg-slate-50 dark:hover:bg-slate-800/50">
                <td className="whitespace-nowrap px-3 py-2 tabular-nums">{t.tradeDate}</td>
                <td className="whitespace-nowrap px-3 py-2 font-medium">{t.symbol}</td>
                <td className="px-3 py-2">
                  <SideBadge side={t.side} />
                </td>
                <td className="whitespace-nowrap px-3 py-2 text-right tabular-nums">
                  {t.price === null ? "—" : price.format(t.price)}
                </td>
                <td className="whitespace-nowrap px-3 py-2">
                  {src ? (
                    <>
                      {src.name}
                      <span className="ml-2 text-xs text-slate-500">{src.family}</span>
                    </>
                  ) : (
                    <span className="text-slate-500">Manual</span>
                  )}
                </td>
                <td className="px-3 py-2 text-right">
                  <RemoveTradeButton id={t.id} label={`the ${t.symbol} ${t.side} on ${t.tradeDate}`} />
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
```

- [ ] **Step 4: Write the add form**

Create `web/src/components/AddTradeForm.tsx`:

```tsx
"use client";

import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";
import symbols from "@/lib/nse-symbols.json";
import type { Side, Trade } from "@/lib/types";

const input = "rounded-md border border-slate-300 bg-transparent px-2 py-1.5 text-sm dark:border-slate-700";

export function AddTradeForm({ today }: { today: string }) {
  const router = useRouter();
  const [symbol, setSymbol] = useState("");
  const [side, setSide] = useState<Side>("buy");
  const [price, setPrice] = useState("");
  const [date, setDate] = useState(today);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    const p = price.trim() === "" ? null : Number(price);
    if (p !== null && !Number.isFinite(p)) {
      setError("Price must be a number.");
      return;
    }
    setBusy(true);
    try {
      const res = await fetch("/api/trades", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ symbol, side, price: p, tradeDate: date }),
      });
      const body = (await res.json().catch(() => ({}))) as { trade?: Trade; error?: string };
      if (!res.ok) throw new Error(body.error ?? `Couldn't save (HTTP ${res.status}).`);
      setSymbol("");
      setPrice("");
      router.refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <form
      onSubmit={submit}
      className="space-y-3 rounded-lg border border-slate-200 bg-white p-4 dark:border-slate-800 dark:bg-slate-900"
    >
      <h2 className="text-sm font-medium">Add a trade</h2>
      <div className="flex flex-wrap items-end gap-3">
        <label className="flex flex-col gap-1 text-xs text-slate-500">
          Stock
          <input
            list="nse-symbols"
            value={symbol}
            onChange={(e) => setSymbol(e.target.value)}
            required
            autoComplete="off"
            placeholder="Symbol or company"
            className={`${input} w-56`}
          />
        </label>
        <datalist id="nse-symbols">
          {symbols.map(([s, name]) => (
            <option key={s} value={s} label={name} />
          ))}
        </datalist>
        <fieldset className="flex gap-1">
          <legend className="sr-only">Side</legend>
          {(["buy", "sell"] as const).map((s) => (
            <label
              key={s}
              className={`cursor-pointer rounded-md border px-3 py-1.5 text-sm font-medium ${
                side === s
                  ? s === "buy"
                    ? "border-emerald-600 bg-emerald-600 text-white"
                    : "border-rose-600 bg-rose-600 text-white"
                  : "border-slate-300 text-slate-600 dark:border-slate-700 dark:text-slate-300"
              }`}
            >
              <input type="radio" name="side" value={s} checked={side === s} onChange={() => setSide(s)} className="sr-only" />
              {s === "buy" ? "Buy" : "Sell"}
            </label>
          ))}
        </fieldset>
        <label className="flex flex-col gap-1 text-xs text-slate-500">
          Price (optional)
          <input
            type="number"
            step="any"
            min="0.01"
            inputMode="decimal"
            value={price}
            onChange={(e) => setPrice(e.target.value)}
            className={`${input} w-32`}
          />
        </label>
        <label className="flex flex-col gap-1 text-xs text-slate-500">
          Date
          <input
            type="date"
            value={date}
            max={today}
            required
            onChange={(e) => setDate(e.target.value)}
            className={input}
          />
        </label>
        <button
          disabled={busy}
          className="rounded-md bg-slate-900 px-4 py-1.5 text-sm font-medium text-white disabled:opacity-60 dark:bg-slate-100 dark:text-slate-900"
        >
          {busy ? "Adding…" : "Add"}
        </button>
      </div>
      {error && <p className="text-sm text-red-600">{error}</p>}
    </form>
  );
}
```

- [ ] **Step 5: Write the page and the nav entry**

Create `web/src/app/(site)/trades/page.tsx`:

```tsx
import { AddTradeForm } from "@/components/AddTradeForm";
import { TradesTable } from "@/components/TradesTable";
import { getStore } from "@/lib/store";
import { TRADES_CAP } from "@/lib/store/types";
import { todayIST } from "@/lib/time";
import { parseTradeFilter } from "@/lib/trades";
import type { Side } from "@/lib/types";

type SP = Promise<Record<string, string | string[] | undefined>>;

const TABS: { label: string; side?: Side }[] = [
  { label: "All" },
  { label: "Bought", side: "buy" },
  { label: "Sold", side: "sell" },
];

export default async function TradesPage({ searchParams }: { searchParams: SP }) {
  const filter = parseTradeFilter(await searchParams);
  const { rows, total } = await getStore().listTrades(filter, TRADES_CAP);

  const href = (side?: Side) => {
    const p = new URLSearchParams();
    if (side) p.set("side", side);
    if (filter.symbol) p.set("symbol", filter.symbol);
    const qs = p.toString();
    return qs ? `/trades?${qs}` : "/trades";
  };

  return (
    <>
      <div>
        <h1 className="text-xl font-semibold">Trades</h1>
        <p className="text-sm text-slate-500">
          {total.toLocaleString("en-US")} trade{total === 1 ? "" : "s"}
          {filter.symbol ? ` · stocks starting ${filter.symbol}` : ""}
        </p>
      </div>
      <AddTradeForm today={todayIST()} />
      <div className="flex flex-wrap items-center justify-between gap-3">
        <nav className="flex gap-1">
          {TABS.map((t) => (
            <a
              key={t.label}
              href={href(t.side)}
              className={`rounded-md px-3 py-1.5 text-sm font-medium ${
                filter.side === t.side
                  ? "bg-slate-900 text-white dark:bg-slate-100 dark:text-slate-900"
                  : "text-slate-600 hover:bg-slate-200 dark:text-slate-300 dark:hover:bg-slate-800"
              }`}
            >
              {t.label}
            </a>
          ))}
        </nav>
        <form method="get" action="/trades" className="flex gap-2">
          {filter.side && <input type="hidden" name="side" value={filter.side} />}
          <input
            type="text"
            name="symbol"
            defaultValue={filter.symbol}
            placeholder="Stock starts with…"
            className="w-44 rounded-md border border-slate-300 bg-transparent px-2 py-1.5 text-sm uppercase placeholder:normal-case dark:border-slate-700"
          />
          <button className="rounded-md border border-slate-300 px-3 py-1.5 text-sm dark:border-slate-700">Search</button>
        </form>
      </div>
      {total > rows.length && (
        <p className="text-xs text-slate-500">
          Showing the newest {rows.length.toLocaleString("en-US")} of {total.toLocaleString("en-US")}.
        </p>
      )}
      <TradesTable rows={rows} />
    </>
  );
}
```

In `web/src/components/NavLinks.tsx`, add `{ href: "/trades", label: "Trades" },` after the History entry.

In `web/src/app/error.tsx`, replace the whole line

```tsx
        missing or wrong, the tables haven&apos;t been created yet (run <code>supabase/migrations/001_init.sql</code>), or
```

with

```tsx
        missing or wrong, the tables haven&apos;t been created yet (run <code>supabase/migrations/001_init.sql</code>, and{" "}
        <code>002_trades.sql</code> for the Trades tab), or
```

so the paragraph reads "… is missing or wrong, the tables haven't been created yet (run supabase/migrations/001_init.sql, and 002_trades.sql for the Trades tab), or Supabase is temporarily unreachable."

In `CLAUDE.md` "Website (`web/`)", add a bullet:

```markdown
- Trades: the `trades` table (`supabase/migrations/002_trades.sql`), Buy/Sell buttons (`TradeButtons`) on every signals table, and the `/trades` page. `POST /api/trades` and `DELETE /api/trades/[id]` are the only writes and reject a cross-site `Origin`. A signal trade's stock, price and day are copied server-side from the signal. Stock suggestions: `src/lib/nse-symbols.json`, regenerated with `go run ./scripts/gen_web_symbols.go`.
```

- [ ] **Step 6: Typecheck, lint, test**

Run: `cd web && npx tsc --noEmit && npm run lint && npm test`
Expected: no problems; all vitest files pass.

- [ ] **Step 7: Check the tab in a browser**

With the fake and `next dev` from Task 5 still running, with Playwright (1366×900):
1. The nav shows **Trades**; `/trades` lists AARTIIND **Sold** from Task 5 with From "Future SELL IchimokuKinkoHyo" and price 468.70.
2. Add: type `M&M`, choose **Buy**, price `3120.5`, keep today's date → **Add** → the list shows M&M **Bought** 3,120.50 "Manual" at the top; the form clears.
3. Add `reliance` with no price → shows RELIANCE, price "—".
4. Typing "Mahindra" in the stock field offers M&M in the suggestions.
5. Add `NOT A SYMBOL` → the form shows "Enter a valid NSE symbol, e.g. RELIANCE or M&M." and nothing is added.
6. **Bought** / **Sold** / **All** filter the list; searching the stock box for `m&m` (lowercase) shows only M&M, the heading says "stocks starting M&M", and the **Sold** tab link then keeps `symbol=M%26M`.
7. **Remove** on RELIANCE → confirm → it disappears.

Do not commit.

---

### Task 7: Full verification and cleanup

**Files:** none created; deletes `internal/zz_e2e_tmp/` and `.playwright-mcp/`.

- [ ] **Step 1: Full checks**

Run:

```bash
cd /c/vsc/algotrader/web && npm test && npx tsc --noEmit && npm run lint && npm run build
cd /c/vsc/algotrader && export PATH="/c/msys64/mingw64/bin:$PATH" && go vet ./... && go test ./...
```

Expected: every vitest file passes; tsc and lint clean; the build lists `/trades`, `/api/trades` and `/api/trades/[id]`; every Go package `ok`.

- [ ] **Step 2: Production build in a browser, desktop and phone**

Stop `next dev`, then start `npx next start -p 3100` with the same environment as Task 5 Step 6. With Playwright:
1. At 1366×900 repeat Task 5 Step 6 checks 2–5 and Task 6 Step 7 checks 1–2 and 7 (they must pass on the production build too).
2. At 390×844: Dashboard and `/trades` have no horizontal page scroll (only the tables scroll inside their frames); the add form wraps onto several lines; buttons are tappable.
3. The browser console shows no errors.

- [ ] **Step 3: The migration-not-run case**

Stop the fake; restart it with `TRADES_MISSING=1 node fake-postgrest.cjs`; re-run the feeder (`go run ./internal/zz_e2e_tmp`). Then:
1. `/`, `/strategies`, `/history` load and show every signal with unrecorded **Buy**/**Sell**.
2. Clicking **Buy** shows the error "Supabase: Could not find the table 'public.trades' in the schema cache" under the buttons, and the button stays **Buy**.
3. `/trades` shows the "Couldn't load signals" page, whose text names `002_trades.sql`.

- [ ] **Step 4: Clean up**

Stop the site and the fake (ports 3100 and 54322), then:

```bash
rm -rf /c/vsc/algotrader/internal/zz_e2e_tmp /c/vsc/algotrader/.playwright-mcp
cd /c/vsc/algotrader && git status --short
```

Expected: only the files listed in this plan's File Structure show as new or modified. Do not commit; tell the user the change is ready and that `002_trades.sql` must be run in Supabase before deploying.
