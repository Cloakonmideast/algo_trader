# Signals Dashboard, Supabase Storage & One-Click Kite Login — Design

Date: 2026-09-24
Status: approved in conversation, pending written-spec review

## Goal

A personal website to see and export every signal the engine fires, backed by
Supabase, plus a daily Kite login that takes one click and a TOTP instead of a
PowerShell script and a `.env` edit.

Single user. The user sets up GitHub, Oracle Cloud, Supabase, Vercel and the
Kite developer console themselves, following `docs/SETUP.md`. Everything else
is in this repo.

## Decisions

| Topic | Decision |
|---|---|
| Engine host | Oracle Cloud Always Free, Ubuntu 24.04 ARM, Mumbai/Hyderabad |
| Website host | Vercel, Next.js (App Router, TypeScript), root directory `web/` |
| Storage | Supabase Postgres; the engine talks to it over the REST API (PostgREST) |
| History | Live signals only, no backfill of past signals |
| Site access | One-time passcode per device, remembered for a year by a signed cookie |
| Kite token | Daily login via a "Log in to Kite" button; the token is stored in Supabase and shared by the engine, the site and the offline tools |
| Discord | Unchanged; the site is an addition |
| Live updates | Dashboard polls every 30 s (the browser never talks to Supabase directly) |

## Architecture

```
Oracle VM: Go engine ── HTTPS (service key) ──► Supabase: signals, kite_session, engine_status
                                                   ▲
Vercel: Next.js server side ── HTTPS (service key) ┘
   ▲
Browser (unlocked by passcode cookie) — pages, JSON API, Excel built client-side
```

- Only two parties hold the Supabase service key: the engine (`.env` on the VM)
  and the Next.js server (Vercel env). RLS is enabled on every table with no
  policies, so the anon key can read or write nothing.
- The Kite access token never leaves the server side. The browser only ever
  learns whether a valid token exists today.

## Data model (`supabase/migrations/001_init.sql`)

```sql
create table signals (
  id          bigint generated always as identity primary key,
  fired_at    timestamptz not null,
  trade_date  date        not null,          -- IST calendar date of fired_at
  token       bigint      not null,
  symbol      text        not null,
  strategy    text        not null,          -- family: 'RSI', 'IchimokuKinkoHyo'
  signal      text        not null,          -- exact variant, e.g. 'IchimokuKinkoHyo (BUY) 1A'
  direction   text        not null check (direction in ('long','short')),
  price       numeric(12,2) not null,
  rsi_daily   numeric(5,2),                  -- RSI signals only
  rsi_weekly  numeric(5,2),
  rsi_monthly numeric(5,2),
  created_at  timestamptz not null default now(),
  unique (signal, token, trade_date)
);
create index on signals (trade_date desc, fired_at desc);
create index on signals (strategy, trade_date);
create index on signals (signal, trade_date);
create index on signals (symbol, trade_date);

create table kite_session (      -- single row
  id           smallint primary key default 1 check (id = 1),
  access_token text        not null,
  user_id      text        not null,
  created_at   timestamptz not null default now()
);

create table engine_status (     -- single row, heartbeat
  id           smallint primary key default 1 check (id = 1),
  last_seen    timestamptz not null,
  instruments  integer     not null,
  feed_connected boolean   not null,
  token_valid  boolean     not null
);

alter table signals       enable row level security;
alter table kite_session  enable row level security;
alter table engine_status enable row level security;
```

**Token validity:** a token is valid for today when `created_at` is at or after
06:00 IST today. Zerodha expires tokens early each morning. The same rule is
implemented in Go and TypeScript.

## Engine changes (Go)

### Signal model
`domain.OrderSignal` gains:
- `Strategy string` (family)
- `Price float64` (tick `LastPrice`)
- `Direction domain.Direction`
- `RSI *domain.RSIReadings{Daily, Weekly, Monthly float64}` (nil for non-RSI)

The strategies fill `Strategy`, `Price` and `RSI`. `Direction` is derived from
the signal name by `domain.DirectionOf`, which is moved from
`backtest.InferDirection`; the backtester calls the shared function so the
site, the exports and the backtests always agree.

