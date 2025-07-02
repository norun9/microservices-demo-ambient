// currencyservice-go/services/health_service.go

package services

import (
	"context"
	"fmt"

	healthpb "github.com/norun9/microservices-demo-ambient/genproto/hipstershop"
)

// HealthCheckService は gRPC の Health チェックを実装します
type HealthCheckService struct {
	healthpb.UnimplementedHealthServer
}

// NewHealthCheckService コンストラクタ
func NewHealthCheckService() *HealthCheckService {
	return &HealthCheckService{}
}

func (h *HealthCheckService) Check(ctx context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	fmt.Println("HealthCheckService: Check called")
	return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
}
