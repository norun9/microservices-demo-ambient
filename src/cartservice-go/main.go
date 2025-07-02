// cartservice-go/main.go

package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/norun9/microservices-demo-ambient/genproto/hipstershop"
	"github.com/norun9/microservices-demo-ambient/src/cartservice-go/cartstore"
	"github.com/norun9/microservices-demo-ambient/src/cartservice-go/services"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"

	healthpb "github.com/norun9/microservices-demo-ambient/genproto/hipstershop/grpc/health/v1"
	"google.golang.org/grpc"
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
	// 2) ICartStore の生成 (Environment 変数 REDIS_ADDR を参照)
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		log.Fatal("REDIS_ADDR environment variable is required")
	}
	log.Printf("Using RedisCartStore with address %s\n", redisAddr)

	// ポート番号が指定されていない場合のみ追加
	if !strings.Contains(redisAddr, ":") {
		redisAddr = redisAddr + ":6379"
	}

	redisStore, err := cartstore.NewRedisCartStore(ctx, redisAddr)
	if err != nil {
		log.Fatalf("failed to create RedisCartStore: %v", err)
	}
	if err := redisStore.Initialize(ctx); err != nil {
		log.Fatalf("failed to initialize RedisCartStore: %v", err)
	}
	store := redisStore
	log.Println("RedisCartStore initialized successfully")
	// ----------------------------------------------------------------

	// ----------------------------------------------------------------
	// 3) gRPC サーバーの起動
	port := os.Getenv("PORT")
	if port == "" {
		port = "7070"
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

	// CartService と HealthCheckService を登録
	cartSvc := services.NewCartServiceServer(store)
	hipstershop.RegisterCartServiceServer(grpcServer, cartSvc)
	log.Println("Registered CartService")

	healthSrv := services.NewHealthCheckService(store)
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
	log.Printf("CartService gRPC server is listening on %s\n", addr)
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
		endpoint = "localhost:4317"
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
