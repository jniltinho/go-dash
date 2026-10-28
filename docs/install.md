# Installing Go Dash on Linux

Two supported ways to run it on a server. Pick one:

| | [A. Binary + systemd](#a-binary--systemd) | [B. Docker + Docker Compose](#b-docker--docker-compose) |
| --- | --- | --- |
| Needs | any Linux with systemd (amd64 or arm64) | Docker Engine + Compose plugin |
| Lives in | `/opt/go-dash` (service `go-dash`) | container `go-dash`, volume `dashboard-data` |
| Database | `/opt/go-dash/data/dashboard.db` | `/data/dashboard.db` inside the volume |
| Settings | `/opt/go-dash/go-dash.env` | `.env` next to `docker-compose.yml` |
| Updates | `sudo ./install.sh` from the new release | `docker load` + `docker compose up -d` |

Both serve plain HTTP on port 8080; put an HTTPS reverse proxy in front for
production ([section C](#c-https-with-a-reverse-proxy)).

The release files come from `make dist` (see [README](README.md#release-artifacts)):

| File | Used by |
| --- | --- |
| `go-dash-<version>-linux-amd64.tar.gz` | A — folder with the binary, `install.sh`, systemd unit, env example, docs, example menu |
| `go-dash-image-<version>.tar.gz`, `docker-compose.yml`, `.env.example` | B — Docker image and the compose files to run it |

Each comes with a `.sha256` file: check it with `sha256sum -c <file>.sha256`.

**Ready-to-install server package.** `make dist-server` builds
`dist/go-dash-<version>-linux-amd64.tar.gz` for one server: the same files as
the release plus a configured `go-dash.env` and a `Makefile`, so the server
only needs `tar -xzf … && cd go-dash-*/ && sudo make install` (needs `make`
and `curl`). Options: `SERVER_ADDR` (default `127.0.0.1:8585`, for a reverse
proxy on the same host; proxy headers are trusted), `ADMIN_PASSWORD` (only its
bcrypt hash goes into the package; needs `python3-bcrypt` or `htpasswd` where
you build) and `MENU` (a `go-dash menu export` file imported on the first
install when the menu is empty):

```bash
docker compose exec -T dashboard go-dash menu export > /tmp/menu.json   # optional: current menu
ADMIN_PASSWORD='...' make dist-server MENU=/tmp/menu.json
```

On the server, `make help` lists `install`, `status`, `logs`, `restart`,
`backup`, `password` and `uninstall` (`PURGE=yes` also removes data).

---

## A. Binary + systemd

### A.1 Install

```bash
# On your machine: build the release (or take it from whoever builds releases).
make dist-bin                                   # dist/go-dash-<version>-linux-amd64.tar.gz
make dist-bin GOARCH=arm64                      # for ARM servers

# Copy it to the server.
scp dist/go-dash-<version>-linux-amd64.tar.gz dist/go-dash-<version>-linux-amd64.sha256 user@server:/tmp/

# On the server.
cd /tmp
sha256sum -c go-dash-<version>-linux-amd64.sha256 --ignore-missing
tar -xzf go-dash-<version>-linux-amd64.tar.gz
cd go-dash-<version>-linux-amd64
sudo ./install.sh
```

`install.sh`:

- creates the system user and group `go-dash` (no login shell);
- installs into `/opt/go-dash` with the layout below;
- creates `/opt/go-dash/go-dash.env` from the example **only if it does not exist yet**;
- installs `/etc/systemd/system/go-dash.service`, then enables and starts it.

| Path | Owner / mode | Content |
| --- | --- | --- |
| `/opt/go-dash/bin/go-dash` | root, 0755 | the binary (frontend embedded) |
| `/opt/go-dash/go-dash.env` | root:go-dash, 0640 | settings and secrets (`DASHBOARD_*`) |
| `/opt/go-dash/dashboard.toml` | root:go-dash, 0640 | optional TOML settings (read when present) |
| `/opt/go-dash/data/` | go-dash, 0750 | SQLite database `dashboard.db` (+ `-wal`/`-shm`) |
| `/opt/go-dash/backups/` | go-dash, 0750 | backups (`go-dash backup`, pre-upgrade copies) |
| `/opt/go-dash/*.example`, `README.md`, `menu.json` | root, 0644 | reference files |
| `/etc/systemd/system/go-dash.service` | root, 0644 | the unit |

The service runs as `go-dash` with systemd hardening: the system is read-only
for it except `data/` and `backups/`, and it has no capabilities. Check with
`systemd-analyze security go-dash`.

Open http://server:8080.

### A.2 Configure

Edit the settings, then restart:

```bash
sudo nano /opt/go-dash/go-dash.env
sudo systemctl restart go-dash
```

Main keys (all of them in [README → Configuration](README.md#configuration)):

| Key | Example | Purpose |
| --- | --- | --- |
| `DASHBOARD_ADDR` | `:8080` | listen address (`127.0.0.1:8080` behind a local proxy) |
| `DASHBOARD_TITLE` | `Go Dash` | toolbar title |
| `DASHBOARD_PUBLIC_URL` | `https://dash.example.com` | own address (never opened in an iframe) |
| `DASHBOARD_DEFAULT_THEME` | `ocean-light` | theme for first visits |
| `DASHBOARD_TRUST_PROXY` / `DASHBOARD_COOKIE_SECURE` | `true` | behind an HTTPS proxy |

**Admin password.** Recommended: store it in the database (only a bcrypt hash
is kept, nothing secret in files):

```bash
sudo runuser -u go-dash -- /opt/go-dash/bin/go-dash --db /opt/go-dash/data/dashboard.db admin set-password
```

`DASHBOARD_ADMIN_PASSWORD_HASH` (or `DASHBOARD_ADMIN_PASSWORD`) in
`go-dash.env` overrides the database password. With no password at all, `/admin`
is disabled (the CLI still works).

**Initial menu.** Either import the example (or your own export) once:

```bash
sudo runuser -u go-dash -- /opt/go-dash/bin/go-dash --db /opt/go-dash/data/dashboard.db menu import /opt/go-dash/menu.json
```

or set `DASHBOARD_SEED=/opt/go-dash/menu.json` to import it on start while the
menu is empty. Then manage it at `/admin`.

**IP and port.** `DASHBOARD_ADDR` is `IP:port`: `:8080` listens on every
interface, `10.0.0.5:9090` only on that address and port, `127.0.0.1:8080`
only locally (behind a reverse proxy). Restart the service after changing it.
The same setting is `addr` in `dashboard.toml` or `--addr` on the command line.

**Firewall** (when users reach port 8080 directly):

```bash
sudo ufw allow 8080/tcp                                          # Ubuntu/Debian
sudo firewall-cmd --permanent --add-port=8080/tcp && sudo firewall-cmd --reload   # RHEL family
```

### A.3 Operate

```bash
systemctl status go-dash
journalctl -u go-dash -f              # logs (JSON)
sudo systemctl restart go-dash
sudo systemctl stop go-dash
/opt/go-dash/bin/go-dash version
```

Command-line tasks run as the service user, pointing at the database:

```bash
alias dash='sudo runuser -u go-dash -- /opt/go-dash/bin/go-dash --db /opt/go-dash/data/dashboard.db'
dash menu list --tree
dash menu export -o /opt/go-dash/backups/menu-$(date +%F).json

# Dashboards: "/" is "default"; named ones are served at /dash/<slug>.
dash dash list
dash dash add acme --title "Acme"
dash menu import menu-acme.json --dash acme
dash dash set-password initech          # asks twice; --stdin for scripts
dash dash clear-password initech
```

Password changes made with the CLI apply to the running service at once (no
restart).

### A.4 Backup and restore

Backups are safe while the service runs (`VACUUM INTO`):

```bash
sudo runuser -u go-dash -- /opt/go-dash/bin/go-dash --db /opt/go-dash/data/dashboard.db \
  backup -o /opt/go-dash/backups/dashboard-$(date +%F-%H%M%S).db
```

`backup` never overwrites an existing file, so each run needs a new name.

Daily at 02:00, keeping 14 days (`sudo crontab -e`):

```cron
0 2 * * * runuser -u go-dash -- /opt/go-dash/bin/go-dash --db /opt/go-dash/data/dashboard.db backup -o /opt/go-dash/backups/dashboard-$(date +\%F).db && find /opt/go-dash/backups -name 'dashboard-*.db' -mtime +14 -delete
```

Restore:

```bash
sudo systemctl stop go-dash
sudo install -o go-dash -g go-dash -m 0640 /opt/go-dash/backups/dashboard-2026-10-20.db /opt/go-dash/data/dashboard.db
sudo rm -f /opt/go-dash/data/dashboard.db-wal /opt/go-dash/data/dashboard.db-shm
sudo systemctl start go-dash
```

### A.5 Upgrade

Extract the new release and run the installer again:

```bash
tar -xzf dashboard-<new>-linux-amd64.tar.gz && cd dashboard-<new>-linux-amd64
sudo ./install.sh
```

It backs up the database to `backups/pre-upgrade-<date>.db` (service running
or stopped), replaces the
binary and the unit, keeps `go-dash.env`, `dashboard.toml` and `data/`, and
restarts the service. Compare `/opt/go-dash/go-dash.env.example` with your
`go-dash.env` for new keys. Database migrations run on start; there is no
separate step.

Replacing only the binary works too: back up, stop, copy, start.

```bash
sudo runuser -u go-dash -- /opt/go-dash/bin/go-dash --db /opt/go-dash/data/dashboard.db \
  backup -o /opt/go-dash/backups/pre-upgrade-$(date +%F-%H%M%S).db
sudo systemctl stop go-dash
sudo install -o root -g root -m 0755 go-dash /opt/go-dash/bin/go-dash
sudo systemctl start go-dash
journalctl -u go-dash -n 50                    # no migration error
curl -s http://127.0.0.1:8080/api/config       # "build" shows the new version
```

**Upgrading to the multi-dashboard version** (migration `0003_dashboards`):

- Every existing item moves to the default dashboard at `/`; the menu, the
  admin password, open admin sessions, themes and each viewer's card order
  stay as they were. Nothing changes until a dashboard or a password is added.
- The schema change is one-way: an older binary cannot read the database
  afterwards. Keep the pre-upgrade backup; to roll back, stop the service,
  restore that backup (A.4) and put the old binary back.
- No new configuration key is required. A reverse proxy must forward
  `/dash/...` as well — the examples in section C forward every path.

### A.6 Uninstall

```bash
sudo systemctl disable --now go-dash
sudo rm /etc/systemd/system/go-dash.service && sudo systemctl daemon-reload
# Data and settings: copy /opt/go-dash/data and go-dash.env first if you need them.
sudo rm -rf /opt/go-dash
sudo userdel go-dash
```

---

## B. Docker + Docker Compose

### B.1 Install Docker Engine and the Compose plugin

Use Docker's official packages (the distro `docker.io`/`docker-compose`
packages are often old). Remove old versions first if present.

**Ubuntu / Debian**

```bash
sudo apt-get remove -y docker.io docker-doc docker-compose podman-docker containerd runc 2>/dev/null || true
sudo apt-get update
sudo apt-get install -y ca-certificates curl
sudo install -m 0755 -d /etc/apt/keyrings
. /etc/os-release   # ID is ubuntu or debian
sudo curl -fsSL "https://download.docker.com/linux/$ID/gpg" -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/$ID $VERSION_CODENAME stable" \
  | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
sudo apt-get update
sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
```

**RHEL / Rocky Linux / AlmaLinux / CentOS Stream**

```bash
sudo dnf remove -y docker docker-client docker-common docker-latest docker-engine podman runc 2>/dev/null || true
sudo dnf -y install dnf-plugins-core
sudo dnf config-manager --add-repo https://download.docker.com/linux/rhel/docker-ce.repo
sudo dnf install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
```

Fedora: the repository is `https://download.docker.com/linux/fedora/docker-ce.repo`;
with DNF5 (Fedora 41+) add it with
`sudo dnf config-manager addrepo --from-repofile=https://download.docker.com/linux/fedora/docker-ce.repo`.

**Start it and check**

```bash
sudo systemctl enable --now docker
sudo docker run --rm hello-world
docker compose version                   # Docker Compose v2 ("docker compose", not "docker-compose")
sudo usermod -aG docker "$USER"          # optional: run docker without sudo (log out and back in)
```

Members of the `docker` group are effectively root on the host; add only admins.

### B.2 Run the released image (no build on the server)

```bash
# Copy dist/go-dash-image-<version>.tar.gz, docker-compose.yml and .env.example to the server, then:
mkdir -p ~/go-dash && cd ~/go-dash        # or /opt/go-dash-docker
docker load -i go-dash-image-<version>.tar.gz
cp .env.example .env                      # image name and tag are already set
nano .env                                 # DASHBOARD_ADMIN_PASSWORD (or _HASH), DASHBOARD_PUBLIC_URL, port
chmod 600 .env
docker compose up -d
docker compose ps                         # STATUS: healthy
```

Open http://server:8080. **IP and port:** `DASHBOARD_BIND_IP` and
`DASHBOARD_PORT` in `.env` choose the host address the container is published
on (e.g. `DASHBOARD_BIND_IP=10.0.0.5`, `DASHBOARD_PORT=9090`; `127.0.0.1`
behind a local proxy); run `docker compose up -d` after changing them. Inside
the container the app always listens on 8080.
The compose file is [`deploy/docker-compose.yml`](../deploy/docker-compose.yml).

### B.3 Or build from the source code

```bash
git clone https://github.com/jniltinho/go-dash.git && cd go-dash
cp .env.example .env && nano .env && chmod 600 .env
make docker-up                            # docker compose up -d --build (root docker-compose.yml)
```

### B.4 Operate

```bash
docker compose ps
docker compose logs -f
docker compose restart
docker compose down                       # stops; the data volume is kept
docker compose exec dashboard go-dash version
docker compose exec dashboard go-dash menu list --tree
docker compose exec dashboard go-dash admin set-password   # interactive; or set it in .env
```

The first start imports the example menu when the menu is empty (build from
source); for a released image, import yours with
`docker compose exec -T dashboard go-dash menu import /dev/stdin < menu.json`.

Where the data is: `docker volume inspect go-dash_dashboard-data` (the prefix is
the compose project, i.e. the folder name) → `Mountpoint`, file `dashboard.db`.

### B.5 Backup, restore, upgrade

```bash
# Backup (safe while running), copied out of the container; `backup` never
# overwrites a file, so use a new name each time and remove it afterwards:
f=dashboard-$(date +%F-%H%M%S).db
docker compose exec dashboard go-dash backup -o /data/$f
docker compose cp dashboard:/data/$f ./$f
docker compose exec dashboard rm /data/$f

# Restore (copies the file, gives it to the container user and drops the old WAL):
docker compose stop dashboard
docker compose run --rm --no-deps --user root -v "$PWD/dashboard-2026-10-20.db:/restore.db:ro" \
  --entrypoint sh dashboard \
  -c 'cp /restore.db /data/dashboard.db && chown dashboard:dashboard /data/dashboard.db && rm -f /data/dashboard.db-wal /data/dashboard.db-shm'
docker compose start dashboard

# Upgrade to a new released image (back up first: migrations run on start and
# the multi-dashboard schema cannot be read by older images):
docker load -i go-dash-image-<new>.tar.gz
sed -i 's/^DASHBOARD_TAG=.*/DASHBOARD_TAG=<new>/' .env
docker compose up -d
```

---

## C. HTTPS with a reverse proxy

Run the dashboard on localhost and let nginx (or Caddy/Traefik) terminate TLS:

```bash
# systemd: in /opt/go-dash/go-dash.env
DASHBOARD_ADDR=127.0.0.1:8080
DASHBOARD_PUBLIC_URL=https://dash.example.com
DASHBOARD_TRUST_PROXY=true
DASHBOARD_COOKIE_SECURE=true

# Docker: in .env (then docker compose up -d)
DASHBOARD_BIND_IP=127.0.0.1
DASHBOARD_PUBLIC_URL=https://dash.example.com
DASHBOARD_TRUST_PROXY=true
DASHBOARD_COOKIE_SECURE=true
```

`/etc/nginx/conf.d/go-dash.conf`:

```nginx
server {
    listen 443 ssl;
    http2 on;                       # nginx < 1.25.1: drop this line and use "listen 443 ssl http2;"
    server_name dash.example.com;
    ssl_certificate     /etc/ssl/certs/dash.example.com.pem;
    ssl_certificate_key /etc/ssl/private/dash.example.com.key;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}

server {
    listen 80;
    server_name dash.example.com;
    return 301 https://$host$request_uri;
}
```

### C.1 Under a sub-path (e.g. `https://intranet.example.com/panel/`)

Set the prefix and restart; one build serves any prefix, and both nginx
styles below work (the server removes the prefix itself when nginx keeps it):

```bash
# /opt/go-dash/go-dash.env (Docker: .env)
DASHBOARD_BASE_PATH=/panel/
DASHBOARD_PUBLIC_URL=https://intranet.example.com/panel/   # optional; its path must equal the base path
DASHBOARD_TRUST_PROXY=true
DASHBOARD_COOKIE_SECURE=true
```

```nginx
# Prefix kept: nginx forwards /panel/... unchanged.
location /panel/ {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}

# …or prefix stripped: note the trailing slash in proxy_pass.
location /panel/ {
    proxy_pass http://127.0.0.1:8080/;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

No `sub_filter`, `proxy_redirect` or `proxy_cookie_path` is needed: links,
redirects and cookies already carry `/panel/`. Cookies are scoped to that
path, so applications on disjoint prefixes of the host (e.g. `/ops/`) do not
receive them; anything published *under* `/panel/` would, so keep that path
for the dashboard alone (a cookie path is not a security boundary). Opening `/panel`
redirects to `/panel/`. The base path must not start with a name the
go-dash serves at its root (`api`, `admin`, `assets`, `dash`, `healthz`,
`index.html`, `favicon.svg`), nor contain `.`/`..` segments or encoded
characters. `curl http://127.0.0.1:8080/healthz` keeps working on the
backend without the prefix.

Alternatively, serve HTTPS directly with `DASHBOARD_TLS_CERT` and
`DASHBOARD_TLS_KEY` (port 443 then needs `CAP_NET_BIND_SERVICE`, see the
comment in the unit).

---

## Troubleshooting

| Symptom | Check |
| --- | --- |
| Service does not start | `journalctl -u go-dash -n 50` (or `docker compose logs dashboard`) |
| Container restarts with `attempt to write a readonly database` | the database file is not owned by the container user: restore with the command in B.5 (it runs `chown`) |
| `/admin` says it is disabled | no admin password: `admin set-password` or `DASHBOARD_ADMIN_PASSWORD_HASH` |
| Login works but the session drops behind HTTPS | set `DASHBOARD_TRUST_PROXY=true` and `DASHBOARD_COOKIE_SECURE=true` |
| A site stays blank in a tab | it forbids iframes: set that item to "New browser tab" in `/admin` |
| `/dash/<slug>` shows "Dashboard not found" | the slug does not exist (or was renamed): `dash dash list`, then `dash dash add <slug> --title ...` |
| Visitors must type a dashboard password again | its password was changed or removed (that logs its viewers out), or the session expired (2 h idle / 12 h) |
| "Too many attempts" on a dashboard password | unlock attempts are limited per IP (`DASHBOARD_LOGIN_RATE` per minute, all dashboards together); wait a minute |
| Under a sub-path the page loads without styles, or `/panel/api/...` gives 404 | `DASHBOARD_BASE_PATH` is not set (or differs from the nginx `location`); check `curl -s http://127.0.0.1:8080/panel/ | grep '<base'` |
| Port 8080 already in use, or listen on another IP | `DASHBOARD_ADDR=IP:port` (systemd) or `DASHBOARD_BIND_IP` / `DASHBOARD_PORT` (Docker) |
| `permission denied` on the database | files in `/opt/go-dash/data` must belong to `go-dash` (`sudo chown -R go-dash:go-dash /opt/go-dash/data`) |
