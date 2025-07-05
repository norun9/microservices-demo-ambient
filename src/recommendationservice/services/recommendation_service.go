// recommendationservice-go/services/recommendation_service.go

package services

import (
	"context"
	"log"
	"math/rand"
	"os"

	pb "github.com/norun9/microservices-demo-ambient/genproto"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// RecommendationService implements the gRPC RecommendationService.
type RecommendationService struct {
	pb.UnimplementedRecommendationServiceServer
	productCatalogClient pb.ProductCatalogServiceClient
	tracer               trace.Tracer
}

// NewRecommendationService constructor.
func NewRecommendationService() (*RecommendationService, error) {
	// Establish a connection to the ProductCatalogService.
	catalogAddr := os.Getenv("PRODUCT_CATALOG_SERVICE_ADDR")
	if catalogAddr == "" {
		log.Fatal("PRODUCT_CATALOG_SERVICE_ADDR environment variable not set")
	}

	conn, err := grpc.NewClient(catalogAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)

	if err != nil {
		return nil, err
	}

	productCatalogClient := pb.NewProductCatalogServiceClient(conn)

	return &RecommendationService{
		productCatalogClient: productCatalogClient,
		tracer:               otel.Tracer("recommendationservice"),
	}, nil
}

// ListRecommendations RPC: returns a list of product recommendations.
func (r *RecommendationService) ListRecommendations(ctx context.Context, req *pb.ListRecommendationsRequest) (*pb.ListRecommendationsResponse, error) {
	_, span := r.tracer.Start(ctx, "ListRecommendations")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", req.UserId),
		attribute.StringSlice("request.product_ids", req.ProductIds),
	)

	log.Printf("Received ListRecommendations request for user: %s", req.UserId)

	// Get the product list from the ProductCatalogService.
	catResponse, err := r.productCatalogClient.ListProducts(ctx, &pb.Empty{})
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		log.Printf("Failed to get products from catalog: %v", err)
		return &pb.ListRecommendationsResponse{}, nil // Ignore the error and continue.
	}

	// Create a list of product IDs.
	var productIDs []string
	for _, product := range catResponse.Products {
		productIDs = append(productIDs, product.Id)
	}

	// Exclude the requested product IDs.
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

	// Randomly select up to 5 products.
	maxResponses := 5
	numProducts := len(filteredProducts)
	numReturn := min(maxResponses, numProducts)

	var recommendations []string
	if numReturn > 0 {
		// Randomly select indices.
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

	return &pb.ListRecommendationsResponse{
		ProductIds: recommendations,
	}, nil
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
