#!/bin/bash
# TitanCompute M2 Production Deployment Script

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "🚀 TitanCompute M2 Production Deployment"
echo "======================================="

# Check prerequisites
check_prerequisites() {
    echo "📋 Checking prerequisites..."
    
    # Check Docker
    if ! command -v docker >/dev/null 2>&1; then
        echo "❌ Docker is required but not installed"
        exit 1
    fi
    
    # Check Docker Compose
    if ! command -v docker-compose >/dev/null 2>&1 && ! docker compose version >/dev/null 2>&1; then
        echo "❌ Docker Compose is required but not installed"
        exit 1
    fi
    
    # Check Go
    if ! command -v go >/dev/null 2>&1; then
        echo "❌ Go is required but not installed"
        exit 1
    fi
    
    # Check protoc
    if ! command -v protoc >/dev/null 2>&1; then
        echo "⚠️ protoc not found, installing protobuf compiler..."
        # Instructions for user
        echo "Please install protobuf compiler:"
        echo "  Ubuntu/Debian: sudo apt install protobuf-compiler"
        echo "  macOS: brew install protobuf"
        echo "  Or download from: https://github.com/protocolbuffers/protobuf/releases"
        exit 1
    fi
    
    echo "✅ Prerequisites check passed"
}

# Generate protocol buffers
generate_protos() {
    echo "📦 Generating protocol buffers..."
    cd "$PROJECT_ROOT"
    
    # Install Go protoc plugins if not present
    if ! command -v protoc-gen-go >/dev/null 2>&1; then
        echo "Installing protoc-gen-go..."
        go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
    fi
    
    if ! command -v protoc-gen-go-grpc >/dev/null 2>&1; then
        echo "Installing protoc-gen-go-grpc..."
        go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
    fi
    
    # Add Go bin to PATH for this session
    export PATH=$PATH:$(go env GOPATH)/bin
    
    # Generate protocol buffers
    ./proto/generate.sh
    
    echo "✅ Protocol buffers generated"
}

# Build Docker images
build_images() {
    echo "🔨 Building Docker images..."
    cd "$SCRIPT_DIR"
    
    # Build coordinator
    echo "🏗️ Building coordinator..."
    docker build -t titancompute-coordinator:latest ../coordinator/
    
    # Build agent
    echo "🏗️ Building agent..."
    docker build -t titancompute-agent:latest ../agent/
    
    echo "✅ Docker images built"
}

# Initialize volumes and networks
init_infrastructure() {
    echo "💾 Initializing infrastructure..."
    cd "$SCRIPT_DIR"
    
    # Create volumes
    docker volume create titan-agent1-models || true
    docker volume create titan-agent2-models || true
    docker volume create titan-agent1-logs || true
    docker volume create titan-agent2-logs || true
    docker volume create titan-coordinator-logs || true
    
    # Create network
    docker network create titan-network || true
    
    echo "✅ Infrastructure initialized"
}

# Start services
start_services() {
    echo "🎬 Starting TitanCompute services..."
    cd "$SCRIPT_DIR"
    
    # Start with docker compose
    if command -v docker-compose >/dev/null 2>&1; then
        docker-compose up -d
    else
        docker compose up -d
    fi
    
    echo "⏳ Waiting for services to be ready..."
    sleep 10
    
    # Check coordinator health
    for i in {1..30}; do
        if curl -s http://localhost:8080/health >/dev/null 2>&1; then
            echo "✅ Coordinator is healthy"
            break
        fi
        if [ $i -eq 30 ]; then
            echo "⚠️ Coordinator health check timeout"
        fi
        sleep 2
    done
    
    echo "✅ Services started"
}

# Show status
show_status() {
    echo "📊 Service Status:"
    echo "=================="
    
    if command -v docker-compose >/dev/null 2>&1; then
        docker-compose ps
    else
        docker compose ps
    fi
    
    echo ""
    echo "🔗 Endpoints:"
    echo "   Coordinator: localhost:50051 (gRPC), localhost:8080 (HTTP)"
    echo "   Agent 1:     localhost:50052 (gRPC)"
    echo "   Agent 2:     localhost:50053 (gRPC)"
    echo ""
    echo "📖 View logs:"
    echo "   All services: docker-compose logs -f"
    echo "   Coordinator: docker-compose logs -f coordinator"
    echo "   Agent 1:     docker-compose logs -f agent-1"
    echo "   Agent 2:     docker-compose logs -f agent-2"
    echo ""
    echo "🧪 Test the system:"
    echo "   cd ../client && python test_client.py"
}

# Main execution
main() {
    check_prerequisites
    generate_protos
    build_images
    init_infrastructure
    start_services
    show_status
    
    echo ""
    echo "🎉 TitanCompute M1 deployment complete!"
    echo "   The system is now ready for testing."
}

# Help function
show_help() {
    echo "TitanCompute M1 Bootstrap Script"
    echo "Usage: $0 [command]"
    echo ""
    echo "Commands:"
    echo "  help     Show this help"
    echo "  build    Build Docker images only"
    echo "  start    Start services only"
    echo "  stop     Stop services"
    echo "  restart  Restart services"
    echo "  logs     Show service logs"
    echo "  status   Show service status"
    echo "  clean    Stop and remove all containers/volumes"
    echo ""
    echo "Default: Run full bootstrap (build + start)"
}

# Handle commands
case "${1:-}" in
    "help")
        show_help
        ;;
    "build")
        check_prerequisites
        generate_protos
        build_images
        ;;
    "start")
        start_services
        show_status
        ;;
    "stop")
        cd "$SCRIPT_DIR"
        if command -v docker-compose >/dev/null 2>&1; then
            docker-compose down
        else
            docker compose down
        fi
        ;;
    "restart")
        cd "$SCRIPT_DIR"
        if command -v docker-compose >/dev/null 2>&1; then
            docker-compose restart
        else
            docker compose restart
        fi
        show_status
        ;;
    "logs")
        cd "$SCRIPT_DIR"
        if command -v docker-compose >/dev/null 2>&1; then
            docker-compose logs -f
        else
            docker compose logs -f
        fi
        ;;
    "status")
        show_status
        ;;
    "clean")
        cd "$SCRIPT_DIR"
        echo "🧹 Cleaning up TitanCompute deployment..."
        if command -v docker-compose >/dev/null 2>&1; then
            docker-compose down -v --remove-orphans
        else
            docker compose down -v --remove-orphans
        fi
        docker volume rm titan-agent1-models titan-agent2-models titan-agent1-logs titan-agent2-logs titan-coordinator-logs 2>/dev/null || true
        docker network rm titan-network 2>/dev/null || true
        echo "✅ Cleanup complete"
        ;;
    "")
        main
        ;;
    *)
        echo "❌ Unknown command: $1"
        show_help
        exit 1
        ;;
esac
