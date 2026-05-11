#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────
#  populate.sh  –  fires curl requests at both servers to
#                  generate interesting metric data
# ─────────────────────────────────────────────────────────────

set -u

# ANSI colours
CYAN=$'\033[36m'
GRAY=$'\033[90m'
RED=$'\033[31m'
GREEN=$'\033[32m'
YELLOW=$'\033[33m'
RESET=$'\033[0m'

step() {
    printf '\n%s>> %s%s\n' "$CYAN" "$1" "$RESET"
}

hit() {
    local method="$1" url="$2"
    local body
    if body=$(curl -sS -X "$method" "$url" 2>/dev/null); then
        printf '  %s[%s %s]  %s%s\n' "$GRAY" "$method" "$url" "$body" "$RESET"
    else
        printf '  %s[%s %s]  FAILED - is the server running?%s\n' "$RED" "$method" "$url" "$RESET"
    fi
}

# ── Server 1 – Order Service ──────────────────────────────────

step "Creating 15 orders on Server 1..."
for i in $(seq 1 15); do
    hit POST "http://localhost:8081/order"
    sleep 0.2
done

step "Completing 7 orders on Server 1..."
for i in $(seq 1 7); do
    hit DELETE "http://localhost:8081/order"
    sleep 0.2
done

step "Hitting Server 1 home page 10 times..."
for i in $(seq 1 10); do
    hit GET "http://localhost:8081/"
    sleep 0.1
done

# ── Server 2 – User Service ───────────────────────────────────

step "Logging in 12 users on Server 2..."
for i in $(seq 1 12); do
    hit POST "http://localhost:8082/login"
    sleep 0.2
done

step "Logging out 5 users on Server 2..."
for i in $(seq 1 5); do
    hit POST "http://localhost:8082/logout"
    sleep 0.2
done

step "Hitting Server 2 home page 10 times..."
for i in $(seq 1 10); do
    hit GET "http://localhost:8082/"
    sleep 0.1
done

# ── Summary ───────────────────────────────────────────────────

echo
printf '%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n' "$GREEN" "$RESET"
printf '%s  Done! Check Prometheus at http://localhost:9090%s\n' "$GREEN" "$RESET"
echo
printf '%s  Queries to try:%s\n' "$YELLOW" "$RESET"
echo '    orders_total'
echo '    orders_total{status="created"}'
echo '    orders_total{status="completed"}'
echo '    active_orders'
echo '    user_logins_total'
echo '    active_sessions'
echo '    http_requests_total'
echo '    rate(orders_total[1m])'
printf '%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n' "$GREEN" "$RESET"