### Supabase client — `internal/transport/supabase`
Plain `net/http` against PostgREST, with the service key in the `apikey` and
`Authorization` headers:
- `InsertSignal(row) (inserted bool, err)`: `POST /rest/v1/signals?on_conflict=signal,token,trade_date`
  with `Prefer: resolution=ignore-duplicates,return=representation`. An empty
  array back means the row was a duplicate, so `inserted = false`.
- `LatestToken() (token, userID string, createdAt time.Time, err)`
- `Heartbeat(status)`: upsert of the `engine_status` row.
- Retries with backoff on 5xx and timeouts (up to 3 attempts, 10 s timeout each).

### Save-then-alert
`Processor.dispatchSignal` becomes:
1. The in-memory `StateRegistry` lock (unchanged) stops repeats within the process.
2. `InsertSignal`:
   - `inserted` → send Discord.
   - Duplicate → skip Discord (already alerted before an engine restart).
   - Error → send Discord anyway, and put the row on an in-memory retry queue
     (bounded to 10k, retried every minute). An alert is never lost because
     Supabase is down.

### Kite session watcher — `internal/service/kitesession.go`
- Polls `LatestToken` every 60 s.
- **Startup:** blocks until a valid token exists, instead of `KITE_ACCESS_TOKEN`
  being required. If the env var is set and valid, it's used directly (local dev).
- **On a token change:** `kc.SetAccessToken`, then restart `MultiTicker` with the
  new token (Stop, then a new instance).
- **Morning boot:** the 08:45 cron job runs `Boot` only if the token is valid
  today. Otherwise the boot is deferred, and the watcher runs it when the
  token arrives.
- **Reminders:** at 08:00 IST on weekdays, and every 30 min until 15:30, while no
  valid token exists: Discord `SendAlert("Kite login needed", SITE_URL)`.

### Heartbeat
Every 60 s: upsert `engine_status` (instrument count, whether the feed is
connected, whether the token is valid). This also keeps the free Supabase
project from pausing.

### Scheduler timezone
`cron.New(cron.WithLocation(IST))`. The VM runs UTC, and the jobs must stay at
08:45 and 15:45 IST.

### Config
- New: `SUPABASE_URL`, `SUPABASE_SERVICE_KEY`, `SITE_URL`.
- `KITE_ACCESS_TOKEN` becomes optional.
- `DISCORD_BOT_TOKEN`, `DISCORD_CHANNEL_ID`, `DISCORD_APPLICATION_ID`,
  `DISCORD_PUBLIC_KEY` become optional, since nothing uses them.
- Add a `.env.example`.

### Offline tools
`backfill`, `scanner` and the backtester's symbol refresh use a shared helper:
the env `KITE_ACCESS_TOKEN` if set, otherwise the Supabase token if valid today.

## Website (`web/`)

Next.js App Router, TypeScript, Tailwind. Supabase is accessed only on the
server, with the service key (`server-only` module). Times are displayed in IST.

### Access control
- `middleware.ts` requires a valid `unlock` cookie on every route except
  `/unlock` and `/api/unlock`.
- `POST /api/unlock` compares against `SITE_PASSCODE` in constant time and sets
  the cookie `unlock = HMAC(COOKIE_SECRET, "v1")`: httpOnly, Secure,
  SameSite=Lax, 1 year.
- Rotating `COOKIE_SECRET` locks every device out.

### Pages
- **`/` Dashboard:**
  - Header: Kite status (valid today / login needed + button) and engine status
    (last seen).
  - Today's counts by family and direction.
  - Latest-signals table (today, newest first), refreshed every 30 s.
  - **Export today** button.
- **`/strategies`:**
  - Sidebar lists families → exact signals with the selected day's counts.
  - Selecting one filters the table.
  - Date picker with ◀ ▶ arrows; state lives in the URL (`?date=&strategy=&signal=`).
  - **Export** button for the current view.
