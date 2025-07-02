// cartservice-go/cartstore/cartstore.go

package cartstore

import (
	"context"

	"github.com/norun9/microservices-demo-ambient/genproto/hipstershop"
)

// ICartStore はカートストレージへの操作を定義するインターフェースです
type ICartStore interface {
	Initialize(ctx context.Context) error

	AddItem(ctx context.Context, userID, productID string, quantity int32) error
	EmptyCart(ctx context.Context, userID string) error
	GetCart(ctx context.Context, userID string) (*hipstershop.Cart, error)

	Ping(ctx context.Context) bool
}
