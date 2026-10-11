# syntax=docker/dockerfile:1
#
# Troupe local / single-node image — 4-stage self-contained build.
#
# Why this exists instead of docker/troupe/Dockerfile:
#   - docker/troupe/Dockerfile expects a host cross-compiled binary named
#     `astonish-linux-${TARGETARCH}`, while the Makefile emits `troupe-linux-*`
#     (Makefile:553 / Makefile:559). Building here removes that coupling.
#   - Nothing needs to be installed on the host beyond Docker.
#
# Base must be glibc (Debian), not Alpine/musl: pkg/codeintel uses purego.Dlopen,
# which produces a dynamically linked binary needing /lib64/ld-linux-*.so.2.
# See docker/troupe/Dockerfile:8-9 for the original note.
#
# Build (from repo root):
#   docker compose -f deploy/compose.yaml build
# or standalone:
#   docker build -f deploy/Dockerfile -t troupe-local:latest .

# ---------------------------------------------------------------------------
# Pinned toolchain images. Bump intentionally.
#   GO_IMAGE   must be >= go.mod's toolchain (go1.26.1); 1.27 satisfies it
#              without triggering a GOTOOLCHAIN download inside the build.
#   NODE_IMAGE must satisfy web/package.json (CI uses Node 22/24).
# ---------------------------------------------------------------------------
ARG GO_IMAGE=golang:1.27-bookworm
ARG NODE_IMAGE=node:22-bookworm-slim
ARG UV_VERSION=0.12.1
ARG DEBIAN_BASE=debian:bookworm-slim

# ===========================================================================
# Stage 1 — web assets (React/Vite + sandbox + slides bundles)
# ===========================================================================
FROM ${NODE_IMAGE} AS web-builder

WORKDIR /app/web

# npm registry for this stage. Empty = official registry.npmjs.org. Set a China
# mirror when the official registry is slow or drops the connection mid-install
# (symptom: "npm error code ECONNRESET ... Client network socket disconnected
# before secure TLS connection was established"):
#   https://registry.npmmirror.com
ARG NPM_REGISTRY=""

# Install everything (devDependencies included): `npm run build` runs
# `tsc --noEmit` first, so typescript must be present — `npm ci --omit=dev`
# would break the build.
COPY web/package.json web/package-lock.json ./
RUN set -eu; \
    if [ -n "$NPM_REGISTRY" ]; then npm config set registry "$NPM_REGISTRY"; fi; \
    npm ci

COPY web/ ./

# Produces web/dist/<version>/{index.html,assets/,sandbox-runtime.js,
# slides-runtime.js,tailwind-browser.js}
RUN npm run build

# ===========================================================================
# Stage 2 — Go binary (UI is embedded via //go:embed all:dist)
# ===========================================================================
FROM ${GO_IMAGE} AS go-builder

ENV CGO_ENABLED=0 \
    GOOS=linux

WORKDIR /src

# Dependency layer: invalidated only when go.mod/go.sum change.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# IMPORTANT: copy the freshly built dist AFTER `COPY . .`. Merge semantics mean
# builder files win over same-named host files, but a stale host web/dist could
# otherwise leave junk behind. Also add `web/dist` to .dockerignore.
COPY --from=web-builder /app/web/dist /src/web/dist

ARG TARGETARCH
ARG VERSION=dev
ARG BINARY_NAME=troupe

# go:embed refuses an empty dist directory, so fail loudly instead of shipping
# a UI-less binary. `touch web/embed.go` mirrors Makefile:102 (cache-busting).
RUN test -n "$(ls -A /src/web/dist 2>/dev/null)" \
    || (echo "web/dist is empty — Stage 1 produced no assets" && exit 1)
RUN touch web/embed.go \
    && go build -trimpath \
        -ldflags="-s -w -X github.com/zhu13818202655/troupe/cmd/troupe.Version=${VERSION}" \
        -o /out/${BINARY_NAME} .

# ===========================================================================
# Stage 3 — runtime toolchain sources (prebuilt, no apt/QEMU cost)
# ===========================================================================
FROM ${NODE_IMAGE} AS node-runtime
FROM ghcr.io/astral-sh/uv:${UV_VERSION} AS uv-runtime

# ===========================================================================
# Stage 4 — runtime image
# ===========================================================================
FROM ${DEBIAN_BASE}

