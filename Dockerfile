# syntax=docker/dockerfile:1

# ---------------------------------------------------------------------------
# Stage 1: frontend (Vue 3 + Vite+ + DHTMLX) -> /src/web/dist
# ---------------------------------------------------------------------------
FROM node:24-bookworm-slim AS frontend
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY frontend/ ./
RUN npx vp check \
    && npm run typecheck \
    && npx vp test run \
    && npx vp build

# ---------------------------------------------------------------------------
# Stage 2: Go binary with the frontend embedded
# ---------------------------------------------------------------------------
FROM golang:1.26-alpine AS backend
ARG VERSION=dev
ARG GIT_COMMIT=unknown
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
COPY cmd/ cmd/
COPY internal/ internal/
COPY --from=frontend /src/web/dist web/dist
RUN go vet ./... \
    && go test ./... \
    && BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
    && CGO_ENABLED=0 go build -trimpath -o /out/go-dash \
       -ldflags "-s -w -X github.com/jniltinho/go-dash/cmd.Version=${VERSION} -X github.com/jniltinho/go-dash/cmd.GitCommit=${GIT_COMMIT} -X github.com/jniltinho/go-dash/cmd.BuildDate=${BUILD_DATE}" .

# ---------------------------------------------------------------------------
# Stage 3: minimal runtime
# ---------------------------------------------------------------------------
FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata wget \
    && addgroup -S dashboard && adduser -S -G dashboard -h /app dashboard \
    && mkdir -p /data && chown dashboard:dashboard /data
ENV TZ=America/Sao_Paulo \
    DASHBOARD_ADDR=:8080 \
    DASHBOARD_DB=/data/dashboard.db \
    DASHBOARD_SEED=/app/examples/menu.json
WORKDIR /app
COPY --from=backend /out/go-dash /usr/local/bin/go-dash
COPY examples/ /app/examples/
COPY dashboard.toml.example /app/dashboard.toml.example
USER dashboard
VOLUME ["/data"]
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["go-dash"]
CMD ["serve"]
