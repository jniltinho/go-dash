#!/usr/bin/env bash
# Builds the ready-to-install server package (called by `make dist-server`).
#
# Inputs (environment): RELEASE (binary name in DIST_DIR), DIST_DIR, VERSION,
# GOOS, GOARCH, SERVER_ADDR (e.g. 127.0.0.1:8585), optional ADMIN_PASSWORD
# (stored only as a bcrypt hash; needs python3-bcrypt or htpasswd) and
# optional MENU (menu export imported on the first install).
set -euo pipefail

: "${RELEASE:?}" "${DIST_DIR:?}" "${VERSION:?}" "${GOOS:?}" "${GOARCH:?}" "${SERVER_ADDR:?}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PKG="go-dash-$VERSION-$GOOS-$GOARCH"
STAGE="$DIST_DIR/stage-server/$PKG"

hash_password() {
  if python3 -c 'import bcrypt' 2>/dev/null; then
    python3 -c 'import bcrypt, os; print(bcrypt.hashpw(os.environ["ADMIN_PASSWORD"].encode(), bcrypt.gensalt(12)).decode())'
  elif command -v htpasswd >/dev/null; then
    htpasswd -niBC 12 "" <<<"$ADMIN_PASSWORD" | tr -d ':\n'
  else
    echo "build.sh: python3-bcrypt or htpasswd is needed to hash ADMIN_PASSWORD" >&2
    exit 1
  fi
}

rm -rf "$DIST_DIR/stage-server" && mkdir -p "$STAGE"
install -m 0755 "$DIST_DIR/$RELEASE" "$STAGE/go-dash"
install -m 0755 "$ROOT/deploy/install.sh" "$STAGE/install.sh"
install -m 0644 "$ROOT/deploy/systemd/go-dash.service" "$ROOT/deploy/systemd/go-dash.env.example" \
  "$ROOT/deploy/server/Makefile" "$ROOT/dashboard.toml.example" "$STAGE/"
install -m 0644 "$ROOT/docs/install.md" "$STAGE/INSTALL.md"
sed -e "s|__ADDR__|$SERVER_ADDR|g" -e "s|__ADDR_PORT__|${SERVER_ADDR##*:}|g" \
  "$ROOT/deploy/server/README.md" > "$STAGE/README.md"

# Ready settings: listen address, trust nginx, admin password hash.
hash=""
[[ -n ${ADMIN_PASSWORD:-} ]] && hash=$(hash_password)
sed -e "s|^DASHBOARD_ADDR=.*|DASHBOARD_ADDR=$SERVER_ADDR|" \
  -e "s|^#DASHBOARD_TRUST_PROXY=.*|DASHBOARD_TRUST_PROXY=true|" \
  -e "s|^DASHBOARD_ADMIN_PASSWORD_HASH=.*|DASHBOARD_ADMIN_PASSWORD_HASH='$hash'|" \
  "$ROOT/deploy/systemd/go-dash.env.example" > "$STAGE/go-dash.env"
chmod 0600 "$STAGE/go-dash.env"

if [[ -n ${MENU:-} ]]; then
  install -m 0644 "$MENU" "$STAGE/menu.json"
fi

tar -czf "$DIST_DIR/$PKG.tar.gz" -C "$DIST_DIR/stage-server" "$PKG"
rm -rf "$DIST_DIR/stage-server"
(cd "$DIST_DIR" && sha256sum "$PKG.tar.gz" > "$PKG.sha256")
pw="not set"
[[ -n $hash ]] && pw="set (bcrypt)"
echo "==> $DIST_DIR/$PKG.tar.gz: listens on $SERVER_ADDR, admin password $pw, menu ${MENU:-none}"
