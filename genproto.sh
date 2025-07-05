#!/bin/bash
set -euo pipefail

go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Create output directory if it doesn't exist
mkdir -p genproto

# Generate Go code from proto files
protoc -I=pb \
  --go_out=genproto --go_opt=paths=source_relative \
  --go-grpc_out=genproto --go-grpc_opt=paths=source_relative \
  pb/demo.proto pb/health.proto

echo "Protobuf Go files generated in ./genproto/hipstershop" 