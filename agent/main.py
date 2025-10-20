#!/usr/bin/env python3

import asyncio
import logging
import sys
import signal
import os

# Add src directory to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'src'))

from src.config import AgentConfig
from src.agent_server import TitanAgent


def setup_logging():
    """Setup structured logging"""
    logging.basicConfig(
        level=logging.INFO,
        format='{"timestamp": "%(asctime)s", "level": "%(levelname)s", "logger": "%(name)s", "message": "%(message)s"}',
        datefmt='%Y-%m-%dT%H:%M:%SZ'
    )


async def main():
    """Main agent entry point"""
    setup_logging()
    logger = logging.getLogger(__name__)
    
    # Load configuration
    config = AgentConfig.load()
    logger.info(f"🔧 Starting agent with config: {config.agent_id}")
    
    # Create and start agent
    agent = TitanAgent(config)
    
    # Setup graceful shutdown
    shutdown_event = asyncio.Event()
    
    def signal_handler():
        logger.info("📡 Received shutdown signal")
        shutdown_event.set()
    
    # Register signal handlers
    for sig in [signal.SIGTERM, signal.SIGINT]:
        signal.signal(sig, lambda s, f: signal_handler())
    
    try:
        # Start agent
        await agent.start()
        
        # Wait for shutdown signal
        await shutdown_event.wait()
        
    except Exception as e:
        logger.error(f"❌ Agent failed: {e}")
        sys.exit(1)
    finally:
        # Cleanup
        await agent.stop()
        logger.info("✅ Agent stopped")


if __name__ == "__main__":
    asyncio.run(main())
