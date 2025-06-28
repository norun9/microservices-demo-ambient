#!/bin/bash -eu
#
# Copyright 2018 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
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