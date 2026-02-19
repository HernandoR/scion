#!/usr/bin/env bash
# =============================================================================
# Web NATS + SSE — Server Setup Walkthrough
# =============================================================================
#
# This script sets up the server-side components for the NATS-to-SSE real-time
# event pipeline (Milestones 7 and 8).
#
# Architecture:
#
#   Browser (EventSource) <-- SSE stream <-- Koa /events endpoint
#                                                |
#                                           SSEManager
#                                                |
#                                           NatsClient
#                                                |
#                                           NATS Server
#                                                |
#                                      Hub / Runtime Broker
#                                      (publishes events)
#
# The pipeline is unidirectional: NATS messages published by the Hub or
# Runtime Broker flow through the web server's SSE Manager and arrive at the
# browser as Server-Sent Events. Subscriptions are declared via query
# parameters at connection time and are immutable for the connection lifetime.
#
# Prerequisites:
#   - Node.js 20+
#   - Docker (for running NATS)
#   - NATS CLI (nats) for publishing test messages (optional but recommended)
#   - curl, jq (optional) for verification
#   - A running Hub API (or DEV_AUTH=true for local development without one)
#
# Usage:
#   chmod +x web-nats-server.sh
#   ./web-nats-server.sh [start|stop|restart|status|teardown]
#
# =============================================================================

set -euo pipefail

NATS_CONTAINER="scion-nats"
NATS_PORT=4222
WEB_PORT="${PORT:-8080}"
WEB_DIR="${WEB_DIR:-web}"
NATS_IMAGE="nats:latest"

# ---------------------------------------------------------------------------
# Helper: colored output
# ---------------------------------------------------------------------------
info()  { printf '\033[1;34m[INFO]\033[0m  %s\n' "$*"; }
ok()    { printf '\033[1;32m[OK]\033[0m    %s\n' "$*"; }
warn()  { printf '\033[1;33m[WARN]\033[0m  %s\n' "$*"; }
err()   { printf '\033[1;31m[ERROR]\033[0m %s\n' "$*"; }

# ---------------------------------------------------------------------------
# 1. NATS Server Management
# ---------------------------------------------------------------------------
#
# Environment Variables Reference:
#
#   SCION_NATS_URL    (none)           NATS server URL(s), comma-separated for clusters
#   NATS_URL          (none)           Fallback if SCION_NATS_URL is not set
#   NATS_TOKEN        (none)           Optional auth token for NATS connection
#   NATS_ENABLED      true if URL set  Explicitly enable/disable NATS
#   NATS_MAX_RECONNECT -1 (infinite)   Max reconnect attempts before giving up
#   DEV_AUTH          (none)           Set to "true" to bypass OAuth for local testing
#   PORT              8080             Web server port
#

start_nats() {
    info "Starting NATS server..."

    # Check if container already exists
    if docker ps -a --format '{{.Names}}' | grep -q "^${NATS_CONTAINER}$"; then
        if docker ps --format '{{.Names}}' | grep -q "^${NATS_CONTAINER}$"; then
            ok "NATS container '${NATS_CONTAINER}' is already running on port ${NATS_PORT}"
            return 0
        fi
        info "Starting existing NATS container '${NATS_CONTAINER}'..."
        docker start "${NATS_CONTAINER}"
    else
        info "Creating NATS container '${NATS_CONTAINER}' on port ${NATS_PORT}..."
        docker run -d --name "${NATS_CONTAINER}" -p "${NATS_PORT}:4222" "${NATS_IMAGE}"
    fi

    # Wait for NATS to be ready
    info "Waiting for NATS to accept connections..."
    local retries=10
    while ! docker exec "${NATS_CONTAINER}" nats-server --signal ping 2>/dev/null && [ "$retries" -gt 0 ]; do
        sleep 0.5
        retries=$((retries - 1))
    done

    # Verify connectivity
    if command -v nats &>/dev/null; then
        nats server check connection --server "nats://localhost:${NATS_PORT}" && \
            ok "NATS server verified via nats CLI" || \
            warn "nats CLI check failed — server may still be starting"
    else
        info "nats CLI not found; skipping CLI verification"
        info "You can verify with: curl -s telnet://localhost:${NATS_PORT}"
    fi

    ok "NATS server is running on nats://localhost:${NATS_PORT}"
}

stop_nats() {
    info "Stopping NATS server..."
    if docker ps --format '{{.Names}}' | grep -q "^${NATS_CONTAINER}$"; then
        docker stop "${NATS_CONTAINER}"
        ok "NATS container stopped"
    else
        warn "NATS container '${NATS_CONTAINER}' is not running"
    fi
}

# ---------------------------------------------------------------------------
# 2. Web Server Management
# ---------------------------------------------------------------------------
#
# Start the web server with NATS enabled. You should see in output:
#
#   +----------------------------------------------------------+
#   |  NATS: enabled (nats://localhost:4222)                    |
#   +----------------------------------------------------------+
#
# And shortly after:
#
#   [NATS] Connected to nats://localhost:4222
#   [NATS] Ready for SSE subscriptions
#

