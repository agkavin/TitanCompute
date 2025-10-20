# TitanCompute Coordinator

The TitanCompute Coordinator is a Go-based service that manages agent registration, intelligent request routing with MCDA scheduling, JWT authentication, and serves both gRPC and REST APIs.

## 🏗️ What's Inside

```
coordinator/
├── cmd/
│   └── main.go           # Entry point - starts coordinator service
├── pkg/
│   ├── config/
│   │   └── config.go     # Configuration management
│   ├── registry/
│   │   └── registry.go   # Agent registry with circuit breaker
│   ├── scheduler/
│   │   └── scheduler.go  # MCDA scheduling algorithm
│   ├── server/
│   │   ├── server.go     # gRPC server implementation
│   │   └── rest_handlers.go # REST API handlers
│   └── proto/            # Generated protocol buffer files
│       └── github.com/titancompute/proto/gen/go/
├── go.mod                # Go module definition
├── go.sum                # Dependency checksums
└── Dockerfile            # Container build configuration
```

## 🧠 How It Works

### Coordinator Architecture

```
Client → REST API (HTTP) → JWT + Agent Selection → Client
         ↓
      gRPC API → MCDA Scheduler → Agent Registry → JWT Generation
         ↓
    Agent Registration → Health Monitoring → Circuit Breaker
```

### Key Components

1. **Main Server** (`cmd/main.go`)
   - Dual server setup: gRPC (50051) + HTTP (8080)
   - Graceful shutdown handling
   - Component initialization and coordination

2. **Agent Registry** (`registry/registry.go`)
   - Agent registration and health tracking
   - Circuit breaker fault tolerance (Healthy → Degraded → Half-Open → Offline)
   - Real-time resource monitoring (VRAM, RAM, CPU)
   - Automatic agent cleanup and recovery

3. **MCDA Scheduler** (`scheduler/scheduler.go`)
   - Memory-Aware Multi-Criteria Decision Analysis
   - Weighted scoring: 40% VRAM + 30% Jobs + 20% RTT + 10% Performance
   - Intelligent model compatibility checking
   - Performance history tracking

4. **gRPC Server** (`server/server.go`)
   - Internal agent communication
   - JWT token generation with RSA-256
   - Agent registration and health reporting
   - System status queries

5. **REST API** (`server/rest_handlers.go`)
   - External client HTTP endpoints
   - JSON request/response handling
   - CORS support for web applications
   - Error handling and validation

## 🔧 Setup

### Prerequisites

- **Go 1.21+**
- **Protocol buffers compiler** (`protoc`)
- **Docker** (for containerized deployment)

### Development Setup

```bash
# 1. Install Go dependencies
cd coordinator
go mod tidy

# 2. Install protobuf tools
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# 3. Generate protocol buffers (from project root)
./proto/generate.sh

# 4. Set environment variables (optional)
export COORDINATOR_PORT=50051
export COORDINATOR_HTTP_PORT=8080
export HEARTBEAT_TIMEOUT=30s
export TOKEN_TTL=300s

# 5. Build and run
go build -o coordinator cmd/main.go
./coordinator
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `COORDINATOR_PORT` | `50051` | gRPC server port |
| `COORDINATOR_HTTP_PORT` | `8080` | REST API server port |
| `HEARTBEAT_TIMEOUT` | `30s` | Agent health timeout |
| `TOKEN_TTL` | `300s` | JWT token lifetime (5 minutes) |
| `CLEANUP_INTERVAL` | `60s` | Registry cleanup interval |

## 🚀 Development Usage

### Running Standalone

```bash
# Build and run coordinator
go run cmd/main.go

# Or build first then run
go build -o coordinator cmd/main.go
./coordinator
```

### Testing gRPC Endpoints

```bash
# Check coordinator health
grpcurl -plaintext localhost:50051 grpc.health.v1.Health/Check

# Query system status
grpcurl -plaintext -d '{"include_agents": true}' \
  localhost:50051 titancompute.v1.CoordinatorService/QuerySystemStatus

# Request inference routing (requires agents)
grpcurl -plaintext -d '{
  "client_id": "test-client",
  "model": "llama3.1:8b-instruct-q4_k_m",
  "prompt": "Hello",
  "max_tokens": 50
}' localhost:50051 titancompute.v1.CoordinatorService/RequestInference
```

### Testing REST API

```bash
# Health check
curl http://localhost:8080/api/v1/health

# System status
curl "http://localhost:8080/api/v1/status?include_agents=true"

# Request inference routing
curl -X POST http://localhost:8080/api/v1/inference/request \
  -H "Content-Type: application/json" \
  -d '{
    "client_id": "test-client",
    "model": "llama3.1:8b-instruct-q4_k_m",
    "prompt": "What is machine learning?",
    "max_tokens": 100,
    "temperature": 0.7
  }'
```

### MCDA Scheduler Testing

```bash
# View scheduler decision logs
go run cmd/main.go 2>&1 | grep "MCDA agent selected"

