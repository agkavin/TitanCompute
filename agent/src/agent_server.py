import asyncio
import logging
import grpc
from grpc import aio
import time
from typing import Dict, Set, AsyncIterable
from uuid import uuid4

from .proto import titancompute_pb2 as pb2
from .proto import titancompute_pb2_grpc as pb2_grpc
from .config import AgentConfig
from .model_manager import ModelManager
from .stats_collector import StatsCollector
from .jwt_validator import JWTValidator


class TitanAgent:
    """Main agent class that handles gRPC services and coordination"""

    def __init__(self, config: AgentConfig):
        self.config = config
        self.logger = logging.getLogger(__name__)
        
        # Initialize components
        self.model_manager = ModelManager(config.ollama_host)
        self.stats_collector = StatsCollector()
        self.jwt_validator = JWTValidator()
        
        # Session management
        self.active_sessions: Dict[str, Dict] = {}
        self.total_requests = 0
        
        # gRPC server
        self.grpc_server = None
        self.coordinator_channel = None
        self.coordinator_stub = None

    async def start(self):
        """Start the agent services"""
        self.logger.info(f"🚀 Starting TitanCompute Agent: {self.config.agent_id}")
        
        # Preload models
        await self.model_manager.preload_models(self.config.supported_models)
        
        # Start gRPC server
        await self._start_grpc_server()
        
        # Register with coordinator
        await self._register_with_coordinator()
        
        # Start health reporting
        asyncio.create_task(self._health_reporting_loop())
        
        self.logger.info(f"✅ Agent {self.config.agent_id} ready on port {self.config.port}")

    async def _start_grpc_server(self):
        """Start the gRPC server for direct client connections"""
        self.grpc_server = aio.server()
        
        # Register agent service
        agent_servicer = AgentServicer(self)
        pb2_grpc.add_AgentServiceServicer_to_server(agent_servicer, self.grpc_server)
        
        # Setup listener
        listen_addr = f'[::]:{self.config.port}'
        self.grpc_server.add_insecure_port(listen_addr)
        
        # Start server
        await self.grpc_server.start()
        self.logger.info(f"🔌 Agent gRPC server listening on {listen_addr}")

    async def _register_with_coordinator(self):
        """Register this agent with the coordinator"""
        try:
            self.coordinator_channel = aio.insecure_channel(self.config.coordinator_endpoint)
            self.coordinator_stub = pb2_grpc.CoordinatorServiceStub(self.coordinator_channel)
            
            # Get system stats for registration
            stats = self.stats_collector.collect()
            
            # Create registration request
            registration = pb2.AgentRegistration(
                agent_id=self.config.agent_id,
                endpoint=f"{self.config.public_host}:{self.config.port}",
                total_vram_mb=stats.total_vram_mb,
                total_ram_mb=stats.total_ram_mb,
                max_jobs=self.config.max_concurrent_jobs,
                supported_models=self.config.supported_models,
                capabilities={
                    "gpu_available": str(self.stats_collector.has_gpu),
                    "ollama_host": self.config.ollama_host
                }
            )
            
            # Register with coordinator
            response = await self.coordinator_stub.RegisterAgent(registration)
            self.logger.info(f"📝 Registered with coordinator: {response.status}")
            
            # Get JWT public key from coordinator for token validation
            await self._configure_jwt_validation()
            
        except Exception as e:
            self.logger.error(f"Failed to register with coordinator: {e}")
            raise

    async def _configure_jwt_validation(self):
        """Configure JWT validation with coordinator's public key"""
        try:
            # Get public key from coordinator for JWT validation
            public_key_request = pb2.PublicKeyRequest()
            public_key_response = await self.coordinator_stub.GetPublicKey(public_key_request)
            
            # Configure JWT validator with the public key
            self.jwt_validator.set_public_key(public_key_response.public_key_pem)
            
            self.logger.info(f"🔐 JWT validation configured with {public_key_response.algorithm} "
                           f"from issuer: {public_key_response.issuer}")
            
        except Exception as e:
            self.logger.warning(f"JWT configuration failed, falling back to basic validation: {e}")
            # Continue without JWT validation - will use basic token validation

    async def _health_reporting_loop(self):
        """Continuously report health to coordinator"""
        while True:
            try:
                await self._report_health()
                await asyncio.sleep(self.config.heartbeat_interval)
            except Exception as e:
                self.logger.error(f"Health reporting failed: {e}")
                await asyncio.sleep(5)  # Retry after 5 seconds

    async def _report_health(self):
        """Send health update to coordinator"""
        if not self.coordinator_stub:
            return
            
        stats = self.stats_collector.collect()
        
        # Calculate RTT by measuring coordinator response time
        start_time = time.time()
        
        health_update = pb2.HealthUpdate(
            agent_id=self.config.agent_id,
            free_vram_mb=stats.free_vram_mb,
            free_ram_mb=stats.free_ram_mb,
            running_jobs=len(self.active_sessions),
            queued_jobs=0,  # Simple implementation - no queuing yet
            cpu_percent=stats.cpu_percent,
            rtt_ms=0.0,  # Will be calculated after response
            timestamp=int(time.time() * 1000)
        )
        
        try:
            # Create a simple stream with one message
            async def health_stream():
                yield health_update
            
            async for ack in self.coordinator_stub.ReportHealth(health_stream()):
                # Calculate RTT based on response time
                rtt_ms = (time.time() - start_time) * 1000
                self.logger.debug(f"Health ack: {ack.status}, RTT: {rtt_ms:.1f}ms")
                break  # Simple implementation - just one message
                
        except Exception as e:
            self.logger.warning(f"Health update failed: {e}")

    def validate_session_token(self, token: str) -> bool:
        """Validate session token with JWT support (M2 enhanced)"""
        try:
            # Try JWT validation first if public key is configured
            if self.jwt_validator.public_key:
                claims = self.jwt_validator.validate_token(token, self.config.agent_id)
                if claims:
                    self.logger.debug(f"JWT token validated for client {claims.client_id}")
                    return True
                else:
                    self.logger.warning("JWT token validation failed")
                    return False
            else:
                # Fallback to basic validation when JWT is not configured
                is_valid = len(token) > 10
                if is_valid:
                    self.logger.debug("Token validated using basic method (JWT not configured)")
                else:
                    self.logger.warning("Token validation failed")
                    
                return is_valid
            
        except Exception as e:
            self.logger.error(f"Token validation error: {e}")
            return False

    async def stop(self):
        """Stop the agent services"""
        self.logger.info(f"🛑 Stopping agent {self.config.agent_id}")
        
        if self.grpc_server:
            await self.grpc_server.stop(grace=10)
            
        if self.coordinator_channel:
            await self.coordinator_channel.close()
            
        await self.model_manager.close()


