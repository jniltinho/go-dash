#!/usr/bin/env bash
# Installs or upgrades Go Dash as the systemd service "go-dash" in /opt/go-dash.
# Run from the extracted release folder (or the repository) as root:
#
#   sudo ./install.sh [path/to/go-dash]
#
# Upgrades replace only the binary, the unit and the docs: settings
# (go-dash.env, dashboard.toml) and data (data/, backups/) are kept.
set -euo pipefail

PREFIX=/opt/go-dash
SERVICE=go-dash
UNIT=/etc/systemd/system/$SERVICE.service
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

die() { echo "install.sh: $*" >&2; exit 1; }

[[ $EUID -eq 0 ]] || die "run as root (sudo $0)"
command -v systemctl >/dev/null || die "systemd is required"

# The binary: argument, release folder, or repository build.
BIN="${1:-}"
if [[ -z $BIN ]]; then
  for candidate in "$HERE/go-dash" "$HERE/../bin/go-dash"; do
    [[ -x $candidate ]] && BIN=$candidate && break
  done
fi
[[ -n $BIN && -x $BIN ]] || die "go-dash binary not found (pass its path as the first argument)"

# Unit and env example: release folder layout, or repository layout.
find_file() {
  local name=$1
  for candidate in "$HERE/$name" "$HERE/systemd/$name" "$HERE/deploy/systemd/$name"; do
    [[ -f $candidate ]] && echo "$candidate" && return
  done
  die "$name not found next to install.sh"
}
UNIT_SRC=$(find_file go-dash.service)
ENV_SRC=$(find_file go-dash.env.example)
# A server package (make dist-server) ships a ready go-dash.env for the first install.
ENV_INITIAL=$ENV_SRC
[[ -f $HERE/go-dash.env ]] && ENV_INITIAL=$HERE/go-dash.env

echo "==> Version to install: $("$BIN" version)"

if ! id -u "$SERVICE" >/dev/null 2>&1; then
  echo "==> Creating system user $SERVICE"
  useradd --system --user-group --home-dir "$PREFIX" --no-create-home \
    --shell "$(command -v nologin || echo /usr/sbin/nologin)" "$SERVICE"
fi

echo "==> Installing into $PREFIX"
install -d -o root -g root -m 0755 "$PREFIX" "$PREFIX/bin"
install -d -o "$SERVICE" -g "$SERVICE" -m 0750 "$PREFIX/data" "$PREFIX/backups"

upgrading=false
if [[ -f $PREFIX/bin/go-dash ]]; then
  upgrading=true
  # Safety copy of the database before switching binaries (migrations may run);
  # works whether the service is running or stopped.
  if [[ -f $PREFIX/data/dashboard.db ]]; then
    stamp=$(date +%Y%m%d-%H%M%S)
    echo "==> Backing up the database to $PREFIX/backups/pre-upgrade-$stamp.db"
    runuser -u "$SERVICE" -- "$PREFIX/bin/go-dash" --db "$PREFIX/data/dashboard.db" \
      backup -o "$PREFIX/backups/pre-upgrade-$stamp.db"
  fi
fi
install -o root -g root -m 0755 "$BIN" "$PREFIX/bin/go-dash.new"
mv -f "$PREFIX/bin/go-dash.new" "$PREFIX/bin/go-dash"

if [[ ! -f $PREFIX/go-dash.env ]]; then
  echo "==> Creating $PREFIX/go-dash.env (edit it, then: systemctl restart $SERVICE)"
  install -o root -g "$SERVICE" -m 0640 "$ENV_INITIAL" "$PREFIX/go-dash.env"
fi
install -o root -g root -m 0644 "$ENV_SRC" "$PREFIX/go-dash.env.example"
for doc in README.md dashboard.toml.example menu.json; do
  [[ -f $HERE/$doc ]] && install -o root -g root -m 0644 "$HERE/$doc" "$PREFIX/$doc"
done

echo "==> Installing $UNIT"
install -o root -g root -m 0644 "$UNIT_SRC" "$UNIT"
systemctl daemon-reload

if $upgrading; then
  systemctl restart "$SERVICE"
else
  systemctl enable --now "$SERVICE"
fi

sleep 1
if systemctl is-active --quiet "$SERVICE"; then
  echo "==> $SERVICE is running: systemctl status $SERVICE / journalctl -u $SERVICE -f"
else
  journalctl -u "$SERVICE" -n 20 --no-pager >&2 || true
  die "$SERVICE did not start"
fi
