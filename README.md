# OpenTelemetry + Prometheus – Learning Project

A hands-on project for learning how OpenTelemetry metrics work with Prometheus.
Two Go HTTP servers each expose a `/metrics` endpoint that Prometheus scrapes every 5 seconds.

---

## Project Structure

```
zed/
├── server1/          # Order Service  (port 8081)
│   └── main.go
├── server2/          # User Service   (port 8082)
│   └── main.go
├── prometheus.yml    # Prometheus scrape config
├── docker-compose.yml
├── start.ps1         # Starts everything + opens the browser
├── populate.ps1      # Fires requests to generate metric data
└── README.md
```

---

## Prerequisites

- [Go 1.21+](https://go.dev/dl/)
- [Docker Desktop](https://www.docker.com/products/docker-desktop/)

---

## Quick Start

**1. Start everything**
```powershell
powershell -ExecutionPolicy Bypass -File .\start.ps1
```
This will:
- Build both servers
- Open each server in its own terminal window
- Start Prometheus in Docker
- Wait for all three services to be healthy
- Open http://localhost:9090 in your browser

**2. Populate data**
```powershell
powershell -ExecutionPolicy Bypass -File .\populate.ps1
```
Fires a batch of requests at both servers. Run it as many times as you like —
counters keep climbing and gauges reflect the latest state.

**3. Stop everything**
```powershell
docker compose down
# then close the two server terminal windows
```

---

## Services

| Service | URL | What it does |
|---|---|---|
| Prometheus UI | http://localhost:9090 | Query and graph metrics |
| Server 1 – Order Service | http://localhost:8081 | Creates / completes orders |
| Server 2 – User Service | http://localhost:8082 | Handles user logins / logouts |

---

## Endpoints

### Server 1 – Order Service (:8081)

| Method | Path | What it does |
|---|---|---|
| `GET` | `/` | Hello page |
| `POST` | `/order` | Create an order |
| `DELETE` | `/order` | Complete an order |
| `GET` | `/metrics` | Prometheus metrics scrape endpoint |

### Server 2 – User Service (:8082)

| Method | Path | What it does |
|---|---|---|
| `GET` | `/` | Hello page |
| `POST` | `/login` | Log a user in |
| `POST` | `/logout` | Log a user out |
| `GET` | `/metrics` | Prometheus metrics scrape endpoint |

---

## Metrics Reference

### Server 1 – Order Service

| Metric | Type | Description |
|---|---|---|
| `orders_total` | Counter | Total orders processed. Has a `status` label: `created` or `completed` |
| `active_orders` | Gauge | Orders currently in-flight. Goes up on `POST /order`, down on `DELETE /order` |
| `order_queue_depth` | Observable Gauge | Simulated queue depth. Re-read from a callback on every Prometheus scrape |
| `http_requests_total` | Counter | All HTTP requests. Labelled by `method` and `path` |

### Server 2 – User Service

| Metric | Type | Description |
|---|---|---|
| `user_logins_total` | Counter | Total successful logins since server start |
| `active_sessions` | Gauge | Users currently logged in. Goes up on login, down on logout |
| `cpu_temperature_celsius` | Observable Gauge | Simulated CPU temperature (~60°C ±15°C). Re-read on every scrape |
| `http_requests_total` | Counter | All HTTP requests. Labelled by `method` and `path` |

---

## Metric Types Explained

### Counter
> A value that **only ever goes up**. Resets to zero when the server restarts.

Use it for: total requests, total errors, total jobs processed.

In Prometheus you rarely query the raw counter — you ask for its **rate of change**:
```
rate(orders_total[1m])   # orders per second, averaged over the last minute
```

### Gauge (UpDownCounter)
> A value that **goes up and down**. Reflects the current state of something.

Use it for: active connections, queue depth, in-flight requests, memory usage.

Query the raw value directly:
```
active_orders      # how many orders are being processed right now
active_sessions    # how many users are currently logged in
```

### Observable Gauge
> Like a gauge, but instead of you updating it, you **register a callback** that
> OpenTelemetry calls at scrape time. The callback reads and returns the current value.

Use it for: metrics you read rather than track yourself — CPU temperature,
memory from the OS, a count from an external queue, file descriptor usage.

```go
meter.Int64ObservableGauge("order_queue_depth",
    metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
        o.Observe(readQueueDepthFromSomewhere())
        return nil
    }),
)
```

---

## How it Fits Together

```
┌─────────────────────────────────────────────────────────────┐
│                        Your Machine                         │
│                                                             │
│   ┌──────────────────┐       ┌──────────────────┐          │
│   │  server1 :8081   │       │  server2 :8082   │          │
│   │  (Order Service) │       │  (User Service)  │          │
│   │                  │       │                  │          │
│   │  GET /metrics ◄──┼───┐   │  GET /metrics ◄──┼───┐     │
│   └──────────────────┘   │   └──────────────────┘   │     │
│                           │                           │     │
│   ┌───────────────────────┴───────────────────────┐  │     │
│   │            Prometheus  :9090  (Docker)        │  │     │
│   │                                               │  │     │
│   │  Scrapes both /metrics every 5 seconds  ──────┘  │     │
│   │  Stores time-series data                         │     │
│   │  Serves query UI at http://localhost:9090        │     │
│   └───────────────────────────────────────────────────     │
└─────────────────────────────────────────────────────────────┘
```

1. Each server uses the **OTel Go SDK** to define metric instruments (counters, gauges)
2. The **OTel Prometheus exporter** bridges those instruments into the Prometheus registry
3. Prometheus **scrapes `/metrics`** on each server every 5 seconds and stores the data
4. You query and graph the data in the **Prometheus UI** at http://localhost:9090

---

## Useful Prometheus Queries

Paste these into the search bar at http://localhost:9090 and switch to the **Graph** tab.

```
# Raw counters (cumulative totals)
orders_total
orders_total{status="created"}
orders_total{status="completed"}
user_logins_total

# Gauges (current state – watch these go up and down)
active_orders
active_sessions
order_queue_depth
cpu_temperature_celsius

# Rate of change (how fast is a counter increasing?)
rate(orders_total[1m])
rate(user_logins_total[1m])
rate(http_requests_total[1m])

# HTTP requests broken down by server, method, and path
http_requests_total
```

---

## How OpenTelemetry + Prometheus Connect

The key is the **Prometheus exporter** — a single line that wires OTel into Prometheus:

```go
// Creates an exporter that registers itself in the default Prometheus registry
exporter, _ := otelprom.New()

// Wire it into the OTel MeterProvider
mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter))
otel.SetMeterProvider(mp)
```

Then `promhttp.Handler()` serves whatever is in that registry at `/metrics`:

```go
http.Handle("/metrics", promhttp.Handler())
```

Everything you record via `meter.Int64Counter(...)` or `meter.Int64UpDownCounter(...)`
flows through the exporter and appears in the `/metrics` output automatically.
