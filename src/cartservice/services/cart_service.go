// cartservice-go/services/cart_service.go

package services

import (
	"context"

	pb "github.com/norun9/microservices-demo-ambient/genproto"
	"github.com/norun9/microservices-demo-ambient/src/cartservice/cartstore"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CartServiceServer implements the CartServiceServer interface.
type CartServiceServer struct {
	store  cartstore.ICartStore
	tracer trace.Tracer
	pb.UnimplementedCartServiceServer
}

// NewCartServiceServer creates a server instance with a store and tracer injected.
func NewCartServiceServer(store cartstore.ICartStore) *CartServiceServer {
	return &CartServiceServer{
		store:  store,
		tracer: otel.Tracer("cartservice"),
	}
}

// AddItem RPC implementation.
func (s *CartServiceServer) AddItem(ctx context.Context, req *pb.AddItemRequest) (*pb.Empty, error) {
	ctx, span := s.tracer.Start(ctx, "AddItem")
	defer span.End()
	span.SetAttributes(
		attribute.String("app.user_id", req.UserId),
		attribute.String("app.product_id", req.Item.ProductId),
		attribute.Int64("app.quantity", int64(req.Item.Quantity)),
	)

	if err := s.store.AddItem(ctx, req.UserId, req.Item.ProductId, req.Item.Quantity); err != nil {
		return nil, status.Errorf(codes.Internal, "AddItem failed: %v", err)
	}
	return &pb.Empty{}, nil
}

// GetCart RPC implementation.
func (s *CartServiceServer) GetCart(ctx context.Context, req *pb.GetCartRequest) (*pb.Cart, error) {
	ctx, span := s.tracer.Start(ctx, "GetCart")
	defer span.End()
	span.SetAttributes(attribute.String("app.user_id", req.UserId))

	cart, err := s.store.GetCart(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "GetCart failed: %v", err)
	}
	return cart, nil
}

// EmptyCart RPC implementation.
func (s *CartServiceServer) EmptyCart(ctx context.Context, req *pb.EmptyCartRequest) (*pb.Empty, error) {
	ctx, span := s.tracer.Start(ctx, "EmptyCart")
	defer span.End()
	span.SetAttributes(attribute.String("app.user_id", req.UserId))

	if err := s.store.EmptyCart(ctx, req.UserId); err != nil {
		return nil, status.Errorf(codes.Internal, "EmptyCart failed: %v", err)
	}
	return &pb.Empty{}, nil
}
