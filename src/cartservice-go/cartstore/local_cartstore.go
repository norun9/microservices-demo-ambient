// cartservice-go/cartstore/local_cartstore.go

package cartstore

import (
	"context"
	"fmt"
	"sync"

	"github.com/norun9/microservices-demo-ambient/src/cartservice-go/genproto/hipstershop"
)

// LocalCartStore は、メモリ上にカートを保持する簡易版ストレージです。
// マルチスレッド環境を考慮して sync.RWMutex または sync.Map で実装します。
type LocalCartStore struct {
	mu    sync.RWMutex
	store map[string]*hipstershop.Cart

	emptyCart *hipstershop.Cart
}

// NewLocalCartStore コンストラクタ
func NewLocalCartStore() *LocalCartStore {
	return &LocalCartStore{
		store:     make(map[string]*hipstershop.Cart),
		emptyCart: &hipstershop.Cart{},
	}
}

// Initialize は初期化処理（ここでは何もしない）
func (l *LocalCartStore) Initialize(ctx context.Context) error {
	fmt.Println("LocalCartStore initialized")
	return nil
}

// AddItem はユーザーのカートに商品を追加します
func (l *LocalCartStore) AddItem(ctx context.Context, userID, productID string, quantity int32) error {
	fmt.Printf("LocalCartStore: AddItem called (userID=%s, productID=%s, quantity=%d)\n", userID, productID, quantity)
	l.mu.Lock()
	defer l.mu.Unlock()

	cart, exists := l.store[userID]
	if !exists {
		cart = &hipstershop.Cart{
			UserId: userID,
			Items:  []*hipstershop.CartItem{},
		}
		l.store[userID] = cart
	}

	// 既存アイテムを探す
	found := false
	for _, item := range cart.Items {
		if item.ProductId == productID {
			item.Quantity += quantity
			found = true
			break
		}
	}
	if !found {
		cart.Items = append(cart.Items, &hipstershop.CartItem{
			ProductId: productID,
			Quantity:  quantity,
		})
	}

	return nil
}

// EmptyCart はユーザーのカートを空にします
func (l *LocalCartStore) EmptyCart(ctx context.Context, userID string) error {
	fmt.Printf("LocalCartStore: EmptyCart called (userID=%s)\n", userID)
	l.mu.Lock()
	defer l.mu.Unlock()

	l.store[userID] = &hipstershop.Cart{UserId: userID}
	return nil
}

// GetCart はユーザーのカート情報を取得します
func (l *LocalCartStore) GetCart(ctx context.Context, userID string) (*hipstershop.Cart, error) {
	fmt.Printf("LocalCartStore: GetCart called (userID=%s)\n", userID)
	l.mu.RLock()
	defer l.mu.RUnlock()

	if cart, exists := l.store[userID]; exists {
		return cart, nil
	}
	// カートが存在しなければ空のカートを返す
	return l.emptyCart, nil
}

// Ping は疎通チェック用。常に true を返します
func (l *LocalCartStore) Ping(ctx context.Context) bool {
	return true
}
