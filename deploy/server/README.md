# Go Dash — server package

Ready to run behind nginx: the service listens on **127.0.0.1:__ADDR_PORT__**
(`DASHBOARD_ADDR=__ADDR__`) and trusts the proxy headers.

## Install

```bash
sha256sum -c go-dash-*.sha256 --ignore-missing     # optional, next to the .tar.gz
tar -xzf go-dash-*.tar.gz && cd go-dash-*/
sudo make install
```

This creates the `go-dash` user, installs into `/opt/go-dash`, enables and
starts the systemd service `go-dash`, imports `menu.json` on the first install
(only when the menu is empty) and checks `http://__ADDR__/healthz`.

Admin: open `/admin` and sign in with the password you were given. Change it
with `sudo make password` (stored as bcrypt in the database).

## nginx

The proxy must point to the service and pass these headers (the session cookie
is marked Secure automatically when `X-Forwarded-Proto` is `https`):

```nginx
location / {
    proxy_pass http://__ADDR__;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

## Day to day

| Command | What it does |
| --- | --- |
| `make status` / `make logs` | service state / follow the logs |
| `sudo make restart` | after editing `/opt/go-dash/go-dash.env` |
| `sudo make backup` | database copy in `/opt/go-dash/backups` |
| `sudo make password` | new admin password |
| `sudo make install` | upgrade: run it from a newer package (settings and data are kept; the database is backed up first; migrations run on start) |
| `sudo make uninstall` | remove the service (`PURGE=yes` also removes data and settings) |

Files: binary `/opt/go-dash/bin/go-dash`, settings `/opt/go-dash/go-dash.env`,
database `/opt/go-dash/data/dashboard.db`, unit `/etc/systemd/system/go-dash.service`.
Full guide: `INSTALL.md`.
