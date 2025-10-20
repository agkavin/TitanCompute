# TitanCompute Agent

The TitanCompute Agent is a Python-based service that runs on compute nodes to handle direct LLM inference with intelligent GGUF quantization and JWT authentication.

## 🏗️ What's Inside

```
agent/
├── main.py                 # Entry point - starts the agent service
├── requirements.txt        # Python dependencies
├── pyproject.toml         # Python project configuration
├── uv.lock               # Dependency lock file
├── Dockerfile            # Container build configuration
├── entrypoint.sh         # Container startup script
└── src/                  # Core agent implementation
    ├── agent_server.py   # gRPC server and service coordination
    ├── config.py         # Configuration management
    ├── jwt_validator.py  # JWT token validation
    ├── model_manager.py  # Ollama integration and model management
    ├── quantization.py   # GGUF quantization logic and selection
    ├── stats_collector.py # System resource monitoring
    └── proto/            # Generated protocol buffer files
        ├── titancompute_pb2.py
        └── titancompute_pb2_grpc.py
```

## 🧠 How It Works

### Agent Architecture

```
Client (with JWT) → Agent gRPC Server → JWT Validation → Model Manager → Ollama → Streaming Response
                           ↓
                    Stats Collector → Resource Monitoring → Coordinator Registration
```

### Key Components

1. **Agent Server** (`agent_server.py`)
   - gRPC service for direct client streaming
   - Handles JWT token validation
   - Manages concurrent inference sessions
   - Reports health and metrics to coordinator

2. **Model Manager** (`model_manager.py`)
   - Integrates with Ollama for LLM inference
   - Handles model loading and preloading
   - Manages GGUF quantization selection
   - Streams inference results to clients

3. **Quantization Engine** (`quantization.py`)
   - Complete bartowski GGUF quantization support (Q8_0 to IQ2_M)
   - Memory-aware quantization selection
   - ARM processor optimizations
   - Automatic fallback for memory-constrained environments

4. **JWT Validator** (`jwt_validator.py`)
   - RSA-256 signature verification
   - Token expiry validation
   - Public key retrieval from coordinator
   - Secure client authentication

5. **Stats Collector** (`stats_collector.py`)
   - Real-time VRAM/RAM/CPU monitoring
   - GPU detection and metrics
   - System health reporting
   - Circuit breaker state management

## 🔧 Setup

### Prerequisites

- **Python 3.12+**
- **Ollama** installed and running
- **CUDA** (optional, for GPU acceleration)
- **Protocol buffers** generated

### Development Setup

```bash
# 1. Install Python dependencies
cd agent
pip install -r requirements.txt

# 2. Generate protocol buffers (from project root)
./proto/generate.sh

# 3. Set environment variables
export AGENT_ID=agent-dev
export COORDINATOR_ENDPOINT=localhost:50051
export PUBLIC_HOST=localhost
export AGENT_PORT=50052
export OLLAMA_HOST=http://localhost:11434
export MAX_CONCURRENT_JOBS=4

# 4. Start Ollama (in another terminal)
ollama serve

# 5. Pull a model for testing
ollama pull llama3.1:8b-instruct-q4_k_m

# 6. Run the agent
python main.py
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `AGENT_ID` | `agent-1` | Unique identifier for this agent |
| `COORDINATOR_ENDPOINT` | `localhost:50051` | Coordinator gRPC address |
| `PUBLIC_HOST` | `localhost` | Public hostname for client connections |
| `AGENT_PORT` | `50052` | Port for agent gRPC server |
| `OLLAMA_HOST` | `http://localhost:11434` | Ollama service endpoint |
| `MAX_CONCURRENT_JOBS` | `4` | Maximum concurrent inference sessions |
| `SUPPORTED_MODELS` | `llama3.1:8b-instruct-q4_k_m` | Comma-separated list of supported models |
| `HEARTBEAT_INTERVAL` | `10` | Seconds between coordinator heartbeats |

## 🚀 Development Usage

### Running Standalone

```bash
# Start agent with custom configuration
AGENT_ID=dev-agent AGENT_PORT=50055 python main.py
```

### Testing with Coordinator

```bash
# 1. Start coordinator (in another terminal)
cd ../coordinator && go run cmd/main.go

# 2. Start agent
python main.py

# 3. Check agent registration
curl "http://localhost:8080/api/v1/status?include_agents=true"
```

### Direct gRPC Testing

