# Setup guide: GitHub → Supabase → Vercel → Kite

Follow these steps once, in order. They take about 30–45 minutes. Commands are
for **PowerShell** on this PC, run from `C:\vsc\algotrader`.

What you end up with:

- The code in a **private** GitHub repository.
- A **Supabase** database holding signals and the daily Kite token.
- The website on **Vercel** at `https://<name>.vercel.app`, locked behind your
  passcode. It redeploys automatically on every push.
- A **Log in to Kite** button that saves each day's access token.

> The trading engine runs separately, on a free Google Cloud server (step 7). Until
> the engine update that connects it to Supabase, the engine keeps sending
> Discord alerts only. The site works (Kite login, test signal in step 6), but
> real signals don't reach it yet, and the header shows "Engine never seen".

---

## 0. Accounts you need

| Account | Where | Notes |
|---|---|---|
| GitHub | https://github.com/signup | Free |
| Supabase | https://supabase.com | Free plan; sign in with GitHub |
| Vercel | https://vercel.com/signup | Free "Hobby" plan (personal use); sign in **with GitHub** |
| Kite Connect app | https://developers.kite.trade | The app whose API key and secret are in your `.env` |
| Google Cloud | https://console.cloud.google.com | For the engine server (step 7); free e2-micro, card needed for verification |

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

## 7. Host the engine on Google Cloud (free e2-micro)

The Go engine (`cmd/engine`) has to run all day: it holds a live connection
to Zerodha, runs the strategies on every tick, and runs the 08:45 and 15:45
jobs. Vercel can't do that, so it gets a small free Linux server from Google
Cloud. It only makes outgoing connections, so you don't open any ports.

Google's always-free server is an **e2-micro** (2 shared vCPUs, 1 GB RAM)
with a 30 GB standard disk. It's free only in three **US** regions. That puts
it about 200 ms from Zerodha, which doesn't matter for these daily-candle
signals.

### 7.1 Create the Google Cloud account and project

1. https://console.cloud.google.com → sign in with your Google account → accept
   the terms. If offered the **Free Trial** ($300 of credits for 90 days),
   accept it; a card is needed for verification.
2. Top bar → project picker → **New project** → name `algotrader` → **Create**,
   then make sure it's selected in the top bar.
3. ☰ menu → **Compute Engine → VM instances**. If asked, click **Enable** for
   the Compute Engine API and wait about a minute.

### 7.2 Set a budget alert first (2 minutes)

☰ → **Billing → Budgets & alerts → Create budget**: name `algotrader`, amount
**₹100** (or $1), keep the default email alerts at 50/90/100% → **Finish**.
If anything ever starts costing money, you get an email instead of a
surprise.

### 7.3 Create the server

**Compute Engine → VM instances → Create instance**, then set exactly:

