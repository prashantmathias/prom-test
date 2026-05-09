package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

var tracer trace.Tracer

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

func handler(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "handler")
	defer span.End()

	_ = ctx

	span.SetAttributes(attribute.String("http.method", r.Method))
	span.SetAttributes(attribute.String("http.url", r.URL.Path))

	time.Sleep(100 * time.Millisecond)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Hello from Server 2"))
}

func main() {
	shutdown := initTracer()
	defer shutdown()

	http.HandleFunc("/", handler)
	log.Println("Server 2 listening on :8082")
	log.Fatal(http.ListenAndServe(":8082", nil))
}
