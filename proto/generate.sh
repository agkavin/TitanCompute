#!/bin/bash
# Generate protocol buffer code for Go and Python

set -e

PROTO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$PROTO_DIR")"

echo "🔧 Generating protocol buffer code..."

# Create output directories
mkdir -p "$PROJECT_ROOT/coordinator/pkg/proto"
mkdir -p "$PROJECT_ROOT/agent/src/proto"

# Generate Go code
echo "📦 Generating Go stubs..."
protoc \
  --go_out="$PROJECT_ROOT/coordinator/pkg/proto" \
  --go-grpc_out="$PROJECT_ROOT/coordinator/pkg/proto" \
  --proto_path="$PROTO_DIR" \
  "$PROTO_DIR/titancompute.proto"

# Generate Python code
echo "🐍 Generating Python stubs..."
if python -c "import grpc_tools.protoc" 2>/dev/null; then
  python -m grpc_tools.protoc \
    --python_out="$PROJECT_ROOT/agent/src/proto" \
    --grpc_python_out="$PROJECT_ROOT/agent/src/proto" \
    --proto_path="$PROTO_DIR" \
    "$PROTO_DIR/titancompute.proto"

  # Fix Python imports (make them relative)
  if [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS
    sed -i '' 's/^import titancompute_pb2/from . import titancompute_pb2/' \
      "$PROJECT_ROOT/agent/src/proto/titancompute_pb2_grpc.py"
  else
    # Linux
    sed -i 's/^import titancompute_pb2/from . import titancompute_pb2/' \
      "$PROJECT_ROOT/agent/src/proto/titancompute_pb2_grpc.py"
  fi
else
  echo "⚠️ grpc_tools not found. Skipping Python code generation."
  echo "   Install with: pip install grpcio-tools"
fi

echo "✅ Protocol buffer generation complete!"
echo "   Go files: coordinator/pkg/proto/"
echo "   Python files: agent/src/proto/"
