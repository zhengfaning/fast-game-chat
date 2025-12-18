#!/bin/bash

# Protobuf 代码生成脚本
# 用于重新生成所有 .proto 文件的 Go 代码

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROTO_DIR="${SCRIPT_DIR}"
PROTOC="${SCRIPT_DIR}/../.local/bin/protoc"

# 检查 protoc 是否存在
if [ ! -f "$PROTOC" ]; then
    echo "❌ 错误: protoc 不存在于 $PROTOC"
    echo "请先安装 protoc"
    exit 1
fi

echo "🚀 开始生成 Protobuf 代码..."
echo ""

# 生成 common 包
echo "📦 生成 common/message_base.proto ..."
cd "$PROTO_DIR"
"$PROTOC" --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    common/message_base.proto

# 生成 chat 包
echo "📦 生成 chat/chat_message.proto ..."
"$PROTOC" --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    -I. \
    chat/chat_message.proto

echo "📦 生成 chat/chat_service.proto ..."
"$PROTOC" --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    -I. \
    chat/chat_service.proto

echo ""
echo "✅ Protobuf 代码生成完成！"
echo ""
echo "生成的文件:"
find . -name "*.pb.go" -type f | sort
