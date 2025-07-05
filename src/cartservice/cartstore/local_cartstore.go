package cartstore

import (
	"context"
	"fmt"
	"sync"

	pb "github.com/norun9/microservices-demo-ambient/genproto"
)

// LocalCartStore is a simple in-memory cart storage.
// It is implemented with sync.RWMutex or sync.Map to handle multi-threaded environments.
type LocalCartStore struct {
	mu    sync.RWMutex
	store map[string]*pb.Cart

	emptyCart *pb.Cart
}

// NewLocalCartStore constructor
func NewLocalCartStore() *LocalCartStore {
	return &LocalCartStore{
		store:     make(map[string]*pb.Cart),
		emptyCart: &pb.Cart{},
	}
}

// Initialize does nothing in this implementation.
func (l *LocalCartStore) Initialize(ctx context.Context) error {
	fmt.Println("LocalCartStore initialized")
	return nil
}

// AddItem adds a product to the user's cart.
func (l *LocalCartStore) AddItem(ctx context.Context, userID, productID string, quantity int32) error {
	fmt.Printf("LocalCartStore: AddItem called (userID=%s, productID=%s, quantity=%d)\n", userID, productID, quantity)
	l.mu.Lock()
	defer l.mu.Unlock()

	cart, exists := l.store[userID]
	if !exists {
		cart = &pb.Cart{
			UserId: userID,
			Items:  []*pb.CartItem{},
		}
		l.store[userID] = cart
	}

	// Look for an existing item.
	found := false
	for _, item := range cart.Items {
		if item.ProductId == productID {
			item.Quantity += quantity
			found = true
			break
		}
	}
	if !found {
		cart.Items = append(cart.Items, &pb.CartItem{
			ProductId: productID,
			Quantity:  quantity,
		})
	}

	return nil
}

// EmptyCart empties a user's cart.
func (l *LocalCartStore) EmptyCart(ctx context.Context, userID string) error {
	fmt.Printf("LocalCartStore: EmptyCart called (userID=%s)\n", userID)
	l.mu.Lock()
	defer l.mu.Unlock()

	l.store[userID] = &pb.Cart{UserId: userID}
	return nil
}

// GetCart retrieves a user's cart.
func (l *LocalCartStore) GetCart(ctx context.Context, userID string) (*pb.Cart, error) {
	fmt.Printf("LocalCartStore: GetCart called (userID=%s)\n", userID)
	l.mu.RLock()
	defer l.mu.RUnlock()

	if cart, exists := l.store[userID]; exists {
		return cart, nil
	}
	// Return an empty cart if it doesn't exist.
	return l.emptyCart, nil
}

// Ping is a health check that always returns true.
func (l *LocalCartStore) Ping(ctx context.Context) bool {
	return true
}

