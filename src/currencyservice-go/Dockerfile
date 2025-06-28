# ビルドステージ
FROM golang:1.24-alpine AS builder

# 必要なツールをインストール
RUN apk add --no-cache protobuf-dev make git

# 作業ディレクトリを設定
WORKDIR /app

# 依存関係ファイルをコピー
COPY go.mod go.sum ./
RUN go mod download

# ソースコードをコピー
COPY . .

# protoc-gen-go と protoc-gen-go-grpc をインストール
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest \
 && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# protobuf ファイルから Go コードを生成
RUN protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    proto/demo.proto

# アプリケーションをビルド
RUN CGO_ENABLED=0 GOOS=linux go build -o currencyservice .

# 実行ステージ
FROM alpine:latest

# ヘルスチェックツールとダウンロードツールをインストール
RUN apk add --no-cache curl bash

# grpc_health_probe をダウンロードして配置
ENV GRPC_HEALTH_PROBE_VERSION v0.4.20
RUN curl -sSL \
    https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64 \
    -o /bin/grpc_health_probe \
  && chmod +x /bin/grpc_health_probe

# 作業ディレクトリを設定
WORKDIR /app

# ビルドステージからバイナリとデータファイルをコピー
COPY --from=builder /app/currencyservice .
COPY --from=builder /app/data ./data

# ポートを公開
EXPOSE 7000

# アプリケーションを実行
ENTRYPOINT ["/app/currencyservice"] 