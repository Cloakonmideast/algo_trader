#!/usr/bin/env bash
# Pull the latest code, test, rebuild the engine and restart it.
# Run on the server: ~/algotrader/deploy/update.sh
set -euo pipefail
cd "$(dirname "$0")/.."
git pull --ff-only
go test ./internal/...
go build -o algotrader ./cmd/engine
sudo systemctl restart algotrader
sleep 3
systemctl --no-pager --lines=15 status algotrader
