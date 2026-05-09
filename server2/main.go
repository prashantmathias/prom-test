package main

import (
	"context"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

var tracer trace.Tracer

// Metric instruments
var (
	httpRequestsTotal metric.Int64Counter       // Counter  – goes up only
	userLoginsTotal   metric.Int64Counter       // Counter  – goes up only
	activeSessions    metric.Int64UpDownCounter // Gauge    – can go up & down
)

// ── Tracer setup (unchanged from before) ────────────────────────────────────

func initTracer() func() {
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		log.Fatal(err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("server2"),
			semconv.ServiceVersion("1.0.0"),
		)),
	)
	otel.SetTracerProvider(tp)
	tracer = tp.Tracer("server2")
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		tp.Shutdown(ctx)
	}
}

// ── Meter (Prometheus) setup ─────────────────────────────────────────────────

func initMeter() func() {
	exporter, err := otelprom.New()
	if err != nil {
		log.Fatal(err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
		sdkmetric.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("server2"),
		)),
	)
	otel.SetMeterProvider(mp)

	meter := mp.Meter("server2")

	// ── COUNTER: http_requests_total ─────────────────────────────────────────
	httpRequestsTotal, err = meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total HTTP requests received, labelled by method and path"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// ── COUNTER: user_logins_total ───────────────────────────────────────────
	// Monotonically increasing – perfect for "how many logins have ever happened?"
	userLoginsTotal, err = meter.Int64Counter(
		"user_logins_total",
		metric.WithDescription("Total successful user logins since server start"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// ── GAUGE (UpDownCounter): active_sessions ───────────────────────────────
	// Tracks how many users are currently logged in.
	// It goes up on login and down on logout – classic gauge behaviour.
	activeSessions, err = meter.Int64UpDownCounter(
		"active_sessions",
		metric.WithDescription("Number of currently active user sessions (gauge – goes up and down)"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// ── GAUGE (ObservableGauge): cpu_temperature_celsius ─────────────────────
	// ObservableGauge is ideal for metrics you don't control directly –
	// you just read them when Prometheus asks (CPU temp, memory, file descriptors…).
	_, err = meter.Float64ObservableGauge(
		"cpu_temperature_celsius",
		metric.WithDescription("Simulated CPU temperature – observed at scrape time"),
		metric.WithFloat64Callback(func(_ context.Context, o metric.Float64Observer) error {
			// pretend we're reading from a sensor: 60°C ± 15°C
			o.Observe(60.0 + (rand.Float64()*30 - 15))
			return nil
		}),
	)
	if err != nil {
		log.Fatal(err)
	}

	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		mp.Shutdown(ctx)
	}
}

// ── Handlers ─────────────────────────────────────────────────────────────────

func homeHandler(w http.ResponseWriter, r *http.Request) {
	_, span := tracer.Start(r.Context(), "homeHandler")
	defer span.End()

	httpRequestsTotal.Add(r.Context(), 1,
		metric.WithAttributes(attribute.String("method", r.Method), attribute.String("path", "/")),
	)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Hello from Server 2 – User Service\n\n" +
		"Try:\n" +
		"  curl -X POST http://localhost:8082/login    (user login)\n" +
		"  curl -X POST http://localhost:8082/logout   (user logout)\n" +
		"  curl         http://localhost:8082/metrics  (Prometheus metrics)\n"))
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx, span := tracer.Start(r.Context(), "loginHandler")
	defer span.End()

	httpRequestsTotal.Add(ctx, 1,
		metric.WithAttributes(attribute.String("method", r.Method), attribute.String("path", "/login")),
	)

	// ↑ Counter – total logins grows forever (useful for rate-of-change in Prometheus)
	userLoginsTotal.Add(ctx, 1)
	// ↑ Gauge – one more active session right now
	activeSessions.Add(ctx, 1)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Logged in! (user_logins_total++ | active_sessions++)\n"))
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx, span := tracer.Start(r.Context(), "logoutHandler")
	defer span.End()

	httpRequestsTotal.Add(ctx, 1,
		metric.WithAttributes(attribute.String("method", r.Method), attribute.String("path", "/logout")),
	)

	// ↓ Gauge goes DOWN – one fewer active session
	activeSessions.Add(ctx, -1)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Logged out! (active_sessions--)\n"))
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	shutdownTracer := initTracer()
	defer shutdownTracer()

	shutdownMeter := initMeter()
	defer shutdownMeter()

	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/logout", logoutHandler)
	http.Handle("/metrics", promhttp.Handler())

	log.Println("Server 2 – User Service – listening on :8082")
	log.Println("  POST /login    → user login")
	log.Println("  POST /logout   → user logout")
	log.Println("  GET  /metrics  → Prometheus scrape endpoint")
	log.Fatal(http.ListenAndServe(":8082", nil))
}
