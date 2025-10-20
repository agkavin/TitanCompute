#!/bin/bash
# TitanCompute M1 Project Validation

set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
echo "🔍 Validating TitanCompute M1 Project Structure"
echo "=============================================="

# Check project structure
check_structure() {
    echo "📁 Checking project structure..."
    
    required_files=(
        "proto/titancompute.proto"
        "proto/generate.sh"
        "coordinator/go.mod"
        "coordinator/cmd/main.go"
        "coordinator/pkg/config/config.go"
        "coordinator/pkg/registry/registry.go"
        "coordinator/pkg/scheduler/scheduler.go"
        "coordinator/pkg/server/server.go"
        "coordinator/Dockerfile"
        "agent/main.py"
        "agent/src/config.py"
        "agent/src/agent_server.py"
        "agent/src/model_manager.py"
        "agent/src/stats_collector.py"
        "agent/requirements.txt"
        "agent/Dockerfile"
        "agent/entrypoint.sh"
        "client/test_client.py"
        "deploy/docker-compose.yml"
        "deploy/bootstrap.sh"
        "README.md"
    )
    
    missing_files=()
    for file in "${required_files[@]}"; do
        if [ ! -f "$PROJECT_ROOT/$file" ]; then
            missing_files+=("$file")
        else
            echo "✅ $file"
        fi
    done
    
    if [ ${#missing_files[@]} -eq 0 ]; then
        echo "✅ All required files present"
    else
        echo "❌ Missing files:"
        printf '%s\n' "${missing_files[@]}"
        return 1
    fi
}

# Check Go build
check_go_build() {
    echo "🔧 Checking Go coordinator build..."
    cd "$PROJECT_ROOT/coordinator"
    
    if go build -o /tmp/coordinator ./cmd/main.go; then
        echo "✅ Go coordinator builds successfully"
        rm -f /tmp/coordinator
    else
        echo "❌ Go coordinator build failed"
        return 1
    fi
}

# Check Python syntax
check_python_syntax() {
    echo "🐍 Checking Python agent syntax..."
    cd "$PROJECT_ROOT/agent"
    
    python_files=(
        "main.py"
        "src/config.py"
        "src/agent_server.py"
        "src/model_manager.py"
        "src/stats_collector.py"
    )
    
    for file in "${python_files[@]}"; do
        if python -m py_compile "$file" 2>/dev/null; then
            echo "✅ $file syntax valid"
        else
            echo "❌ $file syntax error"
            return 1
        fi
    done
}

# Check Docker files
check_docker_files() {
    echo "🐳 Checking Docker files..."
    
    if [ -f "$PROJECT_ROOT/coordinator/Dockerfile" ]; then
        echo "✅ Coordinator Dockerfile exists"
    else
        echo "❌ Coordinator Dockerfile missing"
        return 1
    fi
    
    if [ -f "$PROJECT_ROOT/agent/Dockerfile" ]; then
        echo "✅ Agent Dockerfile exists"
    else
        echo "❌ Agent Dockerfile missing"
        return 1
    fi
    
    if [ -f "$PROJECT_ROOT/deploy/docker-compose.yml" ]; then
        echo "✅ Docker Compose file exists"
    else
        echo "❌ Docker Compose file missing"
        return 1
    fi
}

# Check executable permissions
check_permissions() {
    echo "🔐 Checking executable permissions..."
    
    executables=(
        "proto/generate.sh"
        "agent/main.py"
        "agent/entrypoint.sh"
        "client/test_client.py"
        "deploy/bootstrap.sh"
    )
    
    for file in "${executables[@]}"; do
        if [ -x "$PROJECT_ROOT/$file" ]; then
            echo "✅ $file is executable"
        else
            echo "⚠️ $file is not executable (fixing...)"
            chmod +x "$PROJECT_ROOT/$file"
        fi
    done
}

# Generate summary
generate_summary() {
    echo ""
    echo "📋 TitanCompute M1 Project Summary"
    echo "================================="
    echo ""
    echo "🎯 **M1 MVP Features Implemented:**"
    echo "   ✅ Zero-proxy streaming architecture"
    echo "   ✅ Go-based Coordinator with gRPC"
    echo "   ✅ Python-based Agent with Ollama integration"
    echo "   ✅ Round-robin agent scheduling"
    echo "   ✅ Session token management"
    echo "   ✅ Docker containerization"
    echo "   ✅ Health monitoring"
    echo "   ✅ Test client"
    echo ""
    echo "🏗️ **Architecture:**"
    echo "   • Coordinator: Go gRPC server (port 50051)"
    echo "   • Agent 1: Python + Ollama (port 50052)"
    echo "   • Agent 2: Python + Ollama (port 50053)"
    echo "   • Direct streaming: Client ↔ Agent (bypasses coordinator)"
    echo ""
    echo "🚀 **Quick Start:**"
    echo "   1. cd deploy && ./bootstrap.sh"
    echo "   2. cd ../client && python test_client.py"
    echo ""
    echo "📊 **Key Components:**"
    echo "   • Protocol Buffers: gRPC service definitions"
    echo "   • Coordinator: Agent registry + round-robin scheduler"
    echo "   • Agent: Model manager + stats collector + gRPC server"
    echo "   • Docker: Multi-container deployment"
    echo ""
    echo "🛣️ **Ready for M2:**"
    echo "   • Memory-aware MCDA scheduling"
    echo "   • Circuit breaker fault tolerance"
    echo "   • Complete GGUF quantization support"
    echo "   • JWT authentication"
    echo "   • Prometheus + Grafana monitoring"
}

# Main validation
main() {
    local failed=0
    
    check_structure || failed=1
    echo ""
    
    check_go_build || failed=1
    echo ""
    
    check_python_syntax || failed=1
    echo ""
    
    check_docker_files || failed=1
    echo ""
    
    check_permissions
    echo ""
    
    if [ $failed -eq 0 ]; then
        echo "🎉 TitanCompute M1 validation PASSED!"
        generate_summary
        
        echo ""
        echo "🔥 **Next Steps:**"
        echo "   ./deploy/bootstrap.sh    # Deploy the system"
        echo "   ./deploy/bootstrap.sh help # See all commands"
        echo ""
        return 0
    else
        echo "❌ TitanCompute M1 validation FAILED!"
        echo "   Please fix the issues above and re-run validation."
        return 1
    fi
}

main
