# Setup guide: GitHub → Supabase → Vercel → Kite

Follow these steps once, in order. They take about 30–45 minutes. Commands are
for **PowerShell** on this PC, run from `C:\vsc\algotrader`.

What you end up with:

- The code in a **private** GitHub repository.
- A **Supabase** database holding signals and the daily Kite token.
- The website on **Vercel** at `https://<name>.vercel.app`, locked behind your
  passcode. It redeploys automatically on every push.
- A **Log in to Kite** button that saves each day's access token.

> The trading engine is not hosted yet. It gets its own section (Oracle Cloud)
> with the engine update. Until then the site works, and you can log in to
> Kite and test it with a sample signal (step 6), but no real signals arrive
> and the header shows "Engine never seen".

---

## 0. Accounts you need

| Account | Where | Notes |
|---|---|---|
| GitHub | https://github.com/signup | Free |
| Supabase | https://supabase.com | Free plan; sign in with GitHub |
| Vercel | https://vercel.com/signup | Free "Hobby" plan (personal use); sign in **with GitHub** |
| Kite Connect app | https://developers.kite.trade | The app whose API key and secret are in your `.env` |

You also need **Git** (already installed) and **Node.js 20+** (you have 24).

---

## 1. Put the code on GitHub

### 1.1 Create an empty private repository

1. Go to https://github.com/new.
2. **Repository name**: `algotrader` (or anything you like).
3. Choose **Private**.
4. Leave **Add a README**, **.gitignore** and **license** all **unticked**. The
   repo must start empty.
5. Click **Create repository** and keep the page open; you need its URL, e.g.
   `https://github.com/<your-username>/algotrader.git`.

### 1.2 Check what will be uploaded

```powershell
cd C:\vsc\algotrader
git status --short
```

You should see the source code, `web/`, `supabase/`, `docs/` and a few config
files. You must **not** see `.env`, `web/.env.local`, `market_data.db`, any
`.exe`, `logs/`, `node_modules/` or `INTERVIEW_PREP.txt`. They are all in
`.gitignore`. If one of them shows up, stop and fix `.gitignore` first.

`package.json` / `package-lock.json` at the repo root only hold Playwright (a
browser-testing tool). Uploading them is harmless. To leave them out, add
both names to `.gitignore`.

### 1.3 Commit and push

```powershell
git add -A
git commit -m "Signals website, RSI range signals, setup docs"
git remote add origin https://github.com/<your-username>/algotrader.git
git push -u origin main
```

- The first push opens a browser window asking you to sign in to GitHub (Git
  Credential Manager). Approve it; later pushes won't ask again.
- "LF will be replaced by CRLF" warnings are harmless.

### 1.4 Verify on GitHub

Open the repo page and check that `web/`, `supabase/` and `docs/` are there,
and that **`.env` and `market_data.db` are not**. The repo should say
**Private** next to its name.

---

## 2. Create the Supabase database

### 2.1 Create the project

1. https://supabase.com/dashboard → **New project**.
2. **Name**: `algotrader`.
3. **Database password**: click **Generate**, then save it in your password
   manager. The site doesn't use it, but you need it for any future database
   work.
4. **Region**: **South Asia (Mumbai)**.
5. **Create new project**, then wait about 2 minutes while it starts.

### 2.2 Create the tables

1. Left sidebar → **SQL Editor** → **New query**.
2. Open `supabase/migrations/001_init.sql` from this repo, copy **all** of it,
   and paste it into the editor.
3. Click **Run**. You should see "Success. No rows returned".
4. Left sidebar → **Table Editor**. You should see `signals`, `kite_session`
   and `engine_status`, each without a red "RLS disabled" warning.

### 2.3 Copy the two values the website needs

Left sidebar → **Project Settings** (gear icon):

- **Data API** (or **API**) → **Project URL**, e.g.
  `https://abcdefghijkl.supabase.co`. This is `SUPABASE_URL`.
- **API Keys** → the **Secret key** (starts with `sb_secret_…`). If your
  project only shows **Legacy API keys**, use the **`service_role`** key
  instead. This is `SUPABASE_SERVICE_KEY`.

> ⚠️ The secret / service_role key can read and write everything. Put it only
> in Vercel and in your local `web/.env.local`, never in code, chat or git.
> Do **not** use the `anon` / publishable key: it can't read anything,
> because the tables deny it on purpose.

---

## 3. (Recommended) Try the site locally against Supabase

This catches wrong keys before you deploy.

1. Create `web/.env.local` if it doesn't exist (`git` ignores it) and fill it
   in:

   ```bash
   SUPABASE_URL=https://abcdefghijkl.supabase.co
   SUPABASE_SERVICE_KEY=sb_secret_...
   SITE_PASSCODE=<a passcode you'll remember>
   COOKIE_SECRET=<long random string, see 4.3>
   KITE_API_KEY=<from your .env>
   KITE_API_SECRET=<from your .env>
   KITE_USER_ID=<your Zerodha client ID, e.g. AB1234>
   ```

2. Start it:

   ```powershell
   cd C:\vsc\algotrader\web
   npm install
   npm run dev
   ```

3. Open http://localhost:3000, enter your passcode, and you should see an
   empty dashboard ("No signals yet today"). If you see **"Couldn't load
   signals"**, the red box names the problem, usually a wrong URL or key, or
   the SQL from step 2.2 not run.

(The **Log in to Kite** button only works on the deployed site, because the
Kite redirect URL points there. See step 5.)

Press `Ctrl+C` to stop the dev server.

---

## 4. Deploy the website on Vercel

### 4.1 Import the repository

