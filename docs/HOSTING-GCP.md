# Alternative: host the engine on Google Cloud (free e2-micro)

Use this instead of section 7 of `SETUP.md` (Oracle) if you prefer Google Cloud.
The steps are numbered to match.

The Go engine (`cmd/engine`) has to run all day: it holds a live connection
to Zerodha, runs the strategies on every tick, and runs the 08:45 and 15:45
jobs. Vercel can't do that, so it gets a small free Linux server from Google
Cloud. It only makes outgoing connections, so you don't open any ports.

Google's always-free server is an **e2-micro** (2 shared vCPUs, 1 GB RAM)
with a 30 GB standard disk. It's free only in three **US** regions. That puts
it about 200 ms from Zerodha, which doesn't matter for these daily-candle
signals.

## 7.1 Create the Google Cloud account and project

1. https://console.cloud.google.com → sign in with your Google account → accept
   the terms. If offered the **Free Trial** ($300 of credits for 90 days),
   accept it; a card is needed for verification.
2. Top bar → project picker → **New project** → name `algotrader` → **Create**,
   then make sure it's selected in the top bar.
3. ☰ menu → **Compute Engine → VM instances**. If asked, click **Enable** for
   the Compute Engine API and wait about a minute.

## 7.2 Set a budget alert first (2 minutes)

☰ → **Billing → Budgets & alerts → Create budget**: name `algotrader`, amount
**₹100** (or $1), keep the default email alerts at 50/90/100% → **Finish**.
If anything ever starts costing money, you get an email instead of a
surprise.

## 7.3 Create the server

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

## 7.4 Open a terminal on the server

In the VM instances list, click **SSH** on the `algotrader` row. A terminal
opens in a new browser window, with no keys to manage. Click **Authorize** if
asked.

The prompt looks like `yourname@algotrader:~$`. All commands until 7.9 run
here.

## 7.5 Prepare the server

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

## 7.6 Get the code (read-only access to your private repo)

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

## 7.7 Copy the candle database up (one time)

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

## 7.8 Create the engine's `.env` on the server

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

## 7.9 Run it as a service

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

## 7.10 The daily Kite token

Same as `SETUP.md` step 7.9: click **Log in to Kite** on the website each
trading morning, and the engine picks the token up within a minute. The
server's `.env` needs `SUPABASE_URL`, `SUPABASE_SERVICE_KEY` and `SITE_URL`, and
no `KITE_ACCESS_TOKEN` (see `SETUP.md` step 7.7).

## 7.11 Things to know about the free server

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
