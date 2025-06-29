// recommendationservice-go/services/health_service.go

package services

import (
	"context"

	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// HealthCheckService は gRPC Health Check サービスを実装します
type HealthCheckService struct {
	healthpb.UnimplementedHealthServer
}

// NewHealthCheckService コンストラクタ
func NewHealthCheckService() *HealthCheckService {
	return &HealthCheckService{}
}

// Check はヘルスチェックを実行します
func (h *HealthCheckService) Check(ctx context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	return &healthpb.HealthCheckResponse{
		Status: healthpb.HealthCheckResponse_SERVING,
	}, nil
}

// Watch はヘルスチェックの監視を実装します（未実装）
func (h *HealthCheckService) Watch(req *healthpb.HealthCheckRequest, stream healthpb.Health_WatchServer) error {
	return stream.Send(&healthpb.HealthCheckResponse{
		Status: healthpb.HealthCheckResponse_NOT_SERVING,
	})
}
