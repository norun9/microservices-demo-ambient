package cartstore

import (
	"context"

	pb "github.com/norun9/microservices-demo-ambient/genproto"
)

// ICartStore is an interface for cart storage operations.
type ICartStore interface {
	Initialize(ctx context.Context) error

	AddItem(ctx context.Context, userID, productID string, quantity int32) error
	EmptyCart(ctx context.Context, userID string) error
	GetCart(ctx context.Context, userID string) (*pb.Cart, error)

	Ping(ctx context.Context) bool
}