class AgentServicer(pb2_grpc.AgentServiceServicer):
    """gRPC service implementation for direct client streaming"""

    def __init__(self, agent: TitanAgent):
        self.agent = agent
        self.logger = logging.getLogger(__name__)

    async def StreamInference(
        self,
        request: pb2.StreamRequest,
        context: aio.ServicerContext
    ) -> AsyncIterable[pb2.StreamResponse]:
        """Handle direct inference streaming from clients"""
        
        # Validate session token
        if not self.agent.validate_session_token(request.session_token):
            context.set_code(grpc.StatusCode.UNAUTHENTICATED)
            context.set_details("Invalid session token")
            return

        session_id = str(uuid4())
        self.agent.active_sessions[session_id] = {
            "token": request.session_token,
            "model": request.model,
            "started_at": time.time()
        }
        
        try:
            self.logger.info(f"🎯 Starting inference session {session_id}")
            
            # Stream inference from Ollama
            async for chunk in self.agent.model_manager.stream_inference(
                model=request.model,
                prompt=request.prompt,
                **dict(request.options)
            ):
                # Convert Ollama response to gRPC response
                response = pb2.StreamResponse(
                    session_token=request.session_token,
                    content=chunk.get("response", ""),
                    done=chunk.get("done", False),
                    token=chunk.get("response", ""),
                    created_at=int(time.time() * 1000),
                    metadata={
                        "model": request.model,
                        "session_id": session_id
                    }
                )
                
                yield response
                
                # Break if done
                if chunk.get("done", False):
                    break
                    
        except Exception as e:
            self.logger.error(f"Inference failed for session {session_id}: {e}")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(f"Inference failed: {str(e)}")
        finally:
            # Clean up session
            if session_id in self.agent.active_sessions:
                del self.agent.active_sessions[session_id]
            self.agent.total_requests += 1

    async def GetStatus(
        self,
        request: pb2.AgentStatusRequest,
        context: aio.ServicerContext
    ) -> pb2.AgentStatusResponse:
        """Return agent status information with M2 enhancements"""
        
        stats = self.agent.stats_collector.collect()
        
        # Get enhanced system status including quantization info
        system_status = await self.agent.model_manager.get_system_status()
        
        # Build enhanced status information
        status_details = {
            "quantization_support": "enabled",
            "total_models": len(self.agent.model_manager.loaded_models),
            "jwt_validation": "enabled" if self.agent.jwt_validator.public_key else "fallback",
            "memory_tier": system_status.get("quantization", {}).get("system_memory", {}).get("recommended_tier", "unknown"),
            "is_arm": str(system_status.get("system_info", {}).get("is_arm", False))
        }
        
        return pb2.AgentStatusResponse(
            agent_id=self.agent.config.agent_id,
            status="healthy",
            free_vram_mb=stats.free_vram_mb,
            free_ram_mb=stats.free_ram_mb,
            active_sessions=len(self.agent.active_sessions),
            total_requests_processed=self.agent.total_requests,
            model_loaded=",".join(self.agent.model_manager.loaded_models),
            capabilities=status_details
        )
