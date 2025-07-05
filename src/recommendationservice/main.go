// recommendationservice-go/main.go

package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	pb "github.com/norun9/microservices-demo-ambient/genproto"
	"github.com/norun9/microservices-demo-ambient/src/recommendationservice/services"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	ctx := context.Background()

	// Configure logging.
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
	log.SetOutput(os.Stderr)

	// ----------------------------------------------------------------
	// 1) Initialize OpenTelemetry TracerProvider.
	log.Println("Initializing OpenTelemetry TracerProvider...")
	tp, err := initTracerProvider(ctx)
	if err != nil {
		log.Fatalf("failed to initialize tracer provider: %v", err)
	}
	defer func() {
		if err := tp.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}()
	log.Println("OpenTelemetry TracerProvider initialized successfully")
	// ----------------------------------------------------------------

	// ----------------------------------------------------------------
	// 2) Create RecommendationService.
	log.Println("Initializing RecommendationService...")
	recommendationSvc, err := services.NewRecommendationService()
	if err != nil {
		log.Fatalf("failed to create RecommendationService: %v", err)
	}
	log.Println("RecommendationService initialized successfully")
	// ----------------------------------------------------------------

	// ----------------------------------------------------------------
	// 3) Start gRPC server.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := fmt.Sprintf(":%s", port)
	log.Printf("Starting gRPC server on %s\n", addr)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", addr, err)
	}
	log.Println("Successfully created TCP listener")

	// Add OTel interceptor to gRPC server.
	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	log.Println("Created gRPC server with OpenTelemetry interceptors")

	// Register RecommendationService and HealthCheckService.
	pb.RegisterRecommendationServiceServer(grpcServer, recommendationSvc)
	log.Println("Registered RecommendationService")

	healthSvc := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthSvc)
	healthSvc.SetServingStatus("recommendationservice", healthpb.HealthCheckResponse_SERVING)
	log.Println("Registered HealthCheckService")

	// Configure graceful shutdown.
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		log.Println("Received shutdown signal, initiating graceful shutdown...")
		grpcServer.GracefulStop()
	}()

	// Final check before starting the server.
	log.Println("All services registered, starting gRPC server...")

	// Try to start the server.
	log.Printf("RecommendationService gRPC server is listening on %s\n", addr)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve gRPC server: %v", err)
	}
	// ----------------------------------------------------------------
}

// initTracerProvider initializes an OpenTelemetry TracerProvider and sets up the OTLP exporter.
// The Collector endpoint is specified via the OTEL_EXPORTER_OTLP_ENDPOINT environment variable.
// Example: OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4317
func initTracerProvider(ctx context.Context) (*sdktrace.TracerProvider, error) {
	// 1) Configure OTLP gRPC exporter.
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "dns:///otel-collector.observability.svc.cluster.local:4317"
	}
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// 2) Set up resource information (service name, version, etc.).
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String("recommendationservice"),
			semconv.ServiceVersionKey.String("v1.0.0"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// 3) Build TracerProvider.
	bsp := sdktrace.NewBatchSpanProcessor(exporter)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()), // Consider TraceIDRatioBased for production.
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)
	otel.SetTracerProvider(tp)

	// 4) Configure to use W3C Trace Context.
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return tp, nil
}