```bash
# Test agent status
grpcurl -plaintext localhost:50052 titancompute.v1.AgentService/GetStatus

# Test streaming (requires valid JWT token from coordinator)
grpcurl -plaintext -d '{
  "session_token": "your-jwt-token-here",
  "prompt": "Hello, world!",
  "max_tokens": 50
}' localhost:50052 titancompute.v1.AgentService/StreamInference
```

### Model Management

```bash
# List available models
ollama list

# Pull GGUF quantized models
ollama pull hf.co/bartowski/Meta-Llama-3.1-8B-Instruct-GGUF:Q4_K_M
ollama pull hf.co/bartowski/Meta-Llama-3.1-8B-Instruct-GGUF:Q8_0

# Check quantization recommendations
python -c "
from src.quantization import GGUFQuantizationManager
manager = GGUFQuantizationManager()
memory_info = manager.get_system_memory_info()
print(f'Available memory: {memory_info[1]/1024:.1f} GB')
print(f'Recommended tier: {manager.determine_optimal_tier(memory_info[1])}')
"
```

## 🐛 Debugging

### Common Issues

#### "Coordinator connection failed"
```bash
# Check coordinator is running
curl http://localhost:8080/api/v1/health

# Check network connectivity
telnet localhost 50051
```

#### "Ollama connection failed"
```bash
# Check Ollama is running
curl http://localhost:11434/api/version

# Start Ollama if needed
ollama serve
```

#### "Model not found"
```bash
# List available models
ollama list

# Pull required model
ollama pull llama3.1:8b-instruct-q4_k_m
```

#### "JWT validation failed"
```bash
# Check coordinator JWT endpoint
curl http://localhost:8080/api/v1/public-key

# Verify token is not expired
# Check coordinator logs for JWT issues
```

### Logs and Monitoring

```bash
# View agent logs (JSON structured)
python main.py 2>&1 | jq .

# Monitor system resources
python -c "
from src.stats_collector import StatsCollector
collector = StatsCollector()
stats = collector.collect()
print(f'VRAM: {stats.free_vram_mb}/{stats.total_vram_mb} MB')
print(f'RAM: {stats.free_ram_mb}/{stats.total_ram_mb} MB')
print(f'CPU: {stats.cpu_percent}%')
"

# Check model quantization
python -c "
from src.model_manager import ModelManager
manager = ModelManager('http://localhost:11434')
print('Testing model loading...')
# Add your test code here
"
```

## 🏭 Production Deployment

### Docker Container

```bash
# Build agent image
docker build -t titancompute-agent .

# Run with environment variables
docker run -d \
  -e AGENT_ID=agent-prod-1 \
  -e COORDINATOR_ENDPOINT=coordinator:50051 \
  -e PUBLIC_HOST=agent-1.example.com \
  -e OLLAMA_HOST=http://ollama:11434 \
  -p 50052:50052 \
  titancompute-agent
```

### Multi-Agent Setup

```bash
# Agent 1
AGENT_ID=agent-1 AGENT_PORT=50052 python main.py &

# Agent 2  
AGENT_ID=agent-2 AGENT_PORT=50053 python main.py &

# Agent 3
AGENT_ID=agent-3 AGENT_PORT=50054 python main.py &
```

## 📊 Performance Tuning

### Memory Optimization

- **GGUF Quantization:** Automatically selects optimal quantization based on available memory
- **Model Preloading:** Preloads frequently used models to reduce time-to-first-token
- **Memory Monitoring:** Continuous monitoring prevents OOM errors

### Concurrency Settings

```python
# Adjust based on GPU memory and model size
MAX_CONCURRENT_JOBS=2  # For 8GB VRAM
MAX_CONCURRENT_JOBS=4  # For 16GB VRAM  
MAX_CONCURRENT_JOBS=8  # For 32GB VRAM
```

### Quantization Tiers

| Memory | Tier | Quantizations | Quality |
|--------|------|---------------|---------|
| 8GB+ | Premium | Q8_0, Q6_K_L, Q6_K | Near-original |
| 6-8GB | High | Q5_K_M, Q4_K_M, Q4_K_S | Balanced |
| 4-6GB | Good | IQ4_XS, Q3_K_L, IQ3_M | Efficient |
| <4GB | Emergency | Q2_K, IQ2_M | Minimal |

## 🔗 Integration

The agent integrates with:
- **Coordinator** via gRPC (registration, health reporting)
- **Ollama** via HTTP API (model management, inference)
- **Clients** via gRPC streaming (JWT-authenticated inference)

For complete system integration, see the main project README.