start_web() {
    info "Starting web server with NATS enabled..."

    if ! [ -d "${WEB_DIR}" ]; then
        err "Web directory '${WEB_DIR}' not found."
        err "Set WEB_DIR to the path of the web/ directory, or run from the project root."
        exit 1
    fi

    info "Web server will start in foreground. Press Ctrl+C to stop."
    echo ""
    echo "  SCION_NATS_URL=nats://localhost:${NATS_PORT}"
    echo "  DEV_AUTH=true"
    echo "  PORT=${WEB_PORT}"
    echo ""

    cd "${WEB_DIR}"
    SCION_NATS_URL="nats://localhost:${NATS_PORT}" DEV_AUTH=true PORT="${WEB_PORT}" npm run dev
}

# ---------------------------------------------------------------------------
# Status: check health of all components
# ---------------------------------------------------------------------------

check_status() {
    echo ""
    info "=== Component Status ==="
    echo ""

    # NATS
    if docker ps --format '{{.Names}}' | grep -q "^${NATS_CONTAINER}$"; then
        ok "NATS: running (container: ${NATS_CONTAINER}, port: ${NATS_PORT})"
    else
        warn "NATS: not running"
    fi

    # Web server health endpoints
    # Liveness probe — always 200 if server is running
    if curl -sf "http://localhost:${WEB_PORT}/healthz" >/dev/null 2>&1; then
        ok "Web /healthz: healthy"
        info "  $(curl -s "http://localhost:${WEB_PORT}/healthz")"
    else
        warn "Web /healthz: not reachable (server may not be running)"
    fi

    # Readiness probe — includes NATS status
    #   When connected:    200 { "status": "healthy",   "nats": "connected" }
    #   When disconnected: 503 { "status": "unhealthy", "nats": "reconnecting" }
    #   When NATS disabled:200 { "status": "healthy" }  (no nats field)
    local http_code
    http_code=$(curl -s -o /dev/null -w '%{http_code}' "http://localhost:${WEB_PORT}/readyz" 2>/dev/null || echo "000")
    if [ "$http_code" = "200" ]; then
        ok "Web /readyz: healthy (HTTP 200)"
        info "  $(curl -s "http://localhost:${WEB_PORT}/readyz")"
    elif [ "$http_code" = "503" ]; then
        warn "Web /readyz: unhealthy (HTTP 503) — NATS may be disconnected"
        info "  $(curl -s "http://localhost:${WEB_PORT}/readyz")"
    else
        warn "Web /readyz: not reachable (HTTP ${http_code})"
    fi

    echo ""
}

# ---------------------------------------------------------------------------
# Teardown: stop and remove the NATS container
# ---------------------------------------------------------------------------

teardown() {
    info "Tearing down NATS container..."
    if docker ps -a --format '{{.Names}}' | grep -q "^${NATS_CONTAINER}$"; then
        docker stop "${NATS_CONTAINER}" 2>/dev/null || true
        docker rm "${NATS_CONTAINER}"
        ok "NATS container removed"
    else
        warn "NATS container '${NATS_CONTAINER}' does not exist"
    fi
}

# ---------------------------------------------------------------------------
# Graceful Shutdown Sequence
# ---------------------------------------------------------------------------
#
# When the web server receives SIGTERM or SIGINT, the shutdown order is:
#
#   1. SSE Manager closes all active SSE connections (unsubscribes NATS, ends streams)
#   2. NATS client drains (finishes in-flight messages) then closes
#   3. HTTP server closes
#   4. Force shutdown after 10 seconds if steps don't complete
#
# Expected server logs:
#
#   SIGTERM received. Shutting down gracefully...
#   [SSE] Closing all connections...
#   [NATS] Draining connection...
#   [NATS] Connection drained and closed
#   Server closed successfully
#
# To test graceful shutdown:
#
#   kill -SIGTERM $(pgrep -f "tsx.*index.ts")
#

# ---------------------------------------------------------------------------
# Server-Side Log Messages Reference
# ---------------------------------------------------------------------------
#
#   [NATS] Connected to ...              Successfully connected to NATS server
#   [NATS] Disconnected: ...             Lost connection, will attempt reconnect
#   [NATS] Reconnecting...               Actively attempting to reconnect
#   [NATS] Reconnected to ...            Successfully reconnected
#   [NATS] Draining connection...         Graceful shutdown in progress
#   [NATS] Connection drained and closed  Shutdown complete
#   [NATS] Ready for SSE subscriptions    Initial connection established
#   [SSE] Failed to subscribe to ...      Subscription error during connection setup
#   [SSE] Closing all connections...      Server shutdown closing SSE streams
#

# ---------------------------------------------------------------------------
# Main entry point
# ---------------------------------------------------------------------------

usage() {
    echo "Usage: $0 [start|stop|restart|status|teardown]"
    echo ""
    echo "Commands:"
    echo "  start     Start NATS and the web server (foreground)"
    echo "  stop      Stop the NATS container"
    echo "  restart   Restart the NATS container"
    echo "  status    Check health of all components"
    echo "  teardown  Stop and remove the NATS container"
    echo ""
    echo "Environment:"
    echo "  WEB_DIR   Path to web/ directory (default: web)"
    echo "  PORT      Web server port (default: 8080)"
}

case "${1:-start}" in
    start)
        start_nats
        start_web
        ;;
    stop)
        stop_nats
        ;;
    restart)
        stop_nats
        sleep 1
        start_nats
        ;;
    status)
        check_status
        ;;
    teardown)
        teardown
        ;;
    -h|--help|help)
        usage
        ;;
    *)
        err "Unknown command: $1"
        usage
        exit 1
        ;;
esac
