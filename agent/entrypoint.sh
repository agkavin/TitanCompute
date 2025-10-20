#!/bin/bash
# Agent entrypoint script

set -e
echo "🚀 Starting TitanCompute Agent: $AGENT_ID"

# Start Ollama server in background
echo "📦 Starting Ollama server..."
ollama serve &
OLLAMA_PID=$!

# Function to cleanup on exit
cleanup() {
    echo "🧹 Cleaning up..."
    if kill -0 $OLLAMA_PID 2>/dev/null; then
        kill $OLLAMA_PID
    fi
    exit 0
}

# Setup signal handlers
trap cleanup SIGTERM SIGINT

# Wait for Ollama to be ready
echo "⏳ Waiting for Ollama to start..."
for i in {1..30}; do
    if curl -s http://localhost:11434/api/tags > /dev/null 2>&1; then
        echo "✅ Ollama is ready"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "❌ Ollama failed to start"
        exit 1
    fi
    sleep 2
done

# Preload models if configured
if [ -n "$PRELOAD_MODELS" ]; then
    echo "📥 Preloading models: $PRELOAD_MODELS"
    IFS=',' read -ra MODELS <<< "$PRELOAD_MODELS"
    for model in "${MODELS[@]}"; do
        echo "📦 Pulling model: $model"
        ollama pull "$model" || echo "⚠️ Failed to pull $model"
    done
fi

# Start Agent gRPC server
echo "🔌 Starting Agent gRPC server..."
python main.py &
AGENT_PID=$!

# Wait for either process to exit
wait $AGENT_PID
