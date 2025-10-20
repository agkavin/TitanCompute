# 🚀 TitanCompute Quick Start Guide

Get TitanCompute running in development or deployment mode in minutes.

## Prerequisites ✅

**For Deployment Mode:**
- Docker 20.10+ and Docker Compose 1.29+

**For Development Mode:**
- Docker + Go 1.21+ + Python 3.12+ + `protoc` 3.15+

## 🚀 Deployment Mode (Production)

**For containerized production deployment:**

```bash
# 1. Deploy everything
./scripts/deploy.sh

# 2. Test the system
cd client && python rest_client.py

# 3. Check status
./scripts/deploy.sh status
```

**Services deployed:**
- Coordinator: `localhost:50051` (gRPC), `localhost:8080` (REST)
- Agent 1: `localhost:50052` (gRPC) 
- Agent 2: `localhost:50053` (gRPC)

## 🛠️ Development Mode (Local)

**For local development and testing:**

### Full Development Setup

```bash
# 1. Generate protocol buffers
./proto/generate.sh

# 2. Start coordinator
cd coordinator && go run cmd/main.go &

# 3. Start agent (separate terminal)
cd agent
pip install -r requirements.txt
AGENT_ID=agent-1 AGENT_PORT=50052 python main.py &

# 4. Test with client examples
cd client && python rest_client.py
```

### Mixed Mode (Development)

```bash
# Use deployment for agents, run coordinator locally
./scripts/deploy.sh

# Stop coordinator container and run locally
docker stop titan-coordinator
cd coordinator && go run cmd/main.go
```

## 🧪 Testing Your Setup

### Quick Health Check

```bash
# REST API health check
curl http://localhost:8080/api/v1/health

# System status
curl "http://localhost:8080/api/v1/status?include_agents=true"

# Test inference request
curl -X POST http://localhost:8080/api/v1/inference/request \
  -H "Content-Type: application/json" \
  -d '{
    "client_id": "quickstart-test",
    "model": "llama3.1:8b-instruct-q4_k_m",
    "prompt": "Hello, TitanCompute!",
    "max_tokens": 50
  }'
```

### Client Examples

```bash
cd client

# REST API client example
python rest_client.py

# Shell/curl tests
./test_curl.sh
```

## 🔧 Development Workflow

### Code Changes

```bash
# After modifying coordinator code
cd coordinator && go build -o coordinator cmd/main.go
./coordinator

# After modifying agent code
cd agent && python main.py

# After modifying protocol buffers
./proto/generate.sh
# Then rebuild both services
```

### Development Tools

```bash
# View logs (deployment mode)
./scripts/deploy.sh logs

# Monitor specific service
docker logs -f titan-coordinator
docker logs -f titan-agent-1

# View service status
./scripts/deploy.sh status

# Restart services
./scripts/deploy.sh restart
# Should show:
# titan-coordinator
# titan-agent-1  
# titan-agent-2
```

### Automatic Setup:
- ✅ Protocol buffer generation (Go + Python)
- ✅ Docker image building with dependencies
- ✅ Network configuration (`titan-network`)
- ✅ Volume mounting for logs and models
- ✅ Health checks for all services

## Verification Steps 🔍

### 1. Check Service Health
```bash
# All services should be healthy
docker-compose ps

# Check coordinator health
grpcurl -plaintext localhost:50051 grpc.health.v1.Health/Check

# Check agent health  
grpcurl -plaintext localhost:50052 grpc.health.v1.Health/Check
```

### 2. Test System Status
```bash
# Query system with circuit breaker info
grpcurl -plaintext -d '{"include_agents": true}' \
  localhost:50051 titancompute.v1.CoordinatorService/QuerySystemStatus
```

Expected output:
```json
{
  "totalAgents": 2,
  "healthyAgents": 2,
  "uptime": "0.1 hours",
  "agents": [
    {
      "agentId": "agent-1",
      "status": "healthy",
      "freeVramMb": 4096,
      "runningJobs": 0
    }
  ]
}
```

### 3. Test Inference Flow
```bash
# Request inference (triggers MCDA scheduling)
grpcurl -plaintext -d '{
  "client_id": "quickstart-test",
  "model": "llama3.1:8b-instruct-q4_k_m", 
  "prompt": "What is TitanCompute?",
  "max_tokens": 50
}' localhost:50051 titancompute.v1.CoordinatorService/RequestInference
```

Expected response includes:
- `agent_endpoint`: Selected agent address
- `session_token`: JWT token for direct agent access
- `agent_id`: Which agent was chosen by MCDA

## Troubleshooting 🔧

### Common Issues:

#### 1. "No healthy agents available"
```bash
# Check agent logs
docker-compose logs agent-1

# Verify Ollama is running in agent
docker exec -it titan-agent-1 ollama list
```

#### 2. "Failed to generate protocol buffers"
```bash
# Install protobuf compiler
sudo apt install protobuf-compiler  # Ubuntu/Debian
brew install protobuf               # macOS

# Manually generate
cd proto && ./generate.sh
```

#### 3. "Docker permission denied"
```bash
# Add user to docker group
sudo usermod -aG docker $USER
# Logout and login again
```

#### 4. "Port already in use"
```bash
# Check what's using the ports
sudo netstat -tulpn | grep :50051

# Stop conflicting services
sudo systemctl stop <service-name>
```

### Debug Commands:
```bash
# View all logs
docker-compose logs -f

# Check specific service
docker-compose logs -f coordinator

# Enter agent container
docker exec -it titan-agent-1 bash

# Check system resources
docker stats titan-coordinator titan-agent-1 titan-agent-2
```

## Next Steps 🎯

Once deployment is successful:

1. **Run Full Test Suite**: `python client/test_suite.py`
2. **Try Different Models**: Test various GGUF quantizations
3. **Load Testing**: Multiple concurrent clients
4. **Monitor MCDA**: Watch agent selection in logs
5. **Circuit Breaker**: Simulate failures to test fault tolerance

## Need Help? 📚

- **Documentation**: Check `docs/` folder
- **Configuration**: See `README.md` environment variables section
- **Architecture**: Review `.prompts/overview.md`
- **Logs**: All services use structured JSON logging

---

**🎉 Welcome to TitanCompute M2 Production!**

#### **Option 1: Full Deployment (Recommended)**
```bash
# 1. Prerequisites: Docker, Go 1.24+, protoc
cd /home/marcus/code/TitanCompute/deploy
./bootstrap.sh

# 2. Test all M2 features
cd ../client
python test_suite.py

# 3. Monitor services
cd ../deploy
./bootstrap.sh status
```

#### **Option 2: Quick Start Guide**
```bash
# Use the new quick start guide
cat QUICKSTART.md
```

#### **Option 3: Development Mode**
```bash
# Run coordinator
cd coordinator && go run cmd/main.go

# Run agent (separate terminal)
cd agent && python main.py

# Test (separate terminal)
cd client && python test_suite.py
```