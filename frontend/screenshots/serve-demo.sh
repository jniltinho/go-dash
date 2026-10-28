#!/bin/sh
# Demo instance for README screenshots: example menu, product title, admin enabled.
set -eu
PORT="$1"
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
BIN="${DASHBOARD_BIN:-$ROOT/bin/go-dash}"
DATA="$(mktemp -d)"
export DASHBOARD_ADMIN_PASSWORD="screenshots-only"
exec "$BIN" --db "$DATA/demo.db" --log-level warn serve \
  --addr "127.0.0.1:$PORT" --title "Go Dash" --seed "$ROOT/examples/menu.json" --login-rate 100
