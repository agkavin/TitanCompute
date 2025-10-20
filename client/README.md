# TitanCompute Client Examples

This directory contains clean, focused client examples for testing and integrating with TitanCompute.

## 📁 Files

| File | Type | Description |
|------|------|-------------|
| `rest_client.py` | Python | REST API client example using HTTP requests |
| `test_curl.sh` | Bash | Shell script for testing REST API with curl |
| `README.md` | Documentation | This file |

## 🔧 Prerequisites

### For Python Clients (`rest_client.py`)

1. **Python 3.8+** installed
2. **Required Python packages:**
   ```bash
   pip install requests
   ```

### For Shell Testing (`test_curl.sh`)

1. **curl** installed
2. **jq** installed (for JSON formatting)
   ```bash
   # Ubuntu/Debian
   sudo apt install curl jq
   
   # macOS
   brew install curl jq
   ```

### System Requirements

- **TitanCompute coordinator** running on `localhost:50051` (gRPC) and `localhost:8080` (REST)
- **TitanCompute agents** registered and healthy (for full flow testing)

## 🚀 Usage

### Start TitanCompute System

Before running any client examples, start the TitanCompute system:

```bash
# From project root
./scripts/deploy.sh
```

Wait for the system to be ready:
- Coordinator: `localhost:50051` (gRPC), `localhost:8080` (REST)
- Agents: `localhost:50052`, `localhost:50053`, etc.

### Run REST API Client Example

```bash
cd client
python rest_client.py
```

**What it does:**
1. ✅ Health check via REST API
2. ✅ System status check
3. ✅ Request inference routing (gets JWT token + agent endpoint)  
4. ✅ Connect directly to agent with JWT token
5. ✅ Stream inference results

### Run Shell/curl Tests

```bash
cd client
./test_curl.sh
```

**What it tests:**
1. ✅ `GET /api/v1/health` - Health check
2. ✅ `GET /api/v1/status` - System status  
3. ✅ `POST /api/v1/inference/request` - Get token + agent info
4. ✅ Error handling with invalid requests

## 🔄 Client Flow Architecture

```
1. Client → REST API (localhost:8080) → Get JWT token + agent endpoint
2. Client → Direct gRPC (agent:505XX) → Stream inference with JWT validation
```

## 🏗️ Architecture Note

TitanCompute uses a **hybrid communication architecture** for optimal performance and usability:

- **Client ↔ Coordinator**: **REST API** (simple, universal, web-friendly)
- **Client ↔ Agent**: **Direct gRPC streaming** (high-performance inference)
- **Agent ↔ Coordinator**: **gRPC** (internal system communication)

This design provides the best of both worlds: easy integration via REST API and high-performance streaming where it matters most.

### REST API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/health` | GET | Health check |
| `/api/v1/status` | GET | System status (add `?include_agents=true` for details) |
| `/api/v1/inference/request` | POST | Request inference routing |

### Example REST API Request

```bash
curl -X POST http://localhost:8080/api/v1/inference/request \
  -H "Content-Type: application/json" \
  -d '{
    "client_id": "example-client",
    "model": "llama3.1:8b-instruct-q4_k_m",
    "prompt": "What is machine learning?",
    "max_tokens": 100,
    "temperature": 0.7
  }'
```

### Example Response

```json
{
  "agent_endpoint": "localhost:50052",
  "session_token": "eyJhbGciOiJSUzI1NiIs...",
  "expires_at": 1729123456,
  "job_id": "job_abc123",
  "estimated_rtt_ms": 12.5,
  "agent_id": "agent-1"
}
```

## 🐛 Troubleshooting

### "Coordinator not available"
- Ensure coordinator is running: `docker ps | grep titan-coordinator`
- Check coordinator logs: `docker logs titan-coordinator`
- Verify ports: `curl http://localhost:8080/api/v1/health`

### "No agents available"
- Check agent status: `curl "http://localhost:8080/api/v1/status?include_agents=true"`
- Ensure agents are running: `docker ps | grep titan-agent`
- Check agent logs: `docker logs titan-agent-1`

### "Import errors" (Python clients)
- Install dependencies: `pip install requests`
- Generate protobuf: `cd .. && ./proto/generate.sh`

### "JWT token expired"
- Tokens expire after 5 minutes by default
- Request a new token via REST API
- Check token expiry in response: `expires_at` field

## 💡 Integration Tips

### For Your Applications

1. **Use REST API for simplicity:** Easy HTTP requests, JSON responses
2. **Direct agent streaming:** High-performance gRPC inference
3. **Handle token expiry:** Check `expires_at` and refresh tokens
4. **Direct agent connection:** Never proxy data through coordinator
5. **Error handling:** Check status codes and error messages

### Token Management

```python
import time

# Check if token is expired
def is_token_expired(expires_at):
    return time.time() >= expires_at

# Refresh token when needed
if is_token_expired(token_info['expires_at']):
    token_info = request_new_token()
```

## 📝 Notes

- **No fake implementations:** All examples use real TitanCompute APIs
- **No fallbacks:** Examples fail gracefully if services aren't available  
- **Production ready:** Examples demonstrate real integration patterns
- **JWT security:** All agent connections use JWT token validation
- **Zero-proxy:** Direct client-to-agent streaming for optimal performance
