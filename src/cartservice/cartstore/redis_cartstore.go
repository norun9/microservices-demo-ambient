// cartservice-go/cartstore/redis_cartstore.go

package cartstore

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/extra/redisotel/v8"
	"github.com/go-redis/redis/v8"
	pb "github.com/norun9/microservices-demo-ambient/genproto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// RedisCartStore is a cart store backed by Redis.
type RedisCartStore struct {
	client        *redis.Client
	ctx           context.Context
	emptyCartData []byte
}

// NewRedisCartStore accepts a Redis connection string (e.g., "hostname:port") and returns a store instance.
func NewRedisCartStore(ctx context.Context, redisAddr string) (*RedisCartStore, error) {
	// go-redis/v8 client settings
	opts, err := redis.ParseURL(redisAddr)
	if err != nil {
		// If not in "redis://..." format, use it as a simple Addr.
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
		}
	}

	client := redis.NewClient(opts)
	client.AddHook(redisotel.NewTracingHook())

	// Serialize an empty Cart.
	emptyCart := &pb.Cart{}
	emptyData, _ := proto.Marshal(emptyCart)

	store := &RedisCartStore{
		client:        client,
		ctx:           ctx,
		emptyCartData: emptyData,
	}
	return store, nil
}

// Initialize checks the Redis connection.
func (r *RedisCartStore) Initialize(ctx context.Context) error {
	log.Println("RedisCartStore: initializing connection...")

	// Retry logic
	for i := 0; i < 30; i++ {
		// Create a new context with a timeout.
		log.Printf("RedisCartStore: attempting Ping (attempt %d/30)...", i+1)
		if r.Ping(ctx) {
			log.Printf("RedisCartStore: Ping successful on attempt %d", i+1)
			log.Println("RedisCartStore initialized successfully")
			return nil
		}

		// Wait with exponential backoff.
		backoff := time.Duration(1000*(1<<uint(i))) * time.Millisecond
		if backoff > 30*time.Second {
			backoff = 30 * time.Second
		}
		log.Printf("RedisCartStore: waiting %v before next attempt", backoff)

		// Check if the context was canceled during backoff.
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

// AddItem stores cart information in a Redis Hash for each user.
func (r *RedisCartStore) AddItem(ctx context.Context, userID, productID string, quantity int32) error {
	log.Printf("RedisCartStore: AddItem called (userID=%s, productID=%s, quantity=%d)\n", userID, productID, quantity)

	// Use the user ID as the Hash key and store the Cart binary in the "cart" field.
	key := userID
	field := "cart"

	// Get the current value.
	val, err := r.client.HGet(r.ctx, key, field).Result()
	if err != nil && err != redis.Nil {
		return status.Errorf(codes.FailedPrecondition, "redis HGet error: %v", err)
	}

	var cart pb.Cart
	if err == redis.Nil {
		// Create a new cart if it doesn't exist.
		cart.UserId = userID
		cart.Items = []*pb.CartItem{
			{
				ProductId: productID,
				Quantity:  quantity,
			},
		}
	} else {
		// Restore existing data from proto.
		if parseErr := proto.Unmarshal([]byte(val), &cart); parseErr != nil {
			return status.Errorf(codes.FailedPrecondition, "failed to parse cart data: %v", parseErr)
		}
		// If the same productID exists, add to the quantity; otherwise, append.
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
	}

	// Serialize the modified cart to binary and save it to Redis.
	bin, _ := proto.Marshal(&cart)
	if err := r.client.HSet(r.ctx, key, field, bin).Err(); err != nil {
		return status.Errorf(codes.FailedPrecondition, "redis HSet error: %v", err)
	}
	return nil
}

// EmptyCart empties a user's cart.
func (r *RedisCartStore) EmptyCart(ctx context.Context, userID string) error {
	log.Printf("RedisCartStore: EmptyCart called (userID=%s)\n", userID)

	key := userID
	field := "cart"
	if err := r.client.HSet(r.ctx, key, field, r.emptyCartData).Err(); err != nil {
		return status.Errorf(codes.FailedPrecondition, "redis HSet error: %v", err)
	}
	return nil
}

// GetCart retrieves a cart from Redis, returning an empty one if it doesn't exist.
func (r *RedisCartStore) GetCart(ctx context.Context, userID string) (*pb.Cart, error) {
	log.Printf("RedisCartStore: GetCart called (userID=%s)\n", userID)

	key := userID
	field := "cart"
	val, err := r.client.HGet(r.ctx, key, field).Result()
	if err != nil && err != redis.Nil {
		return nil, status.Errorf(codes.FailedPrecondition, "redis HGet error: %v", err)
	}
	if err == redis.Nil {
		// Return an empty cart if it doesn't exist.
		return &pb.Cart{}, nil
	}
	var cart pb.Cart
	if parseErr := proto.Unmarshal([]byte(val), &cart); parseErr != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "failed to parse cart data: %v", parseErr)
	}
	return &cart, nil
}

// Ping checks if Redis is alive.
func (r *RedisCartStore) Ping(ctx context.Context) bool {
	log.Println("RedisCartStore: executing Ping...")

	// Create a context with a timeout.
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
