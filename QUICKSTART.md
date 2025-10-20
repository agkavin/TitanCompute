# 🚀 TitanCompute Quick Start Guide

## Prerequisites Check ✅

Before starting, ensure you have:

```bash
# Check Docker
docker --version
# Expected: Docker version 20.10+ 

# Check Docker Compose
docker-compose --version
# Expected: docker-compose version 1.29+

# Check Go
go version  
# Expected: go version go1.24+

# Check Protocol Buffers
protoc --version
# Expected: libprotoc 3.15+
```

## 30-Second Startup 🏃‍♂️

```bash
# 1. Clone and enter directory
cd /home/marcus/code/TitanCompute

# 2. Deploy everything
cd deploy && ./bootstrap.sh

# 3. Test the system
cd ../client && python test_suite.py

# 4. Check status
cd ../deploy && ./bootstrap.sh status
```

## What Gets Deployed 📦

### Services Started:
- **Coordinator** (`:50051`) - MCDA scheduling + JWT auth
- **Agent 1** (`:50052`) - GGUF quantization + model serving  
- **Agent 2** (`:50053`) - GGUF quantization + model serving

### Docker Containers:
```bash
docker ps
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
