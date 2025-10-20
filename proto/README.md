# TitanCompute Protocol Buffers

This directory contains the gRPC service definitions and protocol buffer generation tools for TitanCompute's inter-service communication.

## 🏗️ What's Inside

```
proto/
├── titancompute.proto    # Main gRPC service definitions
├── generate.sh          # Protocol buffer code generation script
└── (generated files will appear in respective service directories)
```

## 🧠 How It Works

### Protocol Buffer Architecture

```
titancompute.proto → protoc → Go stubs (coordinator) + Python stubs (agent)
```

### Service Definitions

The `titancompute.proto` file defines two main gRPC services:

1. **CoordinatorService** - Central orchestration
   - `RequestInference` - Client requests inference routing
   - `RegisterAgent` - Agent registration with coordinator
   - `ReportHealth` - Streaming health updates from agents
   - `QuerySystemStatus` - System status queries
   - `GetPublicKey` - JWT public key distribution

2. **AgentService** - Direct client streaming
   - `StreamInference` - Direct streaming inference (bypasses coordinator)
   - `GetStatus` - Agent status queries

### Message Types

#### Request/Response Messages
- `InferenceRequest` / `InferenceResponse` - Inference routing
- `AgentRegistration` / `RegistrationResponse` - Agent registration
- `StatusRequest` / `SystemStatus` - System status
- `StreamRequest` / `StreamResponse` - Direct agent streaming

#### Data Structures
- `AgentInfo` - Agent metadata and capabilities
- `HealthUpdate` / `HealthAck` - Health monitoring
- `SystemMetrics` - Resource utilization data

## 🔧 Setup

### Prerequisites

- **Protocol Buffers Compiler** (`protoc`)
- **Go protobuf plugins** (`protoc-gen-go`, `protoc-gen-go-grpc`)
- **Python grpcio-tools**

### Installation

```bash
# Install protoc (Ubuntu/Debian)
sudo apt install protobuf-compiler

# Install protoc (macOS)
brew install protobuf

# Install Go plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Install Python tools
pip install grpcio-tools
```

## 🚀 Development Usage

### Generate Protocol Buffers

```bash
# Generate all language bindings
./proto/generate.sh

# Or manually generate specific languages
cd proto

# Generate Go code
protoc --go_out=../coordinator/pkg/proto \
       --go-grpc_out=../coordinator/pkg/proto \
       --go_opt=paths=source_relative \
       --go-grpc_opt=paths=source_relative \
       titancompute.proto

# Generate Python code
python -m grpc_tools.protoc \
    --python_out=../agent/src/proto \
    --grpc_python_out=../agent/src/proto \
    --proto_path=. \
    titancompute.proto
```

### Verify Generation

```bash
# Check Go files
ls ../coordinator/pkg/proto/

# Check Python files  
ls ../agent/src/proto/

# Expected files:
# - titancompute.pb.go (messages)
# - titancompute_grpc.pb.go (services) 
# - titancompute_pb2.py (messages)
# - titancompute_pb2_grpc.py (services)
```

## 📝 Protocol Definition Details

### Service Patterns

1. **Request-Response** (Most endpoints)
   ```protobuf
   rpc RequestInference(InferenceRequest) returns (InferenceResponse);
   ```

2. **Bidirectional Streaming** (Health monitoring)
   ```protobuf
   rpc ReportHealth(stream HealthUpdate) returns (stream HealthAck);
   ```

3. **Server Streaming** (Inference results)
   ```protobuf
   rpc StreamInference(StreamRequest) returns (stream StreamResponse);
   ```

### Message Design Principles

- **Versioned APIs** - Forward and backward compatibility
- **Optional Fields** - Graceful degradation
- **Structured Metadata** - Rich context information
- **Streaming Support** - Efficient large data transfer

### Field Types and Validation

