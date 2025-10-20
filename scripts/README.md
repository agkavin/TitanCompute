# TitanCompute Scripts

This directory contains all operational scripts for TitanCompute deployment, validation, and management.

## 📁 Files

| File | Purpose | Description |
|------|---------|-------------|
| `deploy.sh` | Deployment | Main deployment script for TitanCompute system |
| `validate.sh` | Validation | Project structure and component validation |
| `README.md` | Documentation | This file |

## 🚀 Usage

### Deployment Script (`deploy.sh`)

The primary script for managing TitanCompute deployment:

```bash
# Full deployment (recommended)
./scripts/deploy.sh

# Available commands:
./scripts/deploy.sh [command]
```

#### Available Commands

| Command | Description |
|---------|-------------|
| `(no args)` | Full deployment: prerequisites → proto gen → build → start |
| `build` | Only build Docker images |
| `start` | Only start services |
| `stop` | Stop all services |
| `restart` | Restart all services |
| `logs` | Follow all service logs |
| `status` | Show service status |
| `clean` | Complete cleanup (containers, volumes, networks) |

#### Examples

```bash
# Complete setup (first time)
./scripts/deploy.sh

# Quick restart after code changes
./scripts/deploy.sh restart

# View logs
./scripts/deploy.sh logs

# Check status
./scripts/deploy.sh status

# Stop everything
./scripts/deploy.sh stop

# Clean up completely
./scripts/deploy.sh clean
```

### Validation Script (`validate.sh`)

Validates project structure and configuration:

```bash
./scripts/validate.sh
```

**What it checks:**
- ✅ Project file structure
- ✅ Required dependencies (Go, Python, Docker, protoc)
- ✅ Docker containers and networking
- ✅ Protocol buffer generation
- ✅ Configuration files
- ✅ Build artifacts

## 🔧 Prerequisites

### System Requirements

1. **Docker & Docker Compose**
   ```bash
   # Check if installed
   docker --version
   docker-compose --version
   ```

2. **Go 1.21+**
   ```bash
   # Check if installed
   go version
   ```

3. **Python 3.8+**
   ```bash
   # Check if installed
   python3 --version
   ```

4. **Protocol Buffers Compiler**
   ```bash
   # Ubuntu/Debian
   sudo apt install protobuf-compiler
   
   # macOS
   brew install protobuf
   
   # Check if installed
   protoc --version
   ```

### Automated Setup

The deploy script will check prerequisites and provide installation instructions if anything is missing.

## 🏗️ What the Scripts Do

### `deploy.sh` Flow

1. **Prerequisites Check**
   - Verifies Docker, Go, Python, protoc installation
   - Exits with instructions if anything missing

2. **Protocol Buffer Generation**
   - Installs Go protobuf plugins if needed
   - Generates Go stubs in `coordinator/pkg/proto/`
   - Generates Python stubs in `agent/src/proto/`

3. **Docker Image Building**
   - Builds coordinator image (Go-based)
   - Builds agent image (Python-based)

4. **Service Deployment**
   - Uses `docker-compose.yml` in project root
   - Starts coordinator on ports 50051 (gRPC) and 8080 (REST)
   - Starts agents on ports 50052, 50053, etc.
   - Sets up Docker networks and volumes

5. **Health Checks**
   - Waits for coordinator REST API to respond
   - Validates agent registration
   - Shows final service status

### `validate.sh` Flow

1. **Structure Validation**
   - Checks all required source files exist
   - Validates configuration files
   - Verifies Docker and deployment files

2. **Dependency Validation**
   - Tests Go module compilation
   - Tests Python dependency installation
   - Validates protobuf generation

3. **Runtime Validation**
   - Checks if services can start
   - Validates inter-service communication
   - Tests API endpoints

## 🌐 Service Endpoints

When deployed successfully:

| Service | gRPC Port | HTTP Port | Purpose |
|---------|-----------|-----------|---------|
| Coordinator | 50051 | 8080 | Request routing, agent management |
| Agent 1 | 50052 | - | LLM inference, model management |
| Agent 2 | 50053 | - | LLM inference, model management |

### Testing Deployment

```bash
# Check coordinator health
curl http://localhost:8080/api/v1/health

# Check system status
curl http://localhost:8080/api/v1/status

# Test inference request
curl -X POST http://localhost:8080/api/v1/inference/request \
  -H "Content-Type: application/json" \
  -d '{
    "client_id": "test",
    "model": "llama3.1:8b-instruct-q4_k_m",
    "prompt": "Hello",
    "max_tokens": 50
  }'
```

## 🐛 Troubleshooting

### Common Issues

#### "Docker daemon not running"
```bash
sudo systemctl start docker
```

#### "Permission denied accessing Docker socket"
```bash
sudo usermod -aG docker $USER
# Log out and back in
```

#### "Port already in use"
```bash
# Stop conflicting services
./scripts/deploy.sh stop
# Or find and kill process
lsof -i :50051 -i :8080
```

#### "Protocol buffer generation failed"
```bash
# Install missing tools
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

#### "Go build failed"
```bash
# Clean module cache
go clean -modcache
cd coordinator && go mod tidy
```

### Logs and Debugging

```bash
# View all logs
./scripts/deploy.sh logs

# View specific service logs
docker logs titan-coordinator
docker logs titan-agent-1

# Debug network issues
docker network ls
docker network inspect titan-network

# Check container status
docker ps -a
```

## 📝 Notes

- **Scripts are idempotent:** Safe to run multiple times
- **Error handling:** Scripts exit on first error with descriptive messages
- **Cross-platform:** Works on Linux and macOS (Windows via WSL)
- **Development-friendly:** Quick restart/rebuild workflows
- **Production-ready:** Proper health checks and graceful shutdowns

## 🎯 Integration with Client Examples

After successful deployment, test with client examples:

```bash
# REST API client
cd client && python rest_client.py

# gRPC client  
cd client && ./test_curl.sh

# curl tests
cd client && ./test_curl.sh
```
