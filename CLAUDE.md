# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

Go signal-alerting engine for NSE equities: Zerodha Kite Connect (REST + WebSocket) → daily candles in SQLite → strategy evaluation → signals saved to Supabase (shown on the `web/` site) and sent to a Discord webhook. It does not place orders. `ARCHITECTURE.md` has the original design notes and backtester documentation; parts of it are older than the code (e.g. it names only the breakout strategy), so trust the code where they differ.

## Build, test, run (Windows)

`go-sqlite3` needs CGo with a **64-bit** GCC. The system default MinGW is 32-bit, so put MSYS2's first on PATH before any build or test:

```powershell
$env:PATH = "C:\msys64\mingw64\bin;$env:PATH"
go build ./...
go test ./internal/...                                    # tests exist only in internal/strategy and internal/backtest
go test ./internal/strategy -run TestRSI -v                # single test / pattern
go build -o algotrader.exe ./cmd/engine
```

Binaries need env vars from `.env` loaded into the process first. `scan.ps1` shows the loader (it strips trailing `# comments`) and then runs the scanner; reuse that pattern for other commands.

| Command | Purpose | Needs Kite creds? |
|---|---|---|
| `go run ./cmd/engine` | Live engine (long-running, cron + WebSockets) | yes |
| `go run ./cmd/backfill` / `-years N [-force]` | Incremental gap fill / deep history fetch into SQLite | yes |
| `go run ./cmd/backtest -strategy rsi\|ichimoku\|all [-universe all] [-confluence] ...` | Offline replay of stored candles through the live strategies | no (only to refresh `backtest_symbols.csv`) |
| `go run ./cmd/scanner [-all]` | Offline Ichimoku scan of latest candles | yes (instrument master) |
| `go run ./cmd/dbcheck [path]` | Inspect the DB | no |
| `go run ./scripts/<file>.go` | One-off tools (`//go:build ignore`); `gen_nifty500_embed.go` regenerates `internal/service/nifty500_symbols_embed.go` | yes |

Kite tokens expire daily (around 06:00 IST). With `SUPABASE_URL`/`SUPABASE_SERVICE_KEY` set, the engine and the offline tools take the token saved by the website's "Log in to Kite" button (`supabase.ResolveAccessToken`, `service.SessionWatcher`); without them, `KITE_ACCESS_TOKEN` is required. All settings are listed in `.env.example`.

## Architecture

