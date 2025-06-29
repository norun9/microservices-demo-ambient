#!/bin/bash

# protobuf ファイルから Go コードを生成するスクリプト

set -e

# 必要なツールをインストール
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# genproto ディレクトリを作成
mkdir -p genproto/hipstershop

cd proto
protoc \
  --proto_path=. \
  --go_out=../genproto/hipstershop       --go_opt=paths=source_relative \
  --go-grpc_out=../genproto/hipstershop  --go-grpc_opt=paths=source_relative \
  demo.proto

echo "Generated Go code from protobuf files" 