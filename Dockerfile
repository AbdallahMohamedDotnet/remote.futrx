# syntax=docker/dockerfile:1.7
#
# Multi-stage build:
#   1) Build the Preact/Vite frontend → static bundle in /app/public/
#   2) Build the Go binary, embedding the bundle via //go:embed
#   3) Minimal runtime: Alpine + Node (for claude CLI) + tmux + git + ripgrep
#
# Resulting image is ~250 MB (Node alpine base 150 MB + claude CLI 60 MB + rest).

# --- 1) frontend build -----------------------------------------------------
FROM node:20-alpine AS web-build
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund --loglevel=error
COPY web/ ./
# Vite writes to ../public, which we collect below.
RUN npm run build

# --- 2) backend build ------------------------------------------------------
FROM golang:1.22-alpine AS go-build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
COPY --from=web-build /public ./public
# Static binary so it runs on the slim runtime image.
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/remote .

# --- 3) runtime ------------------------------------------------------------
FROM node:20-alpine

# Runtime dependencies:
#   tmux            — kept for the legacy PTY-bridge WS (unused by the chat UI
#                     but the endpoint exists; harmless if you don't hit it)
#   git, ripgrep    — claude's Read/Grep tools rely on these in many flows
#   bash            — claude spawns bash for Bash tool
#   ca-certificates — TLS to api.anthropic.com
RUN apk add --no-cache tmux git ripgrep bash ca-certificates curl \
 && npm install -g --no-audit --no-fund @anthropic-ai/claude-code@latest \
 && claude --version

COPY --from=go-build /out/remote /usr/local/bin/remote

# Persistent volumes:
#   /data         — chat history (events.jsonl + meta.json per chat)
#   /root/.claude — claude CLI's own auth + history state
VOLUME ["/data", "/root/.claude"]

ENV HOST=0.0.0.0 \
    PORT=7682 \
    DATA_DIR=/data \
    HOME=/root \
    TERM=xterm-256color

EXPOSE 7682
WORKDIR /workspace
CMD ["/usr/local/bin/remote"]