**Engine wiring** (`cmd/engine/main.go`) is the single place strategies are registered and components are built. Startup order: config → SQLite → `MarketContext` → Discord → (with Supabase: heartbeat + wait for the website's Kite login) → Kite client → resolve token universe (`SCAN_NIFTY500` > `SCAN_FULL_NSE` > manual `INSTRUMENT_TOKENS`) → processor with strategies → `Bootloader.Boot` → delisted cleanup → cron → worker pool → `MultiTicker`. Shutdown runs in reverse.

**Data flow:**
- `service.Bootloader` fills `domain.MarketContext` (the RAM candle cache) from SQLite first and only calls the Kite historical API for gaps, rate-limited to about 3 req/s. Tokens that never return data go into `no_data_tokens` and are skipped on later boots.
- `scheduler` runs Boot at 08:45 and `EoDRollup` at 15:45 on weekdays. The rollup appends today's candle to SQLite and RAM, then **deletes rows older than `RETENTION_YEARS`**. Deepening history means raising `RETENTION_YEARS` *and* running `backfill -years N`; doing only one has no effect or gets purged.
- `service.MultiTicker` splits tokens across 3 WebSocket connections (Zerodha's per-key limit) and feeds `Processor.Enqueue`. Enqueue drops ticks when the channel is full.
- `service.Processor` workers update `MarketContext.Intraday` (live today-OHLC per token), then run every strategy inside `recover` via `strategy.EvaluateAll`. Each fired signal must pass `StateRegistry.TryLock(signal.StrategyName, token)` (one alert per signal name, per token, per IST calendar day; never released), then `dispatchSignal` saves it to Supabase and alerts Discord. A duplicate save (unique on signal, token, IST trade date) means it was already alerted, e.g. before a restart, so no alert. A failed save still alerts and is retried every minute. The processor's `enrich` fills `Strategy`, `Price` and `Direction` (`domain.DirectionOf`) for every signal.
- `service.SessionWatcher` polls Supabase every minute: a new current token rebuilds the feed (`LiveFeed`) and re-runs Boot; an expired one stops the feed; reminders go to Discord 08:00–15:30 on weekdays. `MultiTicker` sends at most one Discord error alert per connection per outage (`alertGate`). `main.runHeartbeat` writes `engine_status` every minute.

**Strategies** (`internal/strategy`) implement `Strategy{Name(); Evaluate(tick, *MarketContext)}`. Evaluate is called concurrently for every tick, so strategies must be stateless and must treat `ctx.Get()` output as their own copy. Key conventions:
- Each strategy branches on `ctx.Intraday.Get(token)`. When the market is open, today comes from the intraday cache and "yesterday" is the last stored candle; any stored candle dated today is stripped. When it is closed (no ticks today), the **last stored candle is treated as today**. The backtester depends on this: it feeds growing candle prefixes with an empty intraday cache, so strategy code needs no backtest-specific paths and has no look-ahead.
- One strategy emits several signal variants, told apart by `OrderSignal.StrategyName`, e.g. `"RSI (GFS)"`. Direction comes from that name via `domain.DirectionOf` (SELL/BEARISH/DEATH/SHORT means short), which the live engine, backtester and website all share, so keep that wording when adding bearish variants.
- A strategy that can fire several signals on one tick implements `MultiSignal.EvaluateAll` (RSI does: a weekly and a daily signal can land together). The processor and backtester always call `strategy.EvaluateAll(s, …)`, never `Evaluate` directly, so lower-priority signals aren't dropped.
- RSI's rules are pure functions over RSI values (`signalNames(rsiReadings)`, `rangeEventAt`, `turnedUpAt`/`turnedDownAt`) and are unit-tested there. The bull/bear range is re-derived from RSI history on every call, so the strategy keeps no per-stock state.
- `strategy.OnlySignals(inner, names...)` is an allowlist wrapper. The engine registers Ichimoku plus RSI allowlisted to `strategy.RSISignalNames()`.
- Adding a strategy means registering it in `cmd/engine/main.go` for live use **and** adding a catalogue entry with a warmup candle count in `resolveStrategies` in `cmd/backtest/main.go`.

**Backtest** (`internal/backtest`) opens the DB read-only (`mode=ro`), so it can run while the engine is live. Entry is the signal session's close, and horizons count trading sessions. The package covers stop rules (SMA, trailing, intraday, hard max-loss), confluence across strategies, and CSV/text reports written to `-out` (`backtest_out*/` directories hold earlier runs). `engine_test.go` holds detailed stop-fill semantics tests; update them when changing `measureLeg`/`runStop`.

**`cmd/scanner` reimplements the Ichimoku math on its own** instead of using `strategy.IchimokuKinkoHyo`. Changes to one do not carry over to the other.

## Data and conventions

- SQLite schema lives in `internal/repository/database.go`: `daily_candles(token, date, OHLCV)` keyed on `(token, date)`, plus `no_data_tokens`. WAL mode is on. `market_data.db` is about 300 MB and is the live data store, so don't delete it or rewrite it casually.
- All market-day logic uses IST (`Asia/Kolkata`, with a fixed +05:30 fallback).
- Log lines are prefixed with the component tag (`[processor]`, `[main]`, …). `logger.Init` writes stdout plus JSON lines to `logs/algotrader.log`.
- `config.Load` panics on missing required env vars (`KITE_API_KEY`, `DISCORD_WEBHOOK_URL`, and `KITE_ACCESS_TOKEN` only when Supabase isn't configured). The Discord bot variables are optional and unused.
- Server deployment (Oracle, systemd): `deploy/` and `docs/SETUP.md` step 7. After a push, `~/algotrader/deploy/update.sh` on the server pulls, tests, rebuilds and restarts.
- `.exe` files in the root are stale build artifacts, not sources of truth.

## Website (`web/`)

Next.js 16 on Vercel, which reads Supabase server-side only. See `web/README.md`.

- `cd web && npm test` (vitest), `npx tsc --noEmit`, `npm run lint`, `npm run build`. A single file: `npx vitest run src/lib/kite.test.ts`.
- Next 16: gating lives in `src/proxy.ts` (not `middleware.ts`), page `searchParams` is a Promise, and `cookies()` is async. Check `web/node_modules/next/dist/docs/` before using other Next APIs.
- There is no demo data. Every environment needs `SUPABASE_URL`/`SUPABASE_SERVICE_KEY`, and `src/app/error.tsx` explains what to check when Supabase can't be read.
- Signal names in `src/lib/catalog.ts` mirror the Go engine's. Update both together, and keep `directionOf` in step with `backtest.InferDirection`.
- The Kite `access_token` must never reach the browser. `SupabaseStore.status()` deliberately selects only `created_at`.
- Schema: `supabase/migrations/001_init.sql`. Setup steps: `docs/SETUP.md`.
