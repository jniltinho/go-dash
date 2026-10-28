# Link aggregator dashboard — Go (Cobra/Echo v5/SQLite) + Vue 3 (Vite+) + DHTMLX.
#
# `make` builds the frontend into web/dist and then the Go binary embedding it.

BINARY      := go-dash
BIN_DIR     := bin
FRONTEND    := frontend
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
GIT_COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE  ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS     := -s -w \
	-X github.com/jniltinho/go-dash/cmd.Version=$(VERSION) \
	-X github.com/jniltinho/go-dash/cmd.GitCommit=$(GIT_COMMIT) \
	-X github.com/jniltinho/go-dash/cmd.BuildDate=$(BUILD_DATE)

IMAGE       ?= ghcr.io/jniltinho/go-dash
DIST_DIR    := dist
GOOS        ?= linux
GOARCH      ?= amd64
PORT        ?= 8080
VP          := cd $(FRONTEND) && npx vp
# Chrome used by Playwright when its own browser is not installed.
PW_CHROME_PATH ?=

.DEFAULT_GOAL := build
.PHONY: help build frontend frontend-deps backend run dev dev-frontend test test-go test-frontend \
	e2e screenshots check lint fmt typecheck clean dist dist-bin dist-server dist-image docker-build docker-up docker-down docker-logs seed

help: ## Show this help
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

## ---------------------------------------------------------------- build

build: frontend ## Build frontend and Go binary (bin/go-dash)
	$(MAKE) backend

frontend-deps: ## Install frontend dependencies (npm ci)
	cd $(FRONTEND) && npm ci --no-audit --no-fund

$(FRONTEND)/node_modules: $(FRONTEND)/package-lock.json
	cd $(FRONTEND) && npm ci --no-audit --no-fund
	@touch $@

frontend: $(FRONTEND)/node_modules ## Check, type-check and build the frontend into web/dist
	$(VP) check
	cd $(FRONTEND) && npm run typecheck
	$(VP) build

backend: ## Build the Go binary (expects web/dist)
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) .

## ---------------------------------------------------------------- run

run: build ## Build and run the server on $(PORT)
	./$(BIN_DIR)/$(BINARY) serve --addr :$(PORT)

dev: ## Run the Go server reading web/dist from disk (pair with `make dev-frontend`)
	go run . serve --addr :$(PORT) --dev

dev-frontend: ## Run the Vite+ dev server on :9000 proxying /api to :$(PORT)
	cd $(FRONTEND) && DASHBOARD_BACKEND=http://localhost:$(PORT) npx vp dev

seed: ## Import examples/menu.json into ./dashboard.db
	go run . menu import examples/menu.json

## ---------------------------------------------------------------- quality

test: test-go test-frontend ## Run Go and frontend unit tests

test-go: ## Run Go tests (race detector needs CGO)
	go test ./...

test-frontend: $(FRONTEND)/node_modules ## Run frontend unit tests (Vitest via Vite+)
	$(VP) test run

e2e: build ## Run Playwright end-to-end tests against the binary
	cd $(FRONTEND) && PW_CHROME_PATH=$(PW_CHROME_PATH) npx playwright test

screenshots: build ## Regenerate docs/screenshots (docs/README.md theme gallery)
	cd $(FRONTEND) && PW_CHROME_PATH=$(PW_CHROME_PATH) npx playwright test -c playwright.screenshots.config.ts

check: lint typecheck test ## Lint, type-check and test everything

lint: ## go vet + gofmt check + vp check (oxlint/oxfmt)
	go vet ./...
	@test -z "$$(gofmt -l . | grep -v node_modules)" || (gofmt -l . | grep -v node_modules; exit 1)
	$(VP) check

fmt: ## Format Go and frontend code
	gofmt -w $$(find . -name '*.go' -not -path './frontend/*')
	$(VP) check --fix

typecheck: $(FRONTEND)/node_modules ## vue-tsc type-check (not covered by vp check)
	cd $(FRONTEND) && npm run typecheck

clean: ## Remove build outputs (keeps web/dist/.gitkeep)
	rm -rf $(BIN_DIR) $(DIST_DIR)
	find web/dist -mindepth 1 ! -name .gitkeep -delete

## ---------------------------------------------------------------- docker

docker-build: ## Build the Docker image (multi-stage)
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		-t $(IMAGE):$(VERSION) -t $(IMAGE):latest .

## ---------------------------------------------------------------- release

dist: dist-bin dist-image ## Release artifacts in dist/: binary tarball + Docker image tarball (not versioned)

RELEASE     = $(BINARY)-$(VERSION)-$(GOOS)-$(GOARCH)

dist-bin: frontend ## dist/: binary + <release>.tar.gz (binary, install.sh, systemd unit, docs)
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -trimpath -ldflags "$(LDFLAGS)" \
		-o $(DIST_DIR)/$(RELEASE) .
	@# The tarball holds one folder: everything `sudo ./install.sh` needs.
	rm -rf $(DIST_DIR)/stage && mkdir -p $(DIST_DIR)/stage/$(RELEASE)
	cp $(DIST_DIR)/$(RELEASE) $(DIST_DIR)/stage/$(RELEASE)/$(BINARY)
	cp deploy/install.sh deploy/systemd/go-dash.service deploy/systemd/go-dash.env.example \
		README.md dashboard.toml.example $(DIST_DIR)/stage/$(RELEASE)/
	cp docs/install.md $(DIST_DIR)/stage/$(RELEASE)/INSTALL.md
	cp examples/menu.json $(DIST_DIR)/stage/$(RELEASE)/menu.json
	tar -czf $(DIST_DIR)/$(RELEASE).tar.gz -C $(DIST_DIR)/stage $(RELEASE)
	rm -rf $(DIST_DIR)/stage
	cd $(DIST_DIR) && sha256sum $(RELEASE) $(RELEASE).tar.gz > $(RELEASE).sha256

SERVER_ADDR ?= 127.0.0.1:8585

dist-server: dist-bin ## dist/go-dash-<version>-<os>-<arch>.tar.gz: server package (SERVER_ADDR, ADMIN_PASSWORD, MENU=export.json)
	RELEASE=$(RELEASE) DIST_DIR=$(DIST_DIR) VERSION=$(VERSION) GOOS=$(GOOS) GOARCH=$(GOARCH) \
		SERVER_ADDR="$(SERVER_ADDR)" MENU="$(MENU)" deploy/server/build.sh

dist-image: docker-build ## dist/: image tarball + docker-compose.yml and .env.example to run it
	@mkdir -p $(DIST_DIR)
	@# Two steps (no pipe), so a failing `docker save` fails the target.
	docker save -o $(DIST_DIR)/$(BINARY)-image-$(VERSION).tar $(IMAGE):$(VERSION) $(IMAGE):latest
	gzip -9f $(DIST_DIR)/$(BINARY)-image-$(VERSION).tar
	cd $(DIST_DIR) && sha256sum $(BINARY)-image-$(VERSION).tar.gz > $(BINARY)-image-$(VERSION).tar.gz.sha256
	cp deploy/docker-compose.yml $(DIST_DIR)/docker-compose.yml
	sed -e 's|^DASHBOARD_IMAGE=.*|DASHBOARD_IMAGE=$(IMAGE)|' -e 's|^DASHBOARD_TAG=.*|DASHBOARD_TAG=$(VERSION)|' \
		.env.example > $(DIST_DIR)/.env.example

docker-up: ## Start with docker compose (builds if needed)
	docker compose up -d --build

docker-down: ## Stop docker compose
	docker compose down

docker-logs: ## Follow container logs
	docker compose logs -f
