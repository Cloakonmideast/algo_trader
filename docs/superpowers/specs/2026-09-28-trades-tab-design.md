# Trades Tab: Record Buys and Sells — Design

Date: 2026-09-28
Status: approved in conversation, pending written-spec review

## Goal

Let the user record which stocks they actually bought or sold, and see them in
one list on any device:

- a **Buy** and a **Sell** button on every signal row (Dashboard, Strategies,
  History) that records the trade in one click;
- a new **Trades** tab listing everything recorded, where a trade can also be
  added by searching for a stock.

Single user, personal use. It is a simple list: no quantities, no profit/loss.

## Decisions

| Topic | Decision |
|---|---|
| What is tracked | A simple list of buys and sells: date, stock, side, price, and the signal it came from (or "Manual") |
| Storage | New Supabase table `trades`, so every device sees the same list |
| Buttons | Both Buy and Sell on every signal row; a recorded side shows **Bought ✓** / **Sold ✓**, and clicking it again removes that record |
| Trade from a signal | Takes the signal's stock, price and trade date; the server copies them from the signal row, the browser only sends the signal id and side |
| Manual trade | Stock (search or typed), side, optional price, date (defaults to today, never in the future) |
| Stock search | Suggestions from NSE's official list of listed equities, bundled with the site; any valid symbol can also be typed |
| Engine | No change |
| Out of scope | Quantities, profit/loss, live prices, editing a trade (remove and re-add), Excel export of trades, pairing a sell with a buy |

## Data model (`supabase/migrations/002_trades.sql`)

Run once in the Supabase SQL editor, after `001_init.sql`.

```sql
create table if not exists trades (
  id          bigint generated always as identity primary key,
  created_at  timestamptz   not null default now(),
  trade_date  date          not null,                  -- IST day of the trade
  symbol      text          not null check (symbol ~ '^[A-Z0-9&-]{1,30}$'),
  side        text          not null check (side in ('buy', 'sell')),
  price       numeric(12,2) check (price > 0),         -- optional on manual trades
  signal_id   bigint        references signals(id) on delete set null,
  signal      text,                                    -- signal name when recorded; null = manual
  unique (signal_id, side)
);
create index if not exists trades_date_idx   on trades (trade_date desc, id desc);
create index if not exists trades_signal_idx on trades (signal_id);
alter table trades enable row level security;  -- no policies: service key only
```

- `unique (signal_id, side)` makes a double click harmless: one signal can be
  bought once and sold once. Manual trades have a null `signal_id`, which the
  constraint ignores.
- Deleting a signal (e.g. the fake-signal cleanup) keeps its trades: the link
  becomes null and the copied symbol, price and signal name remain.

## Website (`web/`)

### Types and validation (`src/lib/types.ts`, new `src/lib/trades.ts`)

- `Side = "buy" | "sell"`; `Trade { id, createdAt, tradeDate, symbol, side,
  price: number | null, signalId: number | null, signal: string | null }`;
  `TradeFilter { side?, symbol? }`; `TradeMarks = Record<signalId, { buy?:
  tradeId; sell?: tradeId }>`.
- `parseTradeInput(body, today)` returns either a signal trade `{ signalId,
  side }` or a manual trade `{ symbol, side, price, tradeDate }`, or an error
  message. Rules: symbol trimmed, uppercased and matching `^[A-Z0-9&-]{1,30}$`;
  price absent or a finite number above 0 and below 10 crore; date `YYYY-MM-DD`
  and not after today (IST), defaulting to today.
- `parseTradeFilter(searchParams)` for the Trades page's `side` and `symbol`
  query parameters. `symbol` is a prefix match with the same cleanup as the
  signals filter (`REL` finds RELIANCE and RELAXO).

### Store (`src/lib/store/`)

`SignalStore` gains:

- `getSignal(id)` → the signal or null;
- `addTrade(input)` → `{ trade, created }`: an upsert that ignores a duplicate
  `(signal_id, side)` and then returns the existing row, so a repeated click
  never errors;
- `deleteTrade(id)`;
- `listTrades(filter, cap)` → newest first (`trade_date desc, id desc`),
  capped at 1,000 rows;
- `tradeMarks(signalIds)` → `TradeMarks` for the rows on screen, fetched in
  batches of 200 ids to keep request URLs short.

### API routes

Behind the passcode gate like every other route (`src/proxy.ts`). The unlock
cookie is `SameSite=Lax`, so another site cannot send these requests with it;
the routes also reject (403) a request whose `Origin` header names a different
host. Browsers always send `Origin` on these calls; a request without one is
left to the cookie check.

