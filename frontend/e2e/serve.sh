#!/bin/sh
# Starts a dashboard instance for e2e tests with a fresh, seeded database.
# Usage: serve.sh <port> <with-password: yes|no> [default-theme]
set -eu
PORT="$1"
WITH_PASSWORD="$2"
# Without a third argument the server keeps its built-in default theme.
THEME_ARGS=""
if [ -n "${3:-}" ]; then
  THEME_ARGS="--default-theme $3"
fi
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
BIN="${DASHBOARD_BIN:-$ROOT/bin/go-dash}"
DATA="$(mktemp -d)"
DB="$DATA/e2e.db"

"$BIN" --db "$DB" menu import "$ROOT/examples/menu.json" >/dev/null
"$BIN" --db "$DB" menu import "$ROOT/frontend/e2e/fixtures.json" >/dev/null

if [ "$WITH_PASSWORD" = "yes" ]; then
  export DASHBOARD_ADMIN_PASSWORD="e2e-password-123"
else
  unset DASHBOARD_ADMIN_PASSWORD DASHBOARD_ADMIN_PASSWORD_HASH
fi

# Many browser tests log in; the rate limit itself is covered by Go and e2e tests.
# E2E_BASE_PATH serves the instance under a prefix (base-path.spec.ts).
BASE_ARGS=""
if [ -n "${E2E_BASE_PATH:-}" ]; then
  BASE_ARGS="--base-path $E2E_BASE_PATH"
fi

exec "$BIN" --db "$DB" --log-level warn serve --addr "127.0.0.1:$PORT" --max-tabs 3 --login-rate "${E2E_LOGIN_RATE:-200}" $THEME_ARGS $BASE_ARGS
