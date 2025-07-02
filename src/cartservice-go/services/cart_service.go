// cartservice-go/services/cart_service.go

package services

import (
	"context"

	"github.com/norun9/microservices-demo-ambient/genproto/hipstershop"
	"github.com/norun9/microservices-demo-ambient/src/cartservice-go/cartstore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CartServiceServer は自動生成された CartServiceServer インターフェースを実装します
type CartServiceServer struct {
	store cartstore.ICartStore
	hipstershop.UnimplementedCartServiceServer
}

// NewCartServiceServer はストアを注入してサーバーインスタンスを生成します
func NewCartServiceServer(store cartstore.ICartStore) *CartServiceServer {
	return &CartServiceServer{
		store: store,
	}
}

// AddItem RPC の実装
func (s *CartServiceServer) AddItem(ctx context.Context, req *hipstershop.AddItemRequest) (*hipstershop.Empty, error) {
	if err := s.store.AddItem(ctx, req.UserId, req.Item.ProductId, req.Item.Quantity); err != nil {
		return nil, status.Errorf(codes.Internal, "AddItem failed: %v", err)
	}
	return &hipstershop.Empty{}, nil
}

// GetCart RPC の実装
func (s *CartServiceServer) GetCart(ctx context.Context, req *hipstershop.GetCartRequest) (*hipstershop.Cart, error) {
	cart, err := s.store.GetCart(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "GetCart failed: %v", err)
	}
	return cart, nil
}

// EmptyCart RPC の実装
func (s *CartServiceServer) EmptyCart(ctx context.Context, req *hipstershop.EmptyCartRequest) (*hipstershop.Empty, error) {
	if err := s.store.EmptyCart(ctx, req.UserId); err != nil {
		return nil, status.Errorf(codes.Internal, "EmptyCart failed: %v", err)
	}
	return &hipstershop.Empty{}, nil
}
