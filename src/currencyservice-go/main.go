// currencyservice-go/main.go

package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/norun9/microservices-demo-ambient/src/currencyservice-go/genproto/hipstershop"
	"github.com/norun9/microservices-demo-ambient/src/currencyservice-go/services"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"google.golang.org/grpc"

	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	ctx := context.Background()

	// ログの設定
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
	log.SetOutput(os.Stderr)

	// ----------------------------------------------------------------
	// 1) OpenTelemetry TracerProvider の初期化
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
	// 2) CurrencyService の生成
	log.Println("Initializing CurrencyService...")
	currencySvc, err := services.NewCurrencyService()
	if err != nil {
		log.Fatalf("failed to create CurrencyService: %v", err)
	}
	log.Println("CurrencyService initialized successfully")
	// ----------------------------------------------------------------

	// ----------------------------------------------------------------
	// 3) gRPC サーバーの起動
	port := os.Getenv("PORT")
	if port == "" {
		port = "7000"
	}
	addr := fmt.Sprintf(":%s", port)
	log.Printf("Starting gRPC server on %s\n", addr)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", addr, err)
	}
	log.Println("Successfully created TCP listener")

	// gRPC サーバーに OTel のインターセプターを組み込む
	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	log.Println("Created gRPC server with OpenTelemetry interceptors")

	// CurrencyService と HealthCheckService を登録
	hipstershop.RegisterCurrencyServiceServer(grpcServer, currencySvc)
	log.Println("Registered CurrencyService")

	healthSrv := services.NewHealthCheckService()
	healthpb.RegisterHealthServer(grpcServer, healthSrv)
	log.Println("Registered HealthCheckService")

	// グレースフルシャットダウンの設定
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		log.Println("Received shutdown signal, initiating graceful shutdown...")
		grpcServer.GracefulStop()
	}()

	// サーバー起動前に最終確認
	log.Println("All services registered, starting gRPC server...")

	// サーバー起動を試みる
	log.Printf("CurrencyService gRPC server is listening on %s\n", addr)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve gRPC server: %v", err)
	}
	// ----------------------------------------------------------------
}

// initTracerProvider は OpenTelemetry の TracerProvider を初期化し、OTLP エクスポーターを設定します。
// 環境変数 OTEL_EXPORTER_OTLP_ENDPOINT で Collector のエンドポイントを指定。
// 例: OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4317
func initTracerProvider(ctx context.Context) (*sdktrace.TracerProvider, error) {
	// 1) OTLP gRPC エクスポーター設定
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "otel-collector.observability.svc.cluster.local:4317"
	}
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// 2) リソース情報（サービス名・バージョンなど）を設定
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String("cartservice"),
			semconv.ServiceVersionKey.String("v1.0.0"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// 3) TracerProvider の構築
	bsp := sdktrace.NewBatchSpanProcessor(exporter)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()), // 本番では TraceIDRatioBased 等を検討
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)
	otel.SetTracerProvider(tp)

	// 4) W3C Trace Context を使う設定
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return tp, nil
}
