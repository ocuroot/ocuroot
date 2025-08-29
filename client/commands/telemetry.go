package commands

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/about"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
)

const name = "github.com/ocuroot/ocuroot/client/commands"

var (
	tracer = otel.Tracer(name)
	meter  = otel.Meter(name)
	logger = otelslog.NewLogger(name)
)

func setupTelemetry() func() {
	if os.Getenv("ENABLE_OTEL") != "" {
		log.Info("Enabling OpenTelemetry")

		// Create resource.
		res, err := newResource()
		if err != nil {
			panic(err)
		}

		ctx := context.Background()

		tp, err := initTracer(ctx, res)
		if err != nil {
			log.Fatal(err)
		}

		// mp, err := newMeterProvider(ctx, res)
		// if err != nil {
		// 	log.Fatal(err)
		// }

		return func() {
			if err := tp.Shutdown(context.Background()); err != nil {
				log.Printf("Error shutting down tracer provider: %v", err)
			}

			// if err := mp.Shutdown(context.Background()); err != nil {
			// 	log.Printf("Error shutting down meter provider: %v", err)
			// }
		}
	}

	return func() {}
}

func newResource() (*resource.Resource, error) {
	return resource.Merge(resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNamespace("ocuroot"),
			semconv.ServiceName("ocuroot"),
			semconv.ServiceVersion(about.Version),
		))
}

func initTracer(ctx context.Context, res *resource.Resource) (*trace.TracerProvider, error) {
	var options []otlptracehttp.Option

	otlpURL := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if otlpURL == "" {
		otlpURL = "http://localhost:4318"
	}

	options = append(options, otlptracehttp.WithEndpointURL(otlpURL))
	if strings.HasPrefix(otlpURL, "http://") {
		options = append(options, otlptracehttp.WithInsecure())
	}

	headers := os.Getenv("OTEL_EXPORTER_OTLP_HEADERS")
	if headers != "" {
		headerMap := make(map[string]string)
		for _, h := range strings.Split(headers, ",") {
			headerParts := strings.Split(h, "=")
			if len(headerParts) != 2 {
				return nil, fmt.Errorf("invalid header format: %s", h)
			}
			headerMap[headerParts[0]] = headerParts[1]
		}
		options = append(options, otlptracehttp.WithHeaders(headerMap))
	}

	exporter, err := otlptracehttp.New(
		ctx,
		options...,
	)
	if err != nil {
		return nil, err
	}

	tp := trace.NewTracerProvider(
		trace.WithResource(res),
		trace.WithSampler(trace.AlwaysSample()),
		trace.WithBatcher(exporter),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return tp, nil
}