| Route | Does |
|---|---|
| `POST /api/trades` | JSON body `{ signalId, side }` or `{ symbol, side, price?, tradeDate? }`. For a signal trade the server loads the signal and copies its symbol, price, trade date and name (404 if it no longer exists). Returns the trade: 201 if created, 200 if it already existed. 400 with `{ error }` for invalid input. |
| `DELETE /api/trades/[id]` | Removes the trade; 204. Deleting a missing id also returns 204. |

The pages read trades through the store directly, so no list endpoint is
needed.

### Buy/Sell buttons (`SignalsTable`, new `TradeButtons` client component)

- `SignalsTable` gains a last column, **Trade**, and a `marks` prop. Each row
  renders `TradeButtons` with the row's signal id and its marks.
- `TradeButtons` shows **Buy** and **Sell** (outlined, green and red). Clicking
  one posts `{ signalId, side }`; on success the button becomes **Bought ✓** or
  **Sold ✓** (filled). Clicking a filled button deletes that trade and reverts.
  Buttons are disabled while a request is in flight; a failure shows a short
  error under the buttons and keeps the previous state.
- Dashboard, Strategies and History load marks for the signals they show
  (`tradeMarks`) when the page renders. If that read fails (for example before
  `002_trades.sql` has been run), the page still renders without marks and the
  failure is logged; a click then shows the API's error.
- The Dashboard's 30-second refresh keeps each row's button state, since it is
  held per row. New rows arrive unmarked. A trade recorded on another device
  appears after a page reload.

### Trades tab (`/trades`, new page)

- **Nav:** Dashboard · Strategies · History · **Trades**.
- **Add a trade** (client form): stock field with suggestions (a `<datalist>`
  of symbol plus company name), Buy/Sell choice (default Buy), optional price,
  date (default today, max today), **Add**. On success the list refreshes
  (`router.refresh()`) and the form clears; errors show inline.
- **Filters:** All / Bought / Sold links and a stock search box, as query
  parameters (`?side=buy&symbol=REL`) so a filtered view can be bookmarked.
- **List**, newest first: Date · Stock · Side (Bought/Sold badge) · Price
  (or "—") · From (signal short name and family, e.g. "Future BUY
  IchimokuKinkoHyo", or "Manual") · **Remove** (asks for confirmation, then
  deletes and refreshes). An empty list explains the two ways to add trades.
  When more than 1,000 trades match, the page says only the newest 1,000 are
  shown.

### Stock list (`src/lib/nse-symbols.json`, `scripts/gen_web_symbols.go`)

- A JSON array of `[symbol, company name]` for NSE's listed equities, from
  NSE's official `EQUITY_L.csv`, sorted by symbol. It ships with the site and
  is only loaded by the Trades page.
- `go run ./scripts/gen_web_symbols.go` regenerates it (the repo's pattern for
  one-off tools, `//go:build ignore`). New listings can be typed in by hand
  until it is re-run.

## Error handling

- Invalid input: 400 with a message the form shows as is.
- Missing table (migration not run): the API returns Supabase's error; the
  Trades page falls through to `src/app/error.tsx`, which gains a line pointing
  at `supabase/migrations/002_trades.sql`.
- Supabase unreachable: same as the rest of the site.

## Testing

- **Unit (vitest):** `parseTradeInput` (both shapes, every rule and boundary),
  `parseTradeFilter`, the store's marks shaping and id batching, the Origin
  check.
- **Browser (Playwright) against the in-memory fake Supabase**, extended with
  the `trades` endpoints and the real engine code feeding signals:
  - click Buy on a Dashboard row → **Bought ✓**; the Trades tab lists it with
    the signal's price and "From" name; reload keeps both;
  - click **Bought ✓** again → removed from both places;
  - Sell on a Strategies row and on a History row;
  - add a manual trade through the stock search, with and without a price;
  - filters All / Bought / Sold and the stock search;
  - Remove with confirmation;
  - invalid input (future date, bad symbol) shows the error;
  - a double click records one trade;
  - phone width (390 px): no horizontal page scroll outside the tables.
- `npm test`, `npx tsc --noEmit`, `npm run lint`, `npm run build`.

## Deployment

1. Supabase → SQL Editor: run `supabase/migrations/002_trades.sql` once.
2. Push; Vercel redeploys. No new environment variables, no engine change.
3. `docs/SETUP.md` and `web/README.md` gain the migration step.
