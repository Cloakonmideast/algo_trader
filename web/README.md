# AlgoTrader Signals — website

Next.js 16 site for viewing and exporting the engine's signals. Deployed on
Vercel with root directory `web/`.

## Local development

```bash
cd web
npm install
cp .env.example .env.local   # fill in every value (see docs/SETUP.md)
npm run dev                  # http://localhost:3000
```

The site always reads your Supabase project, locally too, so fill in
`SUPABASE_URL` and `SUPABASE_SERVICE_KEY` in `.env.local` (see `docs/SETUP.md`).
Without them every page shows a "Couldn't load signals" message naming the
missing settings.

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
- Trades (the `/trades` tab and the Buy/Sell buttons on signal rows) live in
  the `trades` table (`supabase/migrations/002_trades.sql`). Pages read them
  through the store; `POST /api/trades` and `DELETE /api/trades/[id]` write.