ENV DEBIAN_FRONTEND=noninteractive

# Debian apt mirror for this step. Empty = official http://deb.debian.org.
# On networks where the official source is unreachable, set a China mirror, e.g.
#   http://mirrors.aliyun.com
# It MUST be http:// and NOT https://: ca-certificates is installed by this very
# RUN, so at fetch time the image has no trust store yet (verified — the
# bookworm-slim base ships no ca-certificates.crt) and every HTTPS fetch fails
# TLS verification.
# The host is swapped globally, which is correct: both entries in Bookworm's
# deb822 file (…/debian and …/debian-security) live under the same mirror host.
ARG APT_MIRROR=""

# Dependencies mirror docker/troupe/Dockerfile:53-78. The Chromium shared-library
# set is for headless Chrome (rod), used by slide thumbnail generation and PDF
# export; without them Chrome fails with "error while loading shared libraries:
# libgobject-2.0.so.0". `curl` backs the HEALTHCHECK probe.
RUN set -eu; \
    if [ -n "$APT_MIRROR" ]; then \
        MIRROR="${APT_MIRROR%/}"; \
        for f in /etc/apt/sources.list.d/debian.sources /etc/apt/sources.list; do \
            if [ -f "$f" ]; then sed -i "s|http://deb.debian.org|$MIRROR|g" "$f"; fi; \
        done; \
    fi; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        git \
        curl \
        libglib2.0-0 \
        libnss3 \
        libnspr4 \
        libdbus-1-3 \
        libatk1.0-0 \
        libatk-bridge2.0-0 \
        libcups2 \
        libdrm2 \
        libxkbcommon0 \
        libatspi2.0-0 \
        libxcomposite1 \
        libxdamage1 \
        libxfixes3 \
        libxrandr2 \
        libgbm1 \
        libpango-1.0-0 \
        libcairo2 \
        libasound2 \
        fonts-liberation; \
    apt-get clean; \
    rm -rf /var/lib/apt/lists/*

# Node + npx (host-side MCP servers when no sandbox is wired).
COPY --from=node-runtime /usr/local /usr/local
# uv / uvx (Python MCP servers).
COPY --from=uv-runtime /uv /uvx /usr/local/bin/

RUN node --version && npx --version && uv --version && git --version

WORKDIR /app

# Slide PPTX workers: their Node deps live in `dependencies` (not devDeps),
# so `--omit=dev` is safe. The workers resolve modules via
# createRequire(`${process.cwd()}/package.json`), hence cwd must be /app/web.
# Same registry knob as stage 1 — see the note there.
ARG NPM_REGISTRY=""
COPY web/package.json web/package-lock.json /app/web/
RUN set -eu; \
    if [ -n "$NPM_REGISTRY" ]; then npm config set registry "$NPM_REGISTRY"; fi; \
    cd /app/web; \
    npm ci --omit=dev --ignore-scripts; \
    npm cache clean --force
COPY pkg/docs/slides/pptxworker/worker.mjs pkg/docs/slides/pptxworker/import_worker.mjs /app/pkg/docs/slides/pptxworker/
ENV TROUPE_ROOT=/app

# Config lives here by default when HOME=/root: config.GetConfigDir resolves
# XDG_CONFIG_HOME first, then falls back to os.UserConfigDir(). Pinned
# explicitly so the bind mount in compose.yaml always lands in the same place.
# The fallback also keeps reading a populated pre-rebrand "astonish" directory.
ENV XDG_CONFIG_HOME=/root/.config
# Keep scheduler + channels + fleet active while still logging to stdout
# (see pkg/daemon/logger.go:89-103). Alternative: TROUPE_MODE=api/worker,
# but that disables scheduler and inbound channels.
ENV TROUPE_LOG_DEST=stdout

ARG BINARY_NAME=troupe
COPY --from=go-builder /out/${BINARY_NAME} /usr/local/bin/${BINARY_NAME}

EXPOSE 9393

HEALTHCHECK --interval=30s --timeout=5s --retries=5 --start-period=180s \
    CMD curl -fsS http://127.0.0.1:9393/api/healthz || exit 1

ENTRYPOINT ["/usr/local/bin/troupe"]
CMD ["daemon", "run"]
