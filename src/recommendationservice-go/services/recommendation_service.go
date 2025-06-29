// recommendationservice-go/services/recommendation_service.go

package services

import (
	"context"
	"log"
	"math/rand"
	"os"

	"github.com/norun9/microservices-demo-ambient/src/recommendationservice-go/genproto/hipstershop"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// RecommendationService は gRPC の RecommendationService を実装します
type RecommendationService struct {
	hipstershop.UnimplementedRecommendationServiceServer
	productCatalogClient hipstershop.ProductCatalogServiceClient
	tracer               trace.Tracer
}

// NewRecommendationService コンストラクタ
func NewRecommendationService() (*RecommendationService, error) {
	// ProductCatalogService への接続を確立
	catalogAddr := os.Getenv("PRODUCT_CATALOG_SERVICE_ADDR")
	if catalogAddr == "" {
		log.Fatal("PRODUCT_CATALOG_SERVICE_ADDR environment variable not set")
	}

	conn, err := grpc.Dial(catalogAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	productCatalogClient := hipstershop.NewProductCatalogServiceClient(conn)

	return &RecommendationService{
		productCatalogClient: productCatalogClient,
		tracer:               otel.Tracer("recommendationservice"),
	}, nil
}

// ListRecommendations RPC: 商品の推薦リストを返します
func (r *RecommendationService) ListRecommendations(ctx context.Context, req *hipstershop.ListRecommendationsRequest) (*hipstershop.ListRecommendationsResponse, error) {
	_, span := r.tracer.Start(ctx, "ListRecommendations")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", req.UserId),
		attribute.StringSlice("request.product_ids", req.ProductIds),
	)

	log.Printf("Received ListRecommendations request for user: %s", req.UserId)

	// ProductCatalogService から商品リストを取得
	catResponse, err := r.productCatalogClient.ListProducts(ctx, &hipstershop.Empty{})
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		log.Printf("Failed to get products from catalog: %v", err)
		return &hipstershop.ListRecommendationsResponse{}, nil // エラーを無視して続行
	}

	// 商品IDのリストを作成
	var productIDs []string
	for _, product := range catResponse.Products {
		productIDs = append(productIDs, product.Id)
	}

	// リクエストされた商品IDを除外
	filteredProducts := make([]string, 0)
	requestedSet := make(map[string]bool)
	for _, id := range req.ProductIds {
		requestedSet[id] = true
	}

	for _, id := range productIDs {
		if !requestedSet[id] {
			filteredProducts = append(filteredProducts, id)
		}
	}

	// 最大5個の商品をランダムに選択
	maxResponses := 5
	numProducts := len(filteredProducts)
	numReturn := min(maxResponses, numProducts)

	var recommendations []string
	if numReturn > 0 {
		// ランダムにインデックスを選択
		indices := rand.Perm(numProducts)[:numReturn]
		for _, idx := range indices {
			recommendations = append(recommendations, filteredProducts[idx])
		}
	}

	span.SetAttributes(
		attribute.Int("recommendations.count", len(recommendations)),
		attribute.StringSlice("recommendations.product_ids", recommendations),
	)

	log.Printf("Returning %d recommendations for user %s", len(recommendations), req.UserId)

	return &hipstershop.ListRecommendationsResponse{
		ProductIds: recommendations,
	}, nil
}

// min は2つの整数の最小値を返します
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
