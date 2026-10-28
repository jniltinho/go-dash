# Go Dash server package: run from the extracted folder.
#
#   sudo make install     install or upgrade, then start the go-dash service
#   make status|logs      service state / follow the logs
#
# Settings live in /opt/go-dash/go-dash.env (see README.md).

PREFIX  := /opt/go-dash
SERVICE := go-dash
DASH    := runuser -u $(SERVICE) -- $(PREFIX)/bin/go-dash --db $(PREFIX)/data/dashboard.db
# Listen address from the env file, without quotes (systemd accepts them).
ADDR     = $(shell sed -n 's/^DASHBOARD_ADDR=//p' $(PREFIX)/go-dash.env 2>/dev/null | tail -n1 | tr -d "\"'")
# Written once menu.json was imported, so later installs never import it again.
MENU_DONE := $(PREFIX)/data/.menu-imported

.DEFAULT_GOAL := help
.PHONY: help install check status logs restart start stop backup password uninstall check-root

help: ## Show the targets
	@grep -hE '^[a-z-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

check-root:
	@[ "$$(id -u)" = 0 ] || { echo "run with sudo: sudo make $(MAKECMDGOALS)"; exit 1; }

install: check-root ## Install or upgrade, start the service and import menu.json once (empty menu only)
	@set -e; \
	./install.sh ./go-dash; \
	if [ -f menu.json ] && [ ! -f $(MENU_DONE) ]; then \
	  items=$$($(DASH) menu list --json); \
	  if printf '%s' "$$items" | grep -q '"id"'; then \
	    echo "==> Menu already has items: menu.json not imported"; \
	  else \
	    echo "==> Importing menu.json"; install -o root -g $(SERVICE) -m 0640 menu.json $(PREFIX)/menu.json; \
	    $(DASH) menu import $(PREFIX)/menu.json; \
	  fi; \
	  install -o $(SERVICE) -g $(SERVICE) -m 0640 /dev/null $(MENU_DONE); \
	fi
	@$(MAKE) --no-print-directory check

check: ## Check the service answers on its address
	@addr="$(ADDR)"; case "$$addr" in :*) addr="127.0.0.1$$addr";; esac; \
	for i in 1 2 3 4 5; do \
	  if curl -gfsS "http://$$addr/healthz" >/dev/null 2>&1; then echo "==> OK: http://$$addr/healthz"; exit 0; fi; sleep 1; \
	done; echo "==> no answer on http://$$addr (see: make logs)"; exit 1

status: ## Service state
	@systemctl status $(SERVICE) --no-pager

logs: ## Follow the logs (Ctrl+C to leave)
	@journalctl -u $(SERVICE) -f

restart: check-root ## Restart (after editing /opt/go-dash/go-dash.env)
	systemctl restart $(SERVICE)
	@$(MAKE) --no-print-directory check

start: check-root ## Start the service
	systemctl start $(SERVICE)

stop: check-root ## Stop the service
	systemctl stop $(SERVICE)

backup: check-root ## Back up the database to /opt/go-dash/backups
	$(DASH) backup -o $(PREFIX)/backups/dashboard-$$(date +%F-%H%M%S).db

password: check-root ## Set a new admin password (asked twice; stored as bcrypt in the database)
	$(DASH) admin set-password
	@# The password in the env file would override the database one.
	sed -i -e 's/^DASHBOARD_ADMIN_PASSWORD_HASH=.*/DASHBOARD_ADMIN_PASSWORD_HASH=/' \
	  -e 's/^DASHBOARD_ADMIN_PASSWORD=.*/DASHBOARD_ADMIN_PASSWORD=/' $(PREFIX)/go-dash.env
	systemctl restart $(SERVICE)

uninstall: check-root ## Remove the service and binary (keeps /opt/go-dash/data unless PURGE=yes)
	-systemctl disable --now $(SERVICE)
	rm -f /etc/systemd/system/$(SERVICE).service
	systemctl daemon-reload
	rm -rf $(PREFIX)/bin
	@if [ "$(PURGE)" = yes ]; then rm -rf $(PREFIX); userdel $(SERVICE) 2>/dev/null || true; echo "==> removed $(PREFIX) and user $(SERVICE)"; \
	else echo "==> kept $(PREFIX)/data, backups and go-dash.env (PURGE=yes removes them)"; fi
