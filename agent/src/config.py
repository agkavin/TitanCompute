import os
from dataclasses import dataclass
from typing import List


@dataclass
class AgentConfig:
    """Agent configuration loaded from environment variables"""
    agent_id: str
    coordinator_endpoint: str
    public_host: str
    port: int
    ollama_host: str
    max_concurrent_jobs: int
    supported_models: List[str]
    heartbeat_interval: int

    @classmethod
    def load(cls) -> 'AgentConfig':
        """Load configuration from environment variables"""
        return cls(
            agent_id=os.getenv('AGENT_ID', 'agent-1'),
            coordinator_endpoint=os.getenv('COORDINATOR_ENDPOINT', 'localhost:50051'),
            public_host=os.getenv('PUBLIC_HOST', 'localhost'),
            port=int(os.getenv('AGENT_PORT', '50052')),
            ollama_host=os.getenv('OLLAMA_HOST', 'http://localhost:11434'),
            max_concurrent_jobs=int(os.getenv('MAX_CONCURRENT_JOBS', '4')),
            supported_models=os.getenv('SUPPORTED_MODELS', 'llama3.1:8b-instruct-q4_k_m').split(','),
            heartbeat_interval=int(os.getenv('HEARTBEAT_INTERVAL', '10'))
        )
