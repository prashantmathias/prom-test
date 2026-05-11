# ─────────────────────────────────────────────────────────────
#  populate.ps1  –  fires curl requests at both servers to
#                   generate interesting metric data
# ─────────────────────────────────────────────────────────────

function Write-Step($msg) {
    Write-Host "`n>> $msg" -ForegroundColor Cyan
}

function Invoke-Hit($method, $url, $label) {
    try {
        $response = Invoke-WebRequest -Uri $url -Method $method -UseBasicParsing -ErrorAction Stop
        Write-Host "  [$method $url]  $($response.Content.Trim())" -ForegroundColor DarkGray
    } catch {
        Write-Host "  [$method $url]  FAILED - is the server running?" -ForegroundColor Red
    }
}

# ── Server 1 – Order Service ──────────────────────────────────

Write-Step "Creating 15 orders on Server 1..."
for ($i = 1; $i -le 15; $i++) {
    Invoke-Hit "POST" "http://localhost:8081/order" "create order"
    Start-Sleep -Milliseconds 200
}

Write-Step "Completing 7 orders on Server 1..."
for ($i = 1; $i -le 7; $i++) {
    Invoke-Hit "DELETE" "http://localhost:8081/order" "complete order"
    Start-Sleep -Milliseconds 200
}

Write-Step "Hitting Server 1 home page 10 times..."
for ($i = 1; $i -le 10; $i++) {
    Invoke-Hit "GET" "http://localhost:8081/" "home"
    Start-Sleep -Milliseconds 100
}

# ── Server 2 – User Service ───────────────────────────────────

Write-Step "Logging in 12 users on Server 2..."
for ($i = 1; $i -le 12; $i++) {
    Invoke-Hit "POST" "http://localhost:8082/login" "login"
    Start-Sleep -Milliseconds 200
}

Write-Step "Logging out 5 users on Server 2..."
for ($i = 1; $i -le 5; $i++) {
    Invoke-Hit "POST" "http://localhost:8082/logout" "logout"
    Start-Sleep -Milliseconds 200
}

Write-Step "Hitting Server 2 home page 10 times..."
for ($i = 1; $i -le 10; $i++) {
    Invoke-Hit "GET" "http://localhost:8082/" "home"
    Start-Sleep -Milliseconds 100
}

# ── Summary ───────────────────────────────────────────────────

Write-Host ""
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Green
Write-Host "  Done! Check Prometheus at http://localhost:9090" -ForegroundColor Green
Write-Host ""
Write-Host "  Queries to try:" -ForegroundColor Yellow
Write-Host "    orders_total"
Write-Host "    orders_total{status=`"created`"}"
Write-Host "    orders_total{status=`"completed`"}"
Write-Host "    active_orders"
Write-Host "    user_logins_total"
Write-Host "    active_sessions"
Write-Host "    http_requests_total"
Write-Host "    rate(orders_total[1m])"
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Green
