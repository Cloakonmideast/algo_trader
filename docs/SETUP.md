# Setup guide: GitHub → Supabase → Vercel → Kite

Follow these steps once, in order. They take about 30–45 minutes. Commands are
for **PowerShell** on this PC, run from `C:\vsc\algotrader`.

What you end up with:

- The code in a **private** GitHub repository.
- A **Supabase** database holding signals and the daily Kite token.
- The website on **Vercel** at `https://<name>.vercel.app`, locked behind your
  passcode. It redeploys automatically on every push.
- A **Log in to Kite** button that saves each day's access token.

> The trading engine runs separately, on a free Oracle Cloud server (step 7).
> It saves every signal to Supabase (so it appears on the site within 30
> seconds), sends the Discord alert, and takes the daily Kite token from the
> site's **Log in to Kite** button.

---

## 0. Accounts you need

| Account | Where | Notes |
|---|---|---|
| GitHub | https://github.com/signup | Free |
| Supabase | https://supabase.com | Free plan; sign in with GitHub |
| Vercel | https://vercel.com/signup | Free "Hobby" plan (personal use); sign in **with GitHub** |
| Kite Connect app | https://developers.kite.trade | The app whose API key and secret are in your `.env` |
| Oracle Cloud | https://cloud.oracle.com | For the engine server (step 7); Always Free, home region Hyderabad |

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

## 7. Host the engine on Oracle Cloud (Always Free)

The Go engine (`cmd/engine`) has to run all day: it holds a live connection
to Zerodha, runs the strategies on every tick, and runs the 08:45 and 15:45
jobs. Vercel can't do that, so it runs on a free Oracle Cloud server in your
home region (Hyderabad). It only makes outgoing connections, so you don't
open any ports. (Prefer Google Cloud? See `docs/HOSTING-GCP.md`.)

### 7.1 Create the server

Console → ☰ → **Compute → Instances → Create instance**:

| Setting | Value |
|---|---|
| **Name** | `algotrader` |
| **Image** | **Canonical Ubuntu 24.04** |
| **Shape** | **Ampere VM.Standard.A1.Flex**, 1–2 OCPUs / 6–12 GB (Always Free). If it's out of capacity: retry later, or use **VM.Standard.E2.1.Micro** (also free, but much smaller). |
| **Security** | Shielded instance **off**, confidential computing **off** |
| **Networking** | **Create new virtual cloud network** + **Create new public subnet**; **Automatically assign public IPv4 address** **on** |
| **SSH keys** | **Generate a key pair for me** → **Save private key** (and the public key) |
| **Boot volume** | defaults (46.6 GB, in-transit encryption on) |

**Create**, wait for **Running**, then copy the **Public IP address** from the
instance page.

### 7.2 Put the SSH key in place (on this PC, once)

In PowerShell (use your downloaded file's name):

```powershell
mkdir $HOME\.ssh -Force
Move-Item $HOME\Downloads\ssh-key-2026-09-24.key $HOME\.ssh\oracle-algotrader.key
$me = [System.Security.Principal.WindowsIdentity]::GetCurrent().Name
icacls $HOME\.ssh\oracle-algotrader.key /inheritance:r /grant:r "${me}:(R)"
```

### 7.3 Connect

```powershell
ssh -i $HOME\.ssh\oracle-algotrader.key ubuntu@<PUBLIC_IP>
```

Type `yes` the first time. When the prompt shows `ubuntu@algotrader:~$`, you
are on the server. The steps up to 7.8 run there unless marked "on this PC".

### 7.4 Prepare the server

```bash
sudo timedatectl set-timezone Asia/Kolkata
sudo apt update && sudo apt -y upgrade
sudo apt -y install build-essential git sqlite3
```

Add 2 GB of swap. It matters on the small E2 micro, and does no harm on A1:

```bash
sudo fallocate -l 2G /swapfile && sudo chmod 600 /swapfile
sudo mkswap /swapfile && sudo swapon /swapfile
echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
```

Install Go. This detects arm64 (A1) or amd64 (E2 micro) automatically:

```bash
ARCH=$(dpkg --print-architecture)
curl -LO https://go.dev/dl/go1.25.5.linux-$ARCH.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.25.5.linux-$ARCH.tar.gz
rm go1.25.5.linux-$ARCH.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc && source ~/.bashrc
go version     # go version go1.25.5 linux/arm64 (or amd64)
date           # must show IST
```

The timezone matters: the engine's 08:45 and 15:45 jobs use the server
clock.

### 7.5 Get the code (read-only access to your private repo)

```bash
ssh-keygen -t ed25519 -f ~/.ssh/github_deploy -N ""
cat ~/.ssh/github_deploy.pub
```

Copy the line it prints. On GitHub: your repo → **Settings → Deploy keys →
Add deploy key**. Title `oracle`, paste the key, leave **Allow write access**
unticked → **Add key**. Then on the server:

```bash
printf 'Host github.com\n  IdentityFile ~/.ssh/github_deploy\n' >> ~/.ssh/config
git clone git@github.com:<your-username>/algotrader.git ~/algotrader
cd ~/algotrader
go test ./internal/...
go build -o algotrader ./cmd/engine
```

Type `yes` when asked about github.com's fingerprint. The first build compiles
SQLite from C and takes a few minutes. The tests should print `ok`, and the
build should leave `~/algotrader/algotrader`.

### 7.6 Copy the candle database up (one time)

**On this PC**, stop the engine first if it's running here (so the file isn't
mid-write), then in a **new** PowerShell window:

