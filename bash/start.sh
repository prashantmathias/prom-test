#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────
#  start.sh  –  build, start both servers + Prometheus, then
#               open the dashboard
# ─────────────────────────────────────────────────────────────

set -eu

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

CYAN=$'\033[36m'
GREEN=$'\033[32m'
RED=$'\033[31m'
YELLOW=$'\033[33m'
RESET=$'\033[0m'

step() {
    printf '\n%s>> %s%s\n' "$CYAN" "$1" "$RESET"
}

wait_for_http() {
    local url="$1" label="$2"
    printf '   Waiting for %s (%s)...' "$label" "$url"
    local timeout=30 elapsed=0 code
    while [ "$elapsed" -lt "$timeout" ]; do
        code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 1 "$url" || echo 000)
        if [ "$code" != "000" ] && [ "$code" -lt 500 ]; then
            printf ' %sready!%s\n' "$GREEN" "$RESET"
            return 0
        fi
        sleep 1
        elapsed=$((elapsed + 1))
        printf '.'
    done
    printf ' %sTIMED OUT%s\n' "$RED" "$RESET"
}

# Pick a terminal opener for launching the servers in new windows.
open_terminal() {
    local title="$1" workdir="$2" cmd="$3"
    case "$(uname -s)" in
        Darwin)
            osascript <<EOF >/dev/null
tell application "Terminal"
    activate
    do script "cd '$workdir' && echo '$title' && $cmd"
end tell
EOF
            ;;
        Linux)
            if command -v gnome-terminal >/dev/null 2>&1; then
                gnome-terminal --title="$title" -- bash -c "cd '$workdir' && echo '$title' && $cmd; exec bash"
            elif command -v xterm >/dev/null 2>&1; then
                xterm -T "$title" -e "bash -c \"cd '$workdir' && echo '$title' && $cmd; exec bash\"" &
            else
                echo "No supported terminal emulator found; running '$title' in background." >&2
                ( cd "$workdir" && nohup bash -c "$cmd" >"$title.log" 2>&1 & )
            fi
            ;;
        *)
            ( cd "$workdir" && nohup bash -c "$cmd" >"$title.log" 2>&1 & )
            ;;
    esac
}

open_url() {
    local url="$1"
    case "$(uname -s)" in
        Darwin) open "$url" ;;
        Linux)  xdg-open "$url" >/dev/null 2>&1 || true ;;
        *)      echo "Open $url in your browser." ;;
    esac
}

# ── 1. Build servers ──────────────────────────────────────────
step "Building server1..."
( cd "$ROOT/server1" && go build -o server1 . )

step "Building server2..."
( cd "$ROOT/server2" && go build -o server2 . )

# ── 2. Start server1 in a new window ─────────────────────────
step "Starting Server 1 (Order Service) on :8081..."
open_terminal "Server 1 - Order Service" "$ROOT/server1" "./server1"

# ── 3. Start server2 in a new window ─────────────────────────
step "Starting Server 2 (User Service) on :8082..."
open_terminal "Server 2 - User Service" "$ROOT/server2" "./server2"

# ── 4. Start Prometheus via Docker Compose ───────────────────
step "Starting Prometheus (Docker)..."
( cd "$ROOT" && docker compose up -d )

# ── 5. Wait for everything to be ready ───────────────────────
step "Waiting for services..."
wait_for_http "http://localhost:8081/metrics" "Server 1"
wait_for_http "http://localhost:8082/metrics" "Server 2"
wait_for_http "http://localhost:9090/-/ready"  "Prometheus"

# ── 6. Open Prometheus dashboard ─────────────────────────────
step "Opening Prometheus..."
open_url "http://localhost:9090"

echo
printf '%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n' "$GREEN" "$RESET"
printf '%s  All services are running!%s\n' "$GREEN" "$RESET"
echo
echo "  Prometheus  ->  http://localhost:9090"
echo "  Server 1    ->  http://localhost:8081"
echo "  Server 2    ->  http://localhost:8082"
echo
printf '%s  Generate data:%s\n' "$YELLOW" "$RESET"
echo '    curl -X POST   http://localhost:8081/order'
echo '    curl -X DELETE http://localhost:8081/order'
echo '    curl -X POST   http://localhost:8082/login'
echo '    curl -X POST   http://localhost:8082/logout'
echo
printf '%s  To stop everything:%s\n' "$YELLOW" "$RESET"
echo "    docker compose -f '$ROOT/docker-compose.yml' down"
echo "    # then close the two server terminal windows"
printf '%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n' "$GREEN" "$RESET"
