// cartservice-go/cartstore/redis_cartstore.go

package cartstore

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/norun9/microservices-demo-ambient/src/cartservice-go/genproto/hipstershop"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// RedisCartStore は Redis をバックエンドに使うカートストアです
type RedisCartStore struct {
	client        *redis.Client
	ctx           context.Context
	emptyCartData []byte
}

// NewRedisCartStore は Redis の接続文字列（"hostname:port" 等）を受け取り、ストアインスタンスを返します
func NewRedisCartStore(ctx context.Context, redisAddr string) (*RedisCartStore, error) {
	// go-redis/v8 のクライアント設定
	opts, err := redis.ParseURL(redisAddr)
	if err != nil {
		// もし "redis://..." 形式でない場合は単純に Addr として使う
		opts = &redis.Options{
			Addr:         redisAddr,
			MinIdleConns: 1,
			MaxRetries:   30,
			DialTimeout:  30 * time.Second,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			PoolSize:     10,
			PoolTimeout:  4 * time.Second,
			IdleTimeout:  180 * time.Second,
			// OnConnect: func(ctx context.Context, cn *redis.Conn) error {
			// 	log.Println("Redis connection established")
			// 	return nil
			// },
		}
	}

	client := redis.NewClient(opts)
	// OpenTelemetry 用の Hook を追加

	// 空の Cart をシリアライズしておく
	emptyCart := &hipstershop.Cart{}
	emptyData, _ := proto.Marshal(emptyCart)

	store := &RedisCartStore{
		client:        client,
		ctx:           ctx,
		emptyCartData: emptyData,
	}
	return store, nil
}

// Initialize は Redis 接続確認などを行います
func (r *RedisCartStore) Initialize(ctx context.Context) error {
	log.Println("RedisCartStore: initializing connection...")

	// リトライロジック
	for i := 0; i < 30; i++ {
		// 新しいコンテキストを作成（タイムアウト付き）
		log.Printf("RedisCartStore: attempting Ping (attempt %d/30)...", i+1)
		if r.Ping(ctx) {
			log.Printf("RedisCartStore: Ping successful on attempt %d", i+1)
			log.Println("RedisCartStore initialized successfully")
			return nil
		}

		// 指数バックオフで待機
		backoff := time.Duration(1000*(1<<uint(i))) * time.Millisecond
		if backoff > 30*time.Second {
			backoff = 30 * time.Second
		}
		log.Printf("RedisCartStore: waiting %v before next attempt", backoff)

		// バックオフ中にコンテキストがキャンセルされたかチェック
		select {
		case <-ctx.Done():
			log.Printf("RedisCartStore: context cancelled during backoff: %v", ctx.Err())
			return ctx.Err()
		case <-time.After(backoff):
			continue
		}
	}

	log.Printf("RedisCartStore: failed to connect after 30 attempts")
	return fmt.Errorf("failed to connect to Redis after 30 attempts")
}

// AddItem は Redis の Hash にユーザーごとにカート情報を保持します
func (r *RedisCartStore) AddItem(ctx context.Context, userID, productID string, quantity int32) error {
	log.Printf("RedisCartStore: AddItem called (userID=%s, productID=%s, quantity=%d)\n", userID, productID, quantity)

	// Hash キーをユーザー ID にして、フィールド名 "cart" に Cart のバイナリを保存
	key := userID
	field := "cart"

	// 現在の値を取得
	val, err := r.client.HGet(r.ctx, key, field).Result()
	if err != nil && err != redis.Nil {
		return status.Errorf(codes.FailedPrecondition, "redis HGet error: %v", err)
	}

	var cart hipstershop.Cart
	if err == redis.Nil {
		// カートが存在しなければ新規作成
		cart.UserId = userID
		cart.Items = []*hipstershop.CartItem{
			{
				ProductId: productID,
				Quantity:  quantity,
			},
		}
	} else {
		// 既存データを proto から復元
		if parseErr := proto.Unmarshal([]byte(val), &cart); parseErr != nil {
			return status.Errorf(codes.FailedPrecondition, "failed to parse cart data: %v", parseErr)
		}
		// 同じ productID があれば数量を加算、なければ追加
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
	}

	// 変更後の cart をバイナリにシリアライズして Redis に保存
	bin, _ := proto.Marshal(&cart)
	if err := r.client.HSet(r.ctx, key, field, bin).Err(); err != nil {
		return status.Errorf(codes.FailedPrecondition, "redis HSet error: %v", err)
	}
	return nil
}

// EmptyCart はユーザーのカートを空にします
func (r *RedisCartStore) EmptyCart(ctx context.Context, userID string) error {
	log.Printf("RedisCartStore: EmptyCart called (userID=%s)\n", userID)

	key := userID
	field := "cart"
	if err := r.client.HSet(r.ctx, key, field, r.emptyCartData).Err(); err != nil {
		return status.Errorf(codes.FailedPrecondition, "redis HSet error: %v", err)
	}
	return nil
}

// GetCart は Redis からカートを取得し、存在しなければ空を返します
func (r *RedisCartStore) GetCart(ctx context.Context, userID string) (*hipstershop.Cart, error) {
	log.Printf("RedisCartStore: GetCart called (userID=%s)\n", userID)

	key := userID
	field := "cart"
	val, err := r.client.HGet(r.ctx, key, field).Result()
	if err != nil && err != redis.Nil {
		return nil, status.Errorf(codes.FailedPrecondition, "redis HGet error: %v", err)
	}
	if err == redis.Nil {
		// カートが存在しない場合は空を返す
		return &hipstershop.Cart{}, nil
	}
	var cart hipstershop.Cart
	if parseErr := proto.Unmarshal([]byte(val), &cart); parseErr != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "failed to parse cart data: %v", parseErr)
	}
	return &cart, nil
}

// Ping は Redis が生きているかを確認します
func (r *RedisCartStore) Ping(ctx context.Context) bool {
	log.Println("RedisCartStore: executing Ping...")

	// タイムアウト付きのコンテキストを作成
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	log.Println("RedisCartStore: created timeout context for Ping")

	result, err := r.client.Ping(pingCtx).Result()
	if err != nil {
		log.Printf("RedisCartStore: Ping failed with error: %v", err)
		return false
	}

	log.Printf("RedisCartStore: Ping succeeded with result: %s", result)
	return true
}