# Check agent scoring details
go run cmd/main.go 2>&1 | grep "agent scoring"
```

## 🔍 API Reference

### REST API Endpoints

| Endpoint | Method | Description | Response |
|----------|--------|-------------|----------|
| `/api/v1/health` | GET | Health check | `{"status": "healthy", "service": "titancompute-coordinator"}` |
| `/api/v1/status` | GET | System status | Agent counts, optional agent details |
| `/api/v1/inference/request` | POST | Request routing | JWT token + agent endpoint |

### gRPC Services

```protobuf
service CoordinatorService {
  rpc RequestInference(InferenceRequest) returns (InferenceResponse);
  rpc RegisterAgent(AgentRegistration) returns (RegistrationResponse);
  rpc ReportHealth(stream HealthUpdate) returns (stream HealthAck);
  rpc QuerySystemStatus(StatusRequest) returns (SystemStatus);
  rpc GetPublicKey(PublicKeyRequest) returns (PublicKeyResponse);
}
```

## 🧮 MCDA Scheduling Algorithm

### Scoring Formula

```
Final Score = (0.40 × VRAM_Score) + (0.30 × Load_Score) + (0.20 × RTT_Score) + (0.10 × Performance_Score)
```

### Scoring Components

1. **VRAM Score (40% weight)**
   - Based on free GPU memory vs model requirements
   - Higher score for more available VRAM

2. **Load Score (30% weight)**
   - Based on current jobs vs maximum capacity
   - Higher score for less loaded agents

3. **RTT Score (20% weight)**
   - Based on network round-trip time
   - Higher score for lower latency

4. **Performance Score (10% weight)**
   - Based on historical throughput (tokens/second)
   - Higher score for better performing agents

### Circuit Breaker States

```
Healthy → Degraded → Half-Open → Offline
   ↑         ↓         ↓         ↓
   └─────────┴─────────┴─────────┘
```

- **Healthy**: Full traffic, normal scoring
- **Degraded**: Reduced scoring weight (50%)
- **Half-Open**: Limited trial traffic
- **Offline**: No traffic, excluded from selection

## 🐛 Debugging

### Common Issues

#### "Failed to listen on port"
```bash
# Check if port is in use
lsof -i :50051 -i :8080

# Kill conflicting processes
sudo kill $(lsof -t -i:50051)
```

#### "Protocol buffer not found"
```bash
# Regenerate protocol buffers
cd .. && ./proto/generate.sh

# Check generated files exist
ls pkg/proto/github.com/titancompute/proto/gen/go/
```

#### "No agents available"
```bash
# Check agent registration
curl "http://localhost:8080/api/v1/status?include_agents=true"

# View coordinator logs
go run cmd/main.go 2>&1 | grep -E "(agent|registration)"
```

### Logs and Monitoring

```bash
# Structured JSON logs
go run cmd/main.go 2>&1 | jq .

# Filter specific components
go run cmd/main.go 2>&1 | jq 'select(.component == "scheduler")'

# Monitor agent health
watch -n 2 'curl -s "http://localhost:8080/api/v1/status?include_agents=true" | jq .agents'
```

## 🏭 Production Deployment

### Docker Container

```bash
# Build coordinator image
docker build -t titancompute-coordinator .

# Run with environment variables
docker run -d \
  -e COORDINATOR_PORT=50051 \
  -e COORDINATOR_HTTP_PORT=8080 \
  -e TOKEN_TTL=300s \
  -p 50051:50051 \
  -p 8080:8080 \
  titancompute-coordinator
```

### High Availability Setup

```bash
# Load balancer in front of coordinators
# Multiple coordinator instances
# Shared agent registry (Redis/etcd)
```

## 📊 Performance Tuning

### JWT Token Configuration

```bash
# Shorter TTL for higher security
export TOKEN_TTL=60s

# Longer TTL for fewer API calls
export TOKEN_TTL=600s
```

### Registry Cleanup

```bash
# More frequent cleanup for dynamic environments
export CLEANUP_INTERVAL=30s

# Less frequent for stable environments
export CLEANUP_INTERVAL=120s
```

### MCDA Weight Tuning

Modify weights in `scheduler/scheduler.go`:

```go
// Memory-optimized (prioritize VRAM)
weights := MCDAWeights{
    VRAMWeight: 0.60,
    JobLoadWeight: 0.20,
    LatencyWeight: 0.15,
    PerformanceWeight: 0.05,
}

// Latency-optimized (prioritize RTT)
weights := MCDAWeights{
    VRAMWeight: 0.30,
    JobLoadWeight: 0.20,
    LatencyWeight: 0.40,
    PerformanceWeight: 0.10,
}
```

## 🔗 Integration

The coordinator integrates with:
- **Agents** via gRPC (registration, health monitoring)
- **Clients** via REST API and gRPC (request routing, JWT tokens)
- **Load Balancers** via health check endpoints
- **Monitoring Systems** via structured JSON logs

For complete system integration, see the main project README.
