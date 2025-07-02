// adservice-go/server/main.go

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
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"google.golang.org/grpc"
)

// adServiceServer は自動生成された AdServiceServer インターフェースを実装します
type adServiceServer struct {
	pb.UnimplementedAdServiceServer
}

// adsMap: カテゴリごとに広告を保持する
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

// GetAds implements AdService.GetAds
func (s *adServiceServer) GetAds(ctx context.Context, req *pb.AdRequest) (*pb.AdResponse, error) {
	log.Printf("received GetAds request: context_keys=%v", req.ContextKeys)

	var allAds []*pb.Ad
	for _, key := range req.ContextKeys {
		if ads, ok := adsMap[key]; ok {
			allAds = append(allAds, ads...)
		}
	}
	if len(allAds) == 0 {
		allAds = getRandomAds()
	}

	resp := &pb.AdResponse{
		Ads: allAds,
	}
	return resp, nil
}

// getRandomAds: ランダムに広告を2件選んで返す
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

type HealthService struct {
	pb.UnimplementedHealthServer
}

func (h *HealthService) Check(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	return &pb.HealthCheckResponse{
		Status: pb.HealthCheckResponse_SERVING,
	}, nil
}

func main() {
	// 1) OpenTelemetry の初期化
	tp, err := initTracerProvider()
	if err != nil {
		log.Fatalf("failed to initialize tracer provider: %v", err)
	}
	// プログラム終了時に確実にリソースをフラッシュする
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}()

	// 2) gRPC サーバの生成
	port := os.Getenv("PORT")
	if port == "" {
		port = "9555"
	}
	addr := fmt.Sprintf(":%s", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", addr, err)
	}

	// gRPC サーバーに OpenTelemetry の StatsHandler を追加
	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)

	// 3) AdService サーバーを登録
	pb.RegisterAdServiceServer(grpcServer, &adServiceServer{})

	// 4) health チェックサービスを登録
	healthSvc := &HealthService{}
	pb.RegisterHealthServer(grpcServer, healthSvc)

	log.Printf("AdService gRPC server started, listening on %s", addr)
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("failed to serve gRPC server: %v", err)
	}
}

// initTracerProvider は OpenTelemetry の TracerProvider を初期化し、OTLP エクスポーターを設定します。
// 環境変数 OTEL_EXPORTER_OTLP_ENDPOINT で Collector のエンドポイントを指定。
// 例: OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector.observability.svc.cluster.local:4317
func initTracerProvider() (*sdktrace.TracerProvider, error) {
	ctx := context.Background()

	// 1) OTLP gRPC エクスポーターを作成
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:4317" // デフォルト値（必要に応じて変更）
	}
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP trace exporter: %w", err)
	}

	// 2) リソース（サービス名やバージョンなど）を定義
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String("adservice"), // Service 名を明示
			semconv.ServiceVersionKey.String("v1.0.0"), // バージョンなど
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// 3) TracerProvider のセットアップ
	bsp := sdktrace.NewBatchSpanProcessor(exporter)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()), // すべてサンプリングする（検証用）
		sdktrace.WithResource(res),                    // リソース属性を設定
		sdktrace.WithSpanProcessor(bsp),               // バッチエクスポート
	)
	otel.SetTracerProvider(tp)

	// 4) コンテキスト伝搬の設定（W3C Trace Context）
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return tp, nil
}
