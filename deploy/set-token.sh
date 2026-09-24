#!/usr/bin/env bash
# Interim, until the engine reads the Kite token from Supabase:
# store today's access token in .env and restart the engine.
# Usage on the server: ~/algotrader/deploy/set-token.sh <access_token>
set -euo pipefail
token="${1:?usage: set-token.sh <access_token>}"
env_file="$(dirname "$0")/../.env"
if grep -q '^KITE_ACCESS_TOKEN=' "$env_file"; then
  sed -i "s|^KITE_ACCESS_TOKEN=.*|KITE_ACCESS_TOKEN=${token}|" "$env_file"
else
  echo "KITE_ACCESS_TOKEN=${token}" >> "$env_file"
fi
sudo systemctl restart algotrader
echo "Token updated, engine restarting. Watch it with: journalctl -u algotrader -f"
