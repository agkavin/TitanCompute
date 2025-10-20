FROM python:3.11-slim

# Install system dependencies and Ollama
RUN apt-get update && apt-get install -y \
    curl \
    && rm -rf /var/lib/apt/lists/* \
    && curl -fsSL https://ollama.com/install.sh | sh

# Create app user
RUN useradd -m -u 1001 -s /bin/bash titan

WORKDIR /app

# Copy Python project files
COPY pyproject.toml uv.lock ./
COPY requirements.txt ./

# Install uv and dependencies
RUN pip install uv && \
    uv pip install --system --requirement requirements.txt

# Copy application code
COPY src/ ./src/
COPY main.py ./

# Copy entrypoint script
COPY entrypoint.sh .
RUN chmod +x entrypoint.sh && chown -R titan:titan /app

USER titan

EXPOSE 50052 11434

HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
  CMD python -c "import grpc; grpc.channel_ready_future(grpc.insecure_channel('localhost:50052')).result(timeout=5)" || exit 1

ENTRYPOINT ["./entrypoint.sh"]