```powershell
scp -i $HOME\.ssh\oracle-algotrader.key C:\vsc\algotrader\market_data.db ubuntu@<PUBLIC_IP>:/home/ubuntu/algotrader/
```

That's about 300 MB, so it takes a few minutes. Back on the server,
`ls -lh ~/algotrader/market_data.db` should show about 290M.

### 7.7 Create the engine's `.env` on the server

```bash
nano ~/algotrader/.env
```

Paste the contents of your PC's `C:\vsc\algotrader\.env` (or copy it up with
`scp` if pasting is awkward), then make these changes:

- **Add the three website settings** at the end, with the same Supabase values
  you gave Vercel in step 4.3:

  ```bash
  SUPABASE_URL=https://<project-id>.supabase.co
  SUPABASE_SERVICE_KEY=<the secret / service_role key>
  SITE_URL=https://<your-site>.vercel.app
  ```

- **Delete the `KITE_ACCESS_TOKEN=...` line.** With Supabase set, the engine
  takes the token from the website's login instead.
- **No inline comments.** The service reads `KEY=value # note` as the value
  `value # note`. Comments on their own lines are fine.
- `DATABASE_PATH=market_data.db` (the file you just copied).

Save with `Ctrl+O`, `Enter`, `Ctrl+X`. Then lock the file down:

```bash
chmod 600 ~/algotrader/.env
```

### 7.8 Run it as a service

```bash
sed "s/__USER__/$USER/g" ~/algotrader/deploy/algotrader.service | sudo tee /etc/systemd/system/algotrader.service
sudo systemctl daemon-reload
sudo systemctl enable --now algotrader
journalctl -u algotrader -f      # live log; Ctrl+C stops watching (the engine keeps running)
```

If you haven't logged in to Kite today, the log says **"Waiting for today's
Kite login"**. Click **Log in to Kite** on the website; within a minute the
engine continues. You then see "Boot complete" and a Discord "AlgoTrader
Online" message, and the website header shows **Engine just now**. The service starts on boot and restarts itself
if it crashes.

Useful commands:

| Command | What it does |
|---|---|
| `systemctl status algotrader` | Is it running? |
| `journalctl -u algotrader -f` | Live log |
| `journalctl -u algotrader --since today` | Today's log |
| `sudo systemctl restart algotrader` | Restart |
| `~/algotrader/deploy/update.sh` | After a `git push`: pull, test, rebuild, restart |
| `top` | CPU/memory use |

### 7.9 How the daily Kite token works

Nothing to do on the server. Each trading morning, click **Log in to Kite** on
the website. Every minute the engine checks Supabase for that login:

- **New token:** it reconnects the live feed and refreshes candle data.
  Discord says "Kite token updated".
- **Token expired** (Zerodha expires them around 06:00): it stops the live
  feed, so Discord isn't flooded with connection errors, and says "Kite
  session expired".
- **No login yet between 08:00 and 15:30 on a weekday:** Discord reminds you
  every 30 minutes, with a link to the site.

Logging in again during the day is fine; the engine switches to the new token.

### 7.9a Upgrading a server that runs an older engine

If your server was set up before the engine saved signals to Supabase (its
`.env` has `KITE_ACCESS_TOKEN` and no `SUPABASE_URL`):

1. On this PC, push the latest code (`git add -A; git commit -m "..."; git push`).
2. On the server, add the three settings from 7.7 and delete the
   `KITE_ACCESS_TOKEN` line:

   ```bash
   nano ~/algotrader/.env
   ```

3. Pull, test, rebuild and restart:

   ```bash
   ~/algotrader/deploy/update.sh
   ```

4. Log in to Kite on the website if you haven't today, then watch
   `journalctl -u algotrader -f`. The website header should show **Engine just
   now** within a minute.

### 7.10 Keep the server from being reclaimed

Oracle may reclaim an Always Free server that stays almost completely idle
for 7 days (very low CPU, network and memory use). The engine is light, so
this can happen. To avoid it, upgrade the account to **Pay As You Go**
(Billing → Upgrade). Always Free resources stay free, and Oracle doesn't
reclaim idle servers on paid accounts. Add a small **budget alert** (Billing
→ Budgets) so anything that ever costs money emails you. Check Oracle's
current Always Free terms when you do this.

If it's ever reclaimed, your signal history is safe in Supabase. You redo
7.1–7.8 and copy up the database again.

---

## Daily routine

Each trading morning: open the site on any device → **Log in to Kite** →
Zerodha password + TOTP. One login covers every device and the engine on the
server (step 7.9).

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
