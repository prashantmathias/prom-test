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
	ordersTotal       metric.Int64Counter       // Counter  – goes up only
	activeOrders      metric.Int64UpDownCounter // Gauge    – can go up & down
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
			semconv.ServiceName("server1"),
			semconv.ServiceVersion("1.0.0"),
		)),
	)
	otel.SetTracerProvider(tp)
	tracer = tp.Tracer("server1")
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		tp.Shutdown(ctx)
	}
}

// ── Meter (Prometheus) setup ─────────────────────────────────────────────────

func initMeter() func() {
	// otelprom.New() creates a Prometheus exporter that wires directly into
	// the default Prometheus registry – promhttp.Handler() will serve it.
	exporter, err := otelprom.New()
	if err != nil {
		log.Fatal(err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
		sdkmetric.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("server1"),
		)),
	)
	otel.SetMeterProvider(mp)

	meter := mp.Meter("server1")

	// ── COUNTER: http_requests_total ─────────────────────────────────────────
	// A counter only ever increases. Perfect for "how many times did X happen?"
	httpRequestsTotal, err = meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total HTTP requests received, labelled by method and path"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// ── COUNTER: orders_total ────────────────────────────────────────────────
	// Same idea – labelled by status so you can split created vs completed.
	ordersTotal, err = meter.Int64Counter(
		"orders_total",
		metric.WithDescription("Total orders processed, labelled by status (created|completed)"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// ── GAUGE (UpDownCounter): active_orders ─────────────────────────────────
	// Int64UpDownCounter is the right instrument when a value can go up AND down
	// (e.g. concurrent users, queue depth, open connections).
	activeOrders, err = meter.Int64UpDownCounter(
		"active_orders",
		metric.WithDescription("Number of orders currently in-flight (gauge – goes up and down)"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// ── GAUGE (ObservableGauge): order_queue_depth ───────────────────────────
	// Use an ObservableGauge when you want to *read* a value at scrape time
	// rather than track it yourself (e.g. reading from a queue, memory, etc.).
	// The callback is called every time Prometheus scrapes /metrics.
	_, err = meter.Int64ObservableGauge(
		"order_queue_depth",
		metric.WithDescription("Simulated order queue depth – observed at scrape time"),
		metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
			o.Observe(rand.Int63n(50)) // pretend we're reading from a real queue
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
	w.Write([]byte("Hello from Server 1 – Order Service\n\n" +
		"Try:\n" +
		"  curl -X POST   http://localhost:8081/order   (create order)\n" +
		"  curl -X DELETE http://localhost:8081/order   (complete order)\n" +
		"  curl          http://localhost:8081/metrics  (Prometheus metrics)\n"))
}

func orderHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "orderHandler")
	defer span.End()
	span.SetAttributes(attribute.String("http.method", r.Method))

	httpRequestsTotal.Add(ctx, 1,
		metric.WithAttributes(attribute.String("method", r.Method), attribute.String("path", "/order")),
	)

	switch r.Method {

	case http.MethodPost:
		// ↑ Counter goes up by 1 – it will never decrease
		ordersTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "created")))
		// ↑ Gauge goes up by 1 – order is now active
		activeOrders.Add(ctx, 1)

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("Order created! (orders_total{status=created}++ | active_orders++)\n"))

	case http.MethodDelete:
		// ↑ Counter goes up – completed orders accumulate forever
		ordersTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "completed")))
		// ↓ Gauge goes DOWN – one fewer active order
		activeOrders.Add(ctx, -1)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Order completed! (orders_total{status=completed}++ | active_orders--)\n"))

	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	shutdownTracer := initTracer()
	defer shutdownTracer()

	shutdownMeter := initMeter()
	defer shutdownMeter()

	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/order", orderHandler)

	// promhttp.Handler() serves everything registered in the default Prometheus
	// registry – the OTel Prometheus exporter registered itself there above.
	http.Handle("/metrics", promhttp.Handler())

	log.Println("Server 1 – Order Service – listening on :8081")
	log.Println("  POST   /order   → create an order")
	log.Println("  DELETE /order   → complete an order")
	log.Println("  GET    /metrics → Prometheus scrape endpoint")
	log.Fatal(http.ListenAndServe(":8081", nil))
}