1. https://vercel.com/new. If asked, **Install** the Vercel GitHub app and
   give it access to the `algotrader` repo (choosing "Only select
   repositories" is fine).
2. Find `algotrader` in the list → **Import**.

### 4.2 Configure the project

On the "Configure Project" screen:

1. **Root Directory** → **Edit** → select **`web`** → **Continue**. This is
   the important one: the website lives in `web/`, not the repo root.
2. **Framework Preset**: should now say **Next.js**.
3. Leave the Build and Output settings as they are.

### 4.3 Add the environment variables

Expand **Environment Variables** and add these seven. Name on the left,
value on the right:

| Name | Value |
|---|---|
| `SUPABASE_URL` | Project URL from 2.3 |
| `SUPABASE_SERVICE_KEY` | Secret / service_role key from 2.3 |
| `SITE_PASSCODE` | The passcode you'll type once per device |
| `COOKIE_SECRET` | A long random string (see below) |
| `KITE_API_KEY` | Your Kite Connect API key (same as in `.env`) |
| `KITE_API_SECRET` | Your Kite Connect API secret (same as in `.env`) |
| `KITE_USER_ID` | Your Zerodha client ID, e.g. `AB1234` |

To generate `COOKIE_SECRET`, run this in PowerShell and paste the output:

```powershell
$b = New-Object byte[] 32; [Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($b); [Convert]::ToBase64String($b)
```

Changing `COOKIE_SECRET` later locks every device out; each one then needs
the passcode again. That's the way to revoke a lost phone.

### 4.4 Deploy

1. Click **Deploy** and wait about 1–2 minutes.
2. When it says "Congratulations", click **Continue to Dashboard**.
3. Note the domain shown, e.g. **`algotrader-abc123.vercel.app`**. That's your
   site. You can rename it under **Settings → Domains**; if you do, use the
   new name in step 5.

If you added or changed an environment variable after deploying:
**Deployments** → the latest one → **⋯** → **Redeploy**. Variables only take
effect in a new deployment.

---

## 5. Point Kite at the site

1. https://developers.kite.trade → **My apps** → your app.
2. **Redirect URL**: `https://<your-domain>.vercel.app/api/kite/callback`,
   with your exact domain from 4.4, starting `https://` and ending
   `/api/kite/callback`.
3. **Save**.

This replaces the old manual PowerShell token flow in `readme`. From now on,
the button on the site handles the token.

---

## 6. Check everything works

1. **Passcode:** open `https://<your-domain>.vercel.app`. You're sent to the
   unlock page; enter your passcode. You land on an empty dashboard.
2. **Kite login:** click **Log in to Kite** (top right). On Zerodha's page,
   enter your user ID, password and TOTP. You come back to the site with
   **"✓ Kite token saved"**, and the header shows **"✓ Kite token valid"**.
   - In Supabase → Table Editor → `kite_session` there is now one row with
     today's time.
3. **A test signal:** in Supabase → SQL Editor, run:

   ```sql
   insert into signals (fired_at, trade_date, token, symbol, strategy, signal, direction, price, rsi_daily, rsi_weekly, rsi_monthly)
   values (now(), (now() at time zone 'Asia/Kolkata')::date, 408065, 'INFY', 'RSI',
           'RSI (Daily Strong Bullish)', 'long', 1523.40, 62.1, 64.3, 66.0);
   ```

   Within 30 seconds it appears on the dashboard without a reload. Then check:
   - **Strategies** → **RSI** → **Daily Strong Bullish** shows it.
   - **Export today** downloads an `.xlsx` with it.
4. **Clean up:** remove the test row.

   ```sql
   delete from signals where symbol = 'INFY' and signal = 'RSI (Daily Strong Bullish)'
     and trade_date = (now() at time zone 'Asia/Kolkata')::date;
   ```

5. **Phone:** open the site on your phone and enter the passcode once. It's
   remembered for a year on that device.

---

## Daily routine

Each trading morning: open the site on any device → **Log in to Kite** →
Zerodha password + TOTP. One login covers every device and, once it's
deployed, the engine.

## Updating the site later

```powershell
cd C:\vsc\algotrader
git add -A
git commit -m "Describe the change"
git push
```

Vercel rebuilds and redeploys automatically in about 1–2 minutes. Watch it
under your project → **Deployments**.

## Troubleshooting

| Symptom | Fix |
|---|---|
| **"Couldn't load signals"** on every page | Vercel → Settings → Environment Variables: check `SUPABASE_URL` and `SUPABASE_SERVICE_KEY` (the secret/service_role key, not anon), then **Redeploy**. Also check you ran step 2.2. The error reference on the page matches an entry in Vercel → **Logs**. |
| Vercel build fails with "No Next.js version detected" | **Root Directory** isn't `web`. Settings → General → Root Directory → `web` → Save → Redeploy. |
| Unlock page says **"no COOKIE_SECRET configured"** | Add `COOKIE_SECRET` in Vercel and redeploy. |
| Passcode rejected | `SITE_PASSCODE` in Vercel is exactly what you type (case-sensitive, no extra spaces). Redeploy after changing it. |
| Zerodha shows an error about the redirect URL | The Redirect URL in the Kite developer console must exactly match `https://<domain>/api/kite/callback`. |
| **"Kite login failed: That login was for …"** | `KITE_USER_ID` doesn't match the Zerodha account you logged in with. |
| **"Kite login failed: Token is invalid or has expired"** | The login link was used twice or was too old. Click **Log in to Kite** again. |
| Supabase project **paused** | Free projects pause after about a week with no activity. Open the Supabase dashboard → **Restore**. Once the engine runs, its heartbeat keeps the project awake. |