```protobuf
// Required fields (basic validation)
string client_id = 1;        // Non-empty client identifier
string model = 2;            // Model name/identifier  
string prompt = 3;           // Inference prompt

// Optional fields (with defaults)
int32 max_tokens = 4;        // Default: 100
float temperature = 5;       // Default: 0.7
map<string, string> parameters = 6; // Additional parameters

// Timestamps and metadata
int64 expires_at = 3;        // Unix timestamp
double estimated_rtt_ms = 5; // Network latency estimate
```

## 🔄 Development Workflow

### 1. Modify Protocol Definition

```bash
# Edit titancompute.proto
nano titancompute.proto

# Add new service method
rpc NewMethod(NewRequest) returns (NewResponse);

# Add new message types
message NewRequest {
  string field1 = 1;
  int32 field2 = 2;
}
```

### 2. Regenerate Code

```bash
# Regenerate all language bindings
./generate.sh

# Verify no compilation errors
cd ../coordinator && go build ./...
cd ../agent && python -c "import src.proto.titancompute_pb2"
```

### 3. Update Service Implementations

```bash
# Update Go service implementation
# coordinator/pkg/server/server.go

# Update Python service implementation  
# agent/src/agent_server.py
```

### 4. Test Changes

```bash
# Test gRPC endpoints
grpcurl -plaintext localhost:50051 list titancompute.v1.CoordinatorService
grpcurl -plaintext localhost:50052 list titancompute.v1.AgentService

# Test with new methods
grpcurl -plaintext -d '{"field1": "test"}' localhost:50051 titancompute.v1.CoordinatorService/NewMethod
```

## 🔍 Protocol Analysis Tools

### Inspect Services

```bash
# List available services
grpcurl -plaintext localhost:50051 list

# Describe service methods
grpcurl -plaintext localhost:50051 describe titancompute.v1.CoordinatorService

# Describe message types
grpcurl -plaintext localhost:50051 describe titancompute.v1.InferenceRequest
```

### Test gRPC Communication

```bash
# Health check
grpcurl -plaintext localhost:50051 grpc.health.v1.Health/Check

# System status
grpcurl -plaintext -d '{}' localhost:50051 titancompute.v1.CoordinatorService/QuerySystemStatus

# Inference request
grpcurl -plaintext -d '{
  "client_id": "test",
  "model": "llama3.1:8b-instruct-q4_k_m", 
  "prompt": "Hello",
  "max_tokens": 10
}' localhost:50051 titancompute.v1.CoordinatorService/RequestInference
```

## 🐛 Debugging

### Common Issues

#### "protoc command not found"
```bash
# Install protobuf compiler
sudo apt install protobuf-compiler  # Ubuntu/Debian
brew install protobuf               # macOS
```

#### "protoc-gen-go not found"
```bash
# Install Go plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Add to PATH
export PATH="$PATH:$(go env GOPATH)/bin"
```

#### "Python import errors"
```bash
# Install Python gRPC tools
pip install grpcio grpcio-tools

# Verify proto files exist
ls ../agent/src/proto/titancompute_pb2*.py
```

#### "Generated files outdated"
```bash
# Clean and regenerate
rm -rf ../coordinator/pkg/proto/*
rm -rf ../agent/src/proto/titancompute_pb2*.py
./generate.sh
```

## 📊 Performance Considerations

### Message Size Optimization

- Use appropriate field types (int32 vs int64)
- Avoid large string fields in high-frequency messages
- Use streaming for large responses
- Consider message compression for slow networks

### gRPC Best Practices

- Keep service methods focused and atomic
- Use appropriate streaming patterns
- Handle errors gracefully with status codes
- Implement proper timeout and retry logic

## 🔗 Integration

The protocol buffers integrate with:
- **Coordinator** (Go) - Server-side service implementation
- **Agent** (Python) - Server and client implementation  
- **Clients** (Any language) - Client-side stub generation
- **Tools** (grpcurl, etc.) - Testing and debugging

For language-specific integration details, see the respective service README files.
