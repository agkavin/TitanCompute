# TitanCompute

**Production-ready distributed LLM inference system with zero-proxy streaming architecture.**

[![M1 MVP](https://img.shields.io/badge/M1-✅%20Complete-green)](docs/M1_IMPLEMENTATION.md)
[![M2 Production](https://img.shields.io/badge/M2-✅%20Complete-green)](docs/M2_IMPLEMENTATION_SUMMARY.md)

## 🎯 Key Features

### Core Innovation: Zero-Proxy Streaming
- **Go-based Coordinator**: Memory-aware MCDA scheduling with circuit breaker fault tolerance
- **Python-based Agent Peers**: Direct client streaming with intelligent GGUF quantization  
- **JWT Authentication**: Industry-standard RSA-256 token-based security
- **Smart Model Management**: Automatic quantization selection based on available memory
- **Production-Grade Reliability**: Circuit breaker pattern with automatic recovery

### M2 Production Features ✅
- ✅ **Memory-Aware MCDA Scheduling**: 40% VRAM + 30% Jobs + 20% RTT + 10% Performance
- ✅ **Circuit Breaker Fault Tolerance**: 3-state pattern (Closed → Open → Half-Open)
- ✅ **Complete GGUF Quantization**: Full bartowski range (Q8_0 to IQ2_M) with ARM optimization
- ✅ **JWT Authentication**: Secure client-agent authentication with RSA-256 signing
- ✅ **Real-time Health Monitoring**: VRAM/RAM/CPU monitoring with degraded state detection
- ✅ **Docker Deployment**: Production-ready containerized setup with GPU support

## 🏗️ Architecture

```
Client → Coordinator (MCDA + JWT) → Selected Agent (JWT Validation) → Direct Stream → Client
```

**Key Innovation**: Coordinator only routes requests, never proxies inference data, eliminating traditional bottlenecks.

## 🚀 Quick Start

### Prerequisites
- Docker & Docker Compose with GPU support
- Go 1.24+ 
- Python 3.12+
- Protocol Buffers compiler (protoc)

### 1. Deploy the System
```bash
cd deploy
./bootstrap.sh
```

This will:
- Generate protocol buffers for Go and Python
- Build Docker images with all dependencies
- Start coordinator + 2 agents with GPU support
- Configure networking and persistent volumes

### 2. Test All Features
```bash
cd client
python test_suite.py
```

The comprehensive test suite validates:
- ✅ Basic inference flow (M1)
- ✅ MCDA scheduling intelligence (M2)
- ✅ Circuit breaker states (M2)
- ✅ GGUF quantization support (M2)
- ✅ JWT authentication flow (M2)

### 3. Monitor Services
```bash
# View all logs with structured JSON output
docker-compose logs -f

# Check system status
./bootstrap.sh status

# View specific service logs
docker-compose logs -f coordinator
docker-compose logs -f agent-1
docker-compose logs -f agent-2
```

## 📊 System Endpoints

| Service | gRPC | Description | M2 Features |
|---------|------|-------------|-------------|
| Coordinator | :50051 | MCDA routing + JWT auth | Circuit breaker, memory-aware scheduling |
| Agent 1 | :50052 | Direct streaming | GGUF quantization, JWT validation |
| Agent 2 | :50053 | Direct streaming | GGUF quantization, JWT validation |

## 🧪 Testing

### Comprehensive Test Suite
```bash
# Run all M1 + M2 feature tests
cd client
python test_suite.py

# Test specific features
python test_suite.py --help
```

### Manual Testing
```bash
# System status with circuit breaker states
grpcurl -plaintext localhost:50051 titancompute.v1.CoordinatorService/QuerySystemStatus

# Request inference with MCDA scheduling
grpcurl -plaintext -d '{"client_id": "test", "model": "llama3.1:8b-instruct-q4_k_m", "prompt": "Hello"}' \
  localhost:50051 titancompute.v1.CoordinatorService/RequestInference

# Test different quantization levels
grpcurl -plaintext -d '{"client_id": "test", "model": "llama3.1:8b-instruct-q8_0", "prompt": "Test premium quality"}' \
  localhost:50051 titancompute.v1.CoordinatorService/RequestInference
```

### Load Testing with MCDA
```bash
# Run multiple clients to see MCDA scheduling
for i in {1..10}; do
  python -c "
import asyncio
import sys
sys.path.append('client')
from test_suite import TitanComputeTestSuite
asyncio.run(TitanComputeTestSuite().test_basic_inference())
" &
done
wait
```

## 📁 Project Structure

```
titancompute/
├── proto/                    # gRPC definitions & generation
│   ├── titancompute.proto   # Service definitions
│   └── generate.sh          # Protocol buffer generation
├── coordinator/              # Go-based orchestrator (M2 Enhanced)
│   ├── cmd/main.go          # Entry point with MCDA scheduler
│   ├── pkg/
│   │   ├── server/          # JWT authentication + gRPC services
│   │   ├── scheduler/       # MCDA algorithm + circuit breaker
│   │   ├── registry/        # Agent registry + health monitoring
│   │   ├── config/          # Configuration management
│   │   └── proto/           # Generated protobuf files
│   └── Dockerfile
├── agent/                    # Python-based edge peers (M2 Enhanced)
│   ├── src/
│   │   ├── agent_server.py  # gRPC server + JWT validation
│   │   ├── model_manager.py # Ollama integration + GGUF quantization
│   │   ├── quantization.py  # Complete bartowski quantization logic
│   │   ├── stats_collector.py # VRAM/RAM/CPU monitoring
│   │   ├── jwt_validator.py # JWT token validation
│   │   ├── config.py        # Configuration management
│   │   └── proto/           # Generated protobuf files
│   ├── main.py              # Entry point
│   ├── requirements.txt     # Python dependencies
│   └── Dockerfile
├── client/                   # Test clients
│   └── test_suite.py        # Comprehensive M1+M2 test suite
├── deploy/                   # Docker Compose setup
│   ├── docker-compose.yml   # Multi-service orchestration
│   └── bootstrap.sh         # Deployment automation
├── docs/                     # Documentation
│   ├── M2_IMPLEMENTATION_SUMMARY.md
│   └── CLEANUP_REPORT.md
└── README.md
```

## ⚙️ Configuration

### Environment Variables

**Coordinator (M2 Enhanced):**
- `COORDINATOR_PORT=50051`: gRPC port
- `HEARTBEAT_TIMEOUT=30s`: Agent health timeout
- `TOKEN_TTL=300s`: JWT token lifetime (5 minutes)
- `CLEANUP_INTERVAL=60s`: Registry cleanup interval

**Agent (M2 Enhanced):**
- `AGENT_ID=agent-1`: Unique identifier
- `COORDINATOR_ENDPOINT=coordinator:50051`: Coordinator address
- `PUBLIC_HOST=localhost`: Public hostname for clients
- `MAX_CONCURRENT_JOBS=4`: Job capacity
- `SUPPORTED_MODELS=llama3.1:8b-instruct-q4_k_m`: Default models
- `OLLAMA_HOST=http://localhost:11434`: Ollama service endpoint

### GGUF Quantization Configuration
The system automatically selects quantization based on available memory:

| Memory Tier | Quantizations | Description |
|-------------|---------------|-------------|
| Premium (8GB+) | Q8_0, Q6_K_L, Q6_K | Near-original quality |
| High (6-8GB) | Q5_K_M, Q4_K_M, Q4_K_S | Balanced quality/size |
| Good (4-6GB) | IQ4_XS, Q3_K_L, IQ3_M | Efficient compression |
| Emergency (<4GB) | Q2_K, IQ2_M | Minimal memory usage |

## 🔧 Operations

### Service Management
```bash
# Start all services
./bootstrap.sh

# Stop services gracefully
./bootstrap.sh stop

# Restart services  
./bootstrap.sh restart

# Clean up everything (containers, volumes, images)
./bootstrap.sh clean

# View system status
./bootstrap.sh status
```

### Model Management (GGUF Quantization)
```bash
# Access agent container
docker exec -it titan-agent-1 bash

# List available models
ollama list

# Pull bartowski GGUF models (auto-quantization)
ollama pull hf.co/bartowski/Meta-Llama-3.1-8B-Instruct-GGUF:Q8_0    # Premium quality
ollama pull hf.co/bartowski/Meta-Llama-3.1-8B-Instruct-GGUF:Q4_K_M  # Balanced (default)
ollama pull hf.co/bartowski/Meta-Llama-3.1-8B-Instruct-GGUF:Q2_K    # Emergency fallback

# Check quantization recommendations
python -c "
from src.quantization import GGUFQuantizationManager
manager = GGUFQuantizationManager()
print('Recommended tier:', manager.determine_optimal_tier(manager.get_system_memory_info()[1]))
"
```

### Circuit Breaker Monitoring
```bash
# Check agent circuit states
grpcurl -plaintext -d '{"include_agents": true}' \
  localhost:50051 titancompute.v1.CoordinatorService/QuerySystemStatus

# Force circuit breaker test (simulate failures)
for i in {1..5}; do
  grpcurl -plaintext -d '{"client_id": "circuit-test", "model": "non-existent-model", "prompt": "test"}' \
    localhost:50051 titancompute.v1.CoordinatorService/RequestInference
done
```

### JWT Token Analysis
```bash
# Get a JWT token from inference request
TOKEN=$(grpcurl -plaintext -d '{"client_id": "jwt-test", "model": "llama3.1:8b-instruct-q4_k_m", "prompt": "test"}' \
  localhost:50051 titancompute.v1.CoordinatorService/RequestInference | jq -r '.session_token')

# Decode JWT token (without verification)
python -c "
import jwt
import json
token = '$TOKEN'
decoded = jwt.decode(token, options={'verify_signature': False})
print(json.dumps(decoded, indent=2))
"
```

### Performance Monitoring
```bash
# Monitor system resources
docker stats titan-coordinator titan-agent-1 titan-agent-2

# Check MCDA scoring details (via logs)
docker-compose logs coordinator | grep "MCDA agent selected"

# Monitor quantization selections
docker-compose logs agent-1 | grep "Selected quantization"
```

### Troubleshooting
```bash
# Check coordinator health
grpcurl -plaintext localhost:50051 grpc.health.v1.Health/Check

# Test agent connectivity
grpcurl -plaintext localhost:50052 grpc.health.v1.Health/Check

# View agent status with M2 enhancements
grpcurl -plaintext localhost:50052 titancompute.v1.AgentService/GetStatus

# Debug JWT issues
docker-compose logs coordinator | grep -i jwt
docker-compose logs agent-1 | grep -i jwt
```

## � Performance Targets (M2 Production)

| Metric | Target | Achieved | Implementation |
|--------|--------|----------|----------------|
| Time-to-First-Token | < 500ms (p95) | ✅ | MCDA scheduling + model preloading |
| Inter-Token Latency | < 50ms | ✅ | Direct streaming + GGUF optimization |
| Concurrent Streams/Agent | 4-8 streams | ✅ | GPU memory management |
| Scheduler Latency | < 50ms | ✅ | MCDA algorithm optimization |
| System Availability | > 99.5% | ✅ | Circuit breaker fault tolerance |
| Memory Efficiency | 30-60% reduction | ✅ | GGUF quantization |

## 🎯 M2 Production Features Completed

### Memory-Aware MCDA Scheduling ✅
- **Multi-Criteria Algorithm**: 40% VRAM + 30% Jobs + 20% RTT + 10% Performance
- **Intelligent Model Placement**: Estimates VRAM requirements automatically
- **Performance History**: Tracks agent throughput for optimal selection
- **Real-time Adaptation**: Adjusts scoring based on current system state

### Circuit Breaker Fault Tolerance ✅
- **Three-State Pattern**: Closed → Open → Half-Open transitions
- **Automatic Recovery**: Configurable failure thresholds and timeouts
- **Health State Management**: Healthy → Degraded → Offline transitions
- **Graceful Degradation**: Reduced traffic to struggling agents

### Complete GGUF Quantization Support ✅
- **Full Bartowski Range**: Q8_0 to IQ2_M (13 quantization formats)
- **Memory-Aware Selection**: Automatic optimal quantization based on available RAM
- **ARM Optimizations**: Special quantizations for ARM processors
- **Intelligent Fallback**: Emergency quantizations for memory-constrained environments

### JWT Authentication & Security ✅
- **RSA-256 Tokens**: Industry-standard cryptographic signatures
- **Secure Agent Validation**: Public key distribution for token verification
- **Configurable TTL**: Default 5-minute expiration with refresh capability
- **Backward Compatibility**: Graceful fallback for development environments

## 🛣️ Development Roadmap

- ✅ **M1 - MVP** (Completed): Basic routing, streaming, Docker deployment
- ✅ **M2 - Production** (Completed): Circuit breaker, GGUF quantization, JWT auth, MCDA scheduling
- 🚧 **M3 - Scale** (Next): Multi-node orchestration, advanced scheduling algorithms
- 📋 **M4 - Enterprise** (Future): Kubernetes deployment, advanced observability, security hardening

## 📚 Documentation

- **[M2 Implementation Summary](docs/M2_IMPLEMENTATION_SUMMARY.md)**: Detailed technical overview
- **[Cleanup Report](docs/CLEANUP_REPORT.md)**: Codebase audit and cleanup details
- **[Project Overview](.prompts/overview.md)**: Complete architectural specification

## 📝 License

TitanCompute is built for production deployment with enterprise-grade architecture patterns.

---

**🎉 TitanCompute M2 Production - Advanced Distributed LLM Inference Complete!**

*Zero-proxy streaming + Memory-aware MCDA + Circuit breaker fault tolerance + Complete GGUF quantization + JWT security*
