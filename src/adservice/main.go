package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"time"

	pb "github.com/norun9/microservices-demo-ambient/genproto"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

var meter metric.Meter

// adServiceServer implements the AdServiceServer interface.
type adServiceServer struct {
	pb.UnimplementedAdServiceServer
	tracer trace.Tracer
}

// adsMap holds ads by category.
var adsMap = map[string][]*pb.Ad{
	"clothing":    {{RedirectUrl: "/product/66VCHSJNUP", Text: "Tank top for sale. 20% off."}},
	"accessories": {{RedirectUrl: "/product/1YMWWN1N4O", Text: "Watch for sale. Buy one, get second one for free"}},
	"footwear":    {{RedirectUrl: "/product/L9ECAV7KIM", Text: "Loafers for sale. Buy one, get second one for free"}},
	"hair":        {{RedirectUrl: "/product/2ZYFJ3GM2N", Text: "Hairdryer for sale. 50% off."}},
	"decor":       {{RedirectUrl: "/product/0PUK6V6EV0", Text: "Candle holder for sale. 30% off."}},
	"kitchen": {
		{RedirectUrl: "/product/9SIQT8TOJO", Text: "Bamboo glass jar for sale. 10% off."},
		{RedirectUrl: "/product/6E92ZMYYFZ", Text: "Mug for sale. Buy two, get third one for free"},
	},
}

// GetAds implements AdService.GetAds.
func (s *adServiceServer) GetAds(ctx context.Context, req *pb.AdRequest) (*pb.AdResponse, error) {
	ctx, span := s.tracer.Start(ctx, "GetAds")
	defer span.End()

	log.Printf("received GetAds request: context_keys=%v", req.ContextKeys)
	span.SetAttributes(attribute.StringSlice("app.context_keys", req.ContextKeys))
	span.SetAttributes(attribute.Int("app.ads_requested", len(req.ContextKeys)))

	var allAds []*pb.Ad
	for _, key := range req.ContextKeys {
		if ads, ok := adsMap[key]; ok {
			allAds = append(allAds, ads...)
		}
	}
	if len(allAds) == 0 {
		allAds = getRandomAds()
	}
	span.SetAttributes(attribute.Int("app.ads_served", len(allAds)))

	resp := &pb.AdResponse{
		Ads: allAds,
	}
	return resp, nil
}

// getRandomAds returns 2 random ads.
func getRandomAds() []*pb.Ad {
	var all []*pb.Ad
	for _, ads := range adsMap {
		all = append(all, ads...)
	}
	rand.Seed(time.Now().UnixNano())
	res := []*pb.Ad{}
	for i := 0; i < 2; i++ {
		idx := rand.Intn(len(all))
		res = append(res, all[idx])
	}
	return res
}

func main() {
	ctx := context.Background()
	// 1) Initialize OpenTelemetry TracerProvider.
	tp, err := initTracerProvider(ctx)
	if err != nil {
		log.Fatalf("failed to initialize tracer provider: %v", err)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}()

	// 2) Initialize OpenTelemetry MeterProvider.
	mp, err := initMeterProvider(ctx)
	if err != nil {
		log.Fatalf("failed to initialize meter provider: %v", err)
	}
	defer func() {
		if err := mp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down meter provider: %v", err)
		}
	}()

	// 2) Create gRPC server.
	port := os.Getenv("PORT")
	if port == "" {
		port = "9555"
	}
	addr := fmt.Sprintf(":%s", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", addr, err)
	}

	// Add OpenTelemetry StatsHandler to the gRPC server.
	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)

	// 3) Register AdService server.
	pb.RegisterAdServiceServer(grpcServer, &adServiceServer{
		tracer: otel.Tracer("adservice"),
	})

	// 4) Register health check service.
	healthSvc := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthSvc)
	healthSvc.SetServingStatus("adservice", grpc_health_v1.HealthCheckResponse_SERVING)

	log.Printf("AdService gRPC server started, listening on %s", addr)
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("failed to serve gRPC server: %v", err)
	}
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
			semconv.ServiceNameKey.String("adservice"),
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

// initMeterProvider initializes an OpenTelemetry MeterProvider and sets up the OTLP exporter.
// The Collector endpoint is specified via the OTEL_EXPORTER_OTLP_ENDPOINT environment variable.
func initMeterProvider(ctx context.Context) (*sdkmetric.MeterProvider, error) {
	// 1) Configure OTLP gRPC exporter for metrics.
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "dns:///otel-collector.observability.svc.cluster.local:4317"
	}

	metricExporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(endpoint),
		otlpmetricgrpc.WithInsecure(),                   // Use WithInsecure for plain-text communication
		otlpmetricgrpc.WithDialOption(grpc.WithBlock()), // Block until connection is established
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP metric exporter: %w", err)
	}

	// 2) Set up resource information (service name, version, etc.).
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String("adservice"), // サービス名を��定
			semconv.ServiceVersionKey.String("v1.0.0"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// 3) Build MeterProvider.
	// PeriodicReader is used to push metrics at a regular interval.
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter, sdkmetric.WithInterval(10*time.Second))), // 10秒ごとにエクスポート
	)

	// 4) Set the global MeterProvider.
	otel.SetMeterProvider(meterProvider)

	// Initialize the meter
	meter = otel.Meter("adservice") // メーターの名前を設定

	return meterProvider, nil
}
