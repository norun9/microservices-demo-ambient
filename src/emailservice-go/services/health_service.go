// emailservice-go/services/health_service.go

package services

import (
	"context"
	"fmt"

	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// HealthCheckService は gRPC の Health チェックを実装します
type HealthCheckService struct {
	healthpb.UnimplementedHealthServer
}

// NewHealthCheckService コンストラクタ
func NewHealthCheckService() *HealthCheckService {
	return &HealthCheckService{}
}

// Check RPC: ヘルスチェックを実行します
func (h *HealthCheckService) Check(ctx context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	fmt.Println("HealthCheckService: Check called")
	return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
}

// Watch RPC は未実装（クライアントがストリーミングしない限り必要ありません）
func (h *HealthCheckService) Watch(req *healthpb.HealthCheckRequest, stream healthpb.Health_WatchServer) error {
	// Watch は省略。実装例では単に終了させるかエラーを返すだけ
	return nil
}
