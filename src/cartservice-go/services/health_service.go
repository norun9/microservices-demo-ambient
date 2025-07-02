// cartservice-go/services/health_service.go

package services

import (
	"context"
	"fmt"

	"github.com/norun9/microservices-demo-ambient/src/cartservice-go/cartstore"
	healthpb "github.com/norun9/microservices-demo-ambient/src/cartservice-go/genproto/hipstershop/grpc/health/v1"
)

// HealthCheckService は gRPC の Health チェックを実装します
type HealthCheckService struct {
	store cartstore.ICartStore
	healthpb.UnimplementedHealthServer
}

// NewHealthCheckService コンストラクタ
func NewHealthCheckService(store cartstore.ICartStore) *HealthCheckService {
	return &HealthCheckService{store: store}
}

// Check RPC: ICartStore.Ping を呼び、正常 / 異常を返します
func (h *HealthCheckService) Check(ctx context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	fmt.Println("HealthCheckService: Check called")
	if h.store.Ping(ctx) {
		return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
	}
	return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_NOT_SERVING}, nil
}