| Setting | Value |
|---|---|
| **Name** | `algotrader` |
| **Region** | **us-central1 (Iowa)**. Only `us-central1`, `us-east1` and `us-west1` are free. |
| **Zone** | any |
| **Machine configuration** | **E2** series → Machine type **e2-micro** |
| **OS and storage → Change** | Operating system **Ubuntu**, version **Ubuntu 24.04 LTS** (x86/64), boot disk type **Standard persistent disk** (⚠️ not "Balanced", which isn't free), size **30 GB** → **Select** |
| **Networking → Firewall** | leave **Allow HTTP** and **Allow HTTPS** unticked |

Everything else stays at the defaults. Click **Create**. The right-hand panel
should show an estimate near zero for the e2-micro; if it shows a monthly
price, re-check the region, machine type and disk type.

After about a minute the instance shows a green tick.

> Google may bill a small amount for the server's external IPv4 address
> (roughly $3/month) depending on your account's free-tier terms. The budget
> alert from 7.2 will tell you. The engine needs outgoing internet, so keep
> the external IP.

### 7.4 Open a terminal on the server

In the VM instances list, click **SSH** on the `algotrader` row. A terminal
opens in a new browser window, with no keys to manage. Click **Authorize** if
asked.

The prompt looks like `yourname@algotrader:~$`. All commands until 7.9 run
here.

### 7.5 Prepare the server

Paste these one block at a time:

```bash
sudo timedatectl set-timezone Asia/Kolkata
sudo apt update && sudo apt -y upgrade
sudo apt -y install build-essential git sqlite3 unzip
```

Add 2 GB of swap. With only 1 GB of RAM, the first build can run out of memory
without it:

```bash
sudo fallocate -l 2G /swapfile && sudo chmod 600 /swapfile
sudo mkswap /swapfile && sudo swapon /swapfile
echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
```

Install Go (this server is x86-64, so it's the **amd64** download):

```bash
curl -LO https://go.dev/dl/go1.25.5.linux-amd64.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.25.5.linux-amd64.tar.gz
rm go1.25.5.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc && source ~/.bashrc
go version     # go version go1.25.5 linux/amd64
date           # must show IST
free -h        # Swap: 2.0Gi
```

The timezone matters: the engine's 08:45 and 15:45 jobs use the server
clock.

### 7.6 Get the code (read-only access to your private repo)

```bash
ssh-keygen -t ed25519 -f ~/.ssh/github_deploy -N ""
cat ~/.ssh/github_deploy.pub
```

Copy the line it prints. On GitHub: your repo → **Settings → Deploy keys →
Add deploy key**. Title `gcp`, paste the key, leave **Allow write access**
unticked → **Add key**. Then on the server:

```bash
printf 'Host github.com\n  IdentityFile ~/.ssh/github_deploy\n' >> ~/.ssh/config
git clone git@github.com:<your-username>/algotrader.git ~/algotrader
cd ~/algotrader
go test ./internal/...
go build -o algotrader ./cmd/engine
```

Type `yes` when asked about github.com's fingerprint. The first build compiles
SQLite from C and takes **5–10 minutes** on this small server; later builds
are much faster. The tests should print `ok`, and the build should leave
`~/algotrader/algotrader`.

### 7.7 Copy the candle database up (one time)

**On this PC**, stop the engine first if it's running here (so the file isn't
mid-write), then zip the database in PowerShell:

```powershell
Compress-Archive -Path C:\vsc\algotrader\market_data.db -DestinationPath $HOME\Desktop\market_data.zip -Force
```

In the **browser SSH window**: click the **Upload file** button (the up-arrow
icon at the top right) → choose `market_data.zip` from your Desktop. It lands
in your home folder. Then:

```bash
unzip -o ~/market_data.zip -d ~/algotrader && rm ~/market_data.zip
ls -lh ~/algotrader/market_data.db     # about 290M
```

### 7.8 Create the engine's `.env` on the server

```bash
nano ~/algotrader/.env
```

Paste the contents of your PC's `C:\vsc\algotrader\.env`, then fix two things:

- **Remove inline comments.** The service reads `KEY=value # note` as the
  value `value # note`. Your `KITE_ACCESS_TOKEN=... # Generated daily...`
  line must become just `KITE_ACCESS_TOKEN=...`. Comments on their own lines
  are fine.
- `DATABASE_PATH=market_data.db` (the file you just uploaded).

Save with `Ctrl+O`, `Enter`, `Ctrl+X`. Then lock the file down:

```bash
chmod 600 ~/algotrader/.env
```

### 7.9 Run it as a service

```bash
sed "s/__USER__/$USER/g" ~/algotrader/deploy/algotrader.service | sudo tee /etc/systemd/system/algotrader.service
sudo systemctl daemon-reload
sudo systemctl enable --now algotrader
journalctl -u algotrader -f      # live log; Ctrl+C stops watching (the engine keeps running)
```

You should see the startup banner, "Boot complete", and a Discord
"AlgoTrader Online" message. The service starts on boot and restarts itself
if it crashes.

Useful commands (in the browser SSH window):

| Command | What it does |
|---|---|
| `systemctl status algotrader` | Is it running? |
| `journalctl -u algotrader -f` | Live log |
| `journalctl -u algotrader --since today` | Today's log |
| `sudo systemctl restart algotrader` | Restart |
| `~/algotrader/deploy/update.sh` | After a `git push`: pull, test, rebuild, restart |
| `top` | CPU/memory use, which is worth a look at 09:15–09:30 on the first days |

### 7.10 The daily Kite token (until the engine update)

The engine still reads `KITE_ACCESS_TOKEN` from `.env`. Each trading morning,
generate a token the old way (the PowerShell snippet in `readme`), then open
the browser SSH window and run:

```bash
~/algotrader/deploy/set-token.sh <NEW_ACCESS_TOKEN>
```

This writes the token into `.env` and restarts the engine, which then loads
data and connects. After the engine update this step goes away: the engine
will take the token from **Log in to Kite** on the website.

### 7.11 Things to know about the free server

- **It's small.** The e2-micro has a guaranteed quarter of a CPU and can burst
  above that. At the 09:15 open, when ticks arrive fastest, the engine may
  briefly fall behind; the tick buffer absorbs that, and signals are
  re-checked on every later tick. If `top` shows the engine pinned at high
  CPU for long stretches, tell me. The engine can be tuned to evaluate each
  stock less often.
- **Free-tier limits:** one e2-micro per account, 30 GB standard disk, and
  1 GB a month of outgoing traffic. The engine mostly receives data (free)
  and sends little.
- **When the 90-day trial ends**, Google asks you to activate a full (paid)
  billing account. Do it to keep the server running; the e2-micro stays
  free within the limits above, and the budget alert covers the rest.

---

## Daily routine

Each trading morning: open the site on any device → **Log in to Kite** →
Zerodha password + TOTP. One login covers every device. Until the engine
update, also push that day's token to the server (step 7.10).

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
