# ─────────────────────────────────────────────────────────────
#  start.ps1  –  build, start both servers + Prometheus, then
#               open the dashboard
# ─────────────────────────────────────────────────────────────

$ErrorActionPreference = "Stop"
$root = $PSScriptRoot

function Write-Step($msg) {
    Write-Host "`n>> $msg" -ForegroundColor Cyan
}

function Wait-ForHttp($url, $label) {
    Write-Host "   Waiting for $label ($url)..." -NoNewline
    $timeout = 30
    $elapsed = 0
    while ($elapsed -lt $timeout) {
        try {
            $r = Invoke-WebRequest -Uri $url -UseBasicParsing -TimeoutSec 1 -ErrorAction Stop
            if ($r.StatusCode -lt 500) {
                Write-Host " ready!" -ForegroundColor Green
                return
            }
        } catch {}
        Start-Sleep -Seconds 1
        $elapsed++
        Write-Host -NoNewline "."
    }
    Write-Host " TIMED OUT" -ForegroundColor Red
}

# ── 1. Build servers ──────────────────────────────────────────
Write-Step "Building server1..."
Push-Location "$root\server1"
go build -o server1.exe .
Pop-Location

Write-Step "Building server2..."
Push-Location "$root\server2"
go build -o server2.exe .
Pop-Location

# ── 2. Start server1 in a new window ─────────────────────────
Write-Step "Starting Server 1 (Order Service) on :8081..."
$s1cmd = "cd '$root\server1'; Write-Host 'Server 1 - Order Service' -ForegroundColor Yellow; .\server1.exe"
Start-Process powershell -ArgumentList @("-NoExit", "-Command", $s1cmd)

# ── 3. Start server2 in a new window ─────────────────────────
Write-Step "Starting Server 2 (User Service) on :8082..."
$s2cmd = "cd '$root\server2'; Write-Host 'Server 2 - User Service' -ForegroundColor Yellow; .\server2.exe"
Start-Process powershell -ArgumentList @("-NoExit", "-Command", $s2cmd)

# ── 4. Start Prometheus via Docker Compose ───────────────────
Write-Step "Starting Prometheus (Docker)..."
Push-Location $root
docker compose up -d
Pop-Location

# ── 5. Wait for everything to be ready ───────────────────────
Write-Step "Waiting for services..."
Wait-ForHttp "http://localhost:8081/metrics" "Server 1"
Wait-ForHttp "http://localhost:8082/metrics" "Server 2"
Wait-ForHttp "http://localhost:9090/-/ready"  "Prometheus"

# ── 6. Open Prometheus dashboard ─────────────────────────────
Write-Step "Opening Prometheus..."
Start-Process "http://localhost:9090"

Write-Host ""
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Green
Write-Host "  All services are running!" -ForegroundColor Green
Write-Host ""
Write-Host "  Prometheus  →  http://localhost:9090" -ForegroundColor White
Write-Host "  Server 1    →  http://localhost:8081" -ForegroundColor White
Write-Host "  Server 2    →  http://localhost:8082" -ForegroundColor White
Write-Host ""
Write-Host "  Generate data:" -ForegroundColor Yellow
Write-Host "    curl -X POST   http://localhost:8081/order"
Write-Host "    curl -X DELETE http://localhost:8081/order"
Write-Host "    curl -X POST   http://localhost:8082/login"
Write-Host "    curl -X POST   http://localhost:8082/logout"
Write-Host ""
Write-Host "  To stop everything:" -ForegroundColor Yellow
Write-Host "    docker compose -f '$root\docker-compose.yml' down"
Write-Host "    # then close the two server terminal windows"
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Green
