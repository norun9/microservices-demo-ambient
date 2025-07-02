# adservice-go/Dockerfile

# --- ビルドステージ ---
FROM golang:1.24-alpine AS builder

# 必要なパッケージ（git/protoc など）があれば追加
RUN apk add --no-cache git protoc

# Go モジュールキャッシュを活用するため、先に go.mod と go.sum をコピー
WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download

# プロジェクト全体をコピー
COPY . .

# サーバーをビルド
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/adservice-go ./server

# --- 実行ステージ ---
FROM alpine:3.18

# 実行に必要な証明書などをインストール
RUN apk add --no-cache ca-certificates

# ビルドステージからバイナリをコピー
COPY --from=builder /bin/adservice-go /usr/local/bin/adservice-go

# デフォルトのポート
EXPOSE 9555

# OpenTelemetry設定
ENV OTEL_EXPORTER_OTLP_ENDPOINT=dsn:///otel-collector.observability.svc.cluster.local:4317
ENV OTEL_SERVICE_NAME=adservice
ENV OTEL_SERVICE_VERSION=v1.0.0

# grpc_health_probe をダウンロードして配置
ENV GRPC_HEALTH_PROBE_VERSION=v0.4.20
RUN apk add --no-cache curl \
    && curl -sSL \
    https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64 \
    -o /bin/grpc_health_probe \
    && chmod +x /bin/grpc_health_probe

# エントリポイント
ENTRYPOINT ["/usr/local/bin/adservice-go"]