- **`/history`:**
  - Filters: date range, families and signals (multi-select), direction,
    symbol search.
  - 100 rows per page; sort by time, symbol or signal. State lives in the URL.
  - **Export to Excel** exports the whole filtered result.
- **`/unlock`:** passcode form.
- Mobile: tables scroll sideways; filters collapse into a panel.

### API routes
- `GET /api/signals?from&to&strategy&signal&direction&symbol&page&pageSize&sort`:
  JSON used by the pages and by export. The server pages through Supabase in
  1,000-row batches. Export requests are capped at 100k rows, with a clear
  error above that.
- `GET /api/status`: `{ tokenValidToday, tokenCreatedAt, engineLastSeen, feedConnected }`.
- `GET /api/kite/login`: redirect to `https://kite.zerodha.com/connect/login?v=3&api_key=…`.
- `GET /api/kite/callback?request_token&status`:
  1. Checksum = `sha256(api_key + request_token + api_secret)`; `POST https://api.kite.trade/session/token`.
  2. Reject unless the response's `user_id` equals `KITE_USER_ID`.
  3. Upsert `kite_session`.
  4. Redirect to `/?kite=ok` (or `?kite=error&reason=…`).

### Excel export
Built in the browser with `exceljs` from `/api/signals` JSON:
- **"Signals" sheet:** one row per signal. Columns: Date, Time (IST), Symbol,
  Strategy, Signal, Direction, Price, RSI D/W/M. Frozen header and autofilter.
- **"Summary" sheet:** count per exact signal, long/short split.
- **File name:** `signals_<from>_to_<to>[_<filter>].xlsx`, or `signals_<date>.xlsx`
  for a single day.

### Vercel environment
`SUPABASE_URL`, `SUPABASE_SERVICE_KEY`, `SITE_PASSCODE`, `COOKIE_SECRET`,
`KITE_API_KEY`, `KITE_API_SECRET`, `KITE_USER_ID`.

## Deployment assets

- `deploy/setup.sh`: installs Go and gcc, creates `/opt/algotrader`, sets the
  timezone and installs the service.
- `deploy/algotrader.service`: systemd unit with `Restart=always`,
  `EnvironmentFile=/opt/algotrader/.env` and `WorkingDirectory=/opt/algotrader`.
- `deploy/update.sh`: `git pull`, `go test ./internal/...`, `go build`,
  `systemctl restart`.
- `docs/SETUP.md`: step by step for Supabase (project + run the migration),
  Oracle VM (create, copy `market_data.db`, run the setup), Vercel (import repo,
  root `web/`, env vars), the Kite developer console (redirect URL) and GitHub.

## Error handling

| Failure | Behaviour |
|---|---|
| Supabase down when a signal fires | Discord still alerts; the row is queued and retried every minute |
| No token at startup or in the morning | Engine waits; Discord reminders; the site shows "login needed" |
| Kite callback for another Zerodha user | Rejected, token untouched, error shown on the dashboard |
| Kite callback `status != success` or exchange fails | Error shown; the existing token is kept |
| Engine not running | Site shows the "last seen" age in amber after 3 min, red after 10 |
| Export > 100k rows | Clear message asking to narrow the range |

## Testing

- **Go:**
  - Supabase client against `httptest` (insert / duplicate / 5xx retry).
  - Save-then-alert decisions (fake client and fake sender).
  - Token watcher: validity window, change detection, deferred boot, reminder
    schedule (injected clock).
  - `DirectionOf` table test.
  - RSI and Ichimoku fill `Price` / `Strategy` / `RSI`.
- **Web (vitest):**
  - Cookie sign/verify.
  - Passcode check.
  - Kite checksum and callback handler (mocked fetch: success, wrong user,
    failed exchange).
  - Token-valid-today in IST.
  - Workbook builder (sheets, headers, summary counts).
- **Manual end-to-end (in SETUP.md):** unlock the site, log in to Kite, see the
  engine pick up the token, insert a test signal, see it on the dashboard,
  export it.

## Out of scope

Backfilling past signals, order placement, multiple users, the Discord bot or
interactions, and hosting and GitHub setup themselves (documented, not performed).
