#!/usr/bin/env python3
"""
TitanCompute Comprehensive Test Suite
Tests all M1 and M2 features end-to-end including MCDA scheduling, circuit breaker, GGUF quantization, and JWT authentication
"""

import asyncio
import logging
import sys
import os
import time
import jwt
import grpc
from grpc import aio

# Add the agent proto path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..', 'agent', 'src'))

from proto import titancompute_pb2 as pb2
from proto import titancompute_pb2_grpc as pb2_grpc


class TitanComputeTestSuite:
    """Comprehensive test suite for TitanCompute system"""

    def __init__(self, coordinator_endpoint: str = "localhost:50051"):
        self.coordinator_endpoint = coordinator_endpoint
        self.logger = logging.getLogger(__name__)

    async def test_basic_inference(self, prompt: str = "What is the capital of France?", model: str = "llama3.1:8b-instruct-q4_k_m"):
        """Test basic inference flow (M1 MVP functionality)"""
        self.logger.info(f"🎯 Testing basic inference: '{prompt[:50]}...'")
        
        try:
            # Step 1: Request routing from coordinator
            async with aio.insecure_channel(self.coordinator_endpoint) as coordinator_channel:
                coordinator_stub = pb2_grpc.CoordinatorServiceStub(coordinator_channel)
                
                # Request inference routing
                inference_request = pb2.InferenceRequest(
                    client_id="test-client",
                    model=model,
                    prompt=prompt,
                    max_tokens=100,
                    temperature=0.7
                )
                
                routing_response = await coordinator_stub.RequestInference(inference_request)
                self.logger.info(f"✅ Got agent: {routing_response.agent_endpoint}")
                self.logger.info(f"🎫 Session token: {routing_response.session_token[:20]}...")
            
            # Step 2: Connect directly to agent for streaming
            agent_endpoint = routing_response.agent_endpoint
            async with aio.insecure_channel(agent_endpoint) as agent_channel:
                agent_stub = pb2_grpc.AgentServiceStub(agent_channel)
                
                # Create streaming request
                stream_request = pb2.StreamRequest(
                    session_token=routing_response.session_token,
                    model=model,
                    prompt=prompt,
                    max_tokens=100,
                    temperature=0.7,
                    stream=True
                )
                
                # Stream inference results
                tokens = []
                async for response in agent_stub.StreamInference(stream_request):
                    if response.content.strip():
                        tokens.append(response.content)
                        print(response.content, end='', flush=True)
                    
                    if response.done:
                        print()  # New line at end
                        break
                
                complete_response = ''.join(tokens)
                self.logger.info(f"✅ Basic inference successful: {len(tokens)} chunks, {len(complete_response)} chars")
                return complete_response

        except Exception as e:
            self.logger.error(f"❌ Basic inference failed: {e}")
            return None

    async def test_mcda_scheduling(self):
        """Test M2 MCDA scheduling by making multiple requests"""
        self.logger.info("🧠 Testing MCDA scheduling with multiple requests...")
        
        try:
            agents_selected = []
            
            async with aio.insecure_channel(self.coordinator_endpoint) as channel:
                stub = pb2_grpc.CoordinatorServiceStub(channel)
                
                # Make multiple inference requests to see MCDA in action
                for i in range(5):
                    request = pb2.InferenceRequest(
                        client_id=f"mcda-test-client-{i}",
                        model="llama3.1:8b-instruct-q4_k_m",
                        prompt=f"Test prompt {i}: What is 2+2?",
                        max_tokens=50
                    )
                    
                    response = await stub.RequestInference(request)
                    agents_selected.append(response.agent_id)
                    self.logger.info(f"Request {i+1}: Agent {response.agent_id} selected")
                    
                    # Small delay between requests
                    await asyncio.sleep(0.1)
                
                # Analyze selection patterns
                unique_agents = set(agents_selected)
                self.logger.info(f"✅ MCDA scheduling test: {len(unique_agents)} unique agents used across 5 requests")
                self.logger.info(f"   Agent distribution: {dict((agent, agents_selected.count(agent)) for agent in unique_agents)}")
                
                return len(unique_agents) > 0
                
        except Exception as e:
            self.logger.error(f"❌ MCDA scheduling test failed: {e}")
            return False

    async def test_system_status(self):
        """Test system status with circuit breaker information"""
        self.logger.info("📊 Testing system status and circuit breaker states...")
        
        try:
            async with aio.insecure_channel(self.coordinator_endpoint) as channel:
                stub = pb2_grpc.CoordinatorServiceStub(channel)
                
                request = pb2.StatusRequest(
                    include_agents=True,
                    include_metrics=True
                )
                
                response = await stub.QuerySystemStatus(request)
                
                self.logger.info(f"📊 System Status:")
                self.logger.info(f"   Total agents: {response.total_agents}")
                self.logger.info(f"   Healthy agents: {response.healthy_agents}")
                
                # Check circuit breaker states
                circuit_states = {}
                for agent in response.agents:
                    status = agent.status
                    circuit_states[status] = circuit_states.get(status, 0) + 1
                    self.logger.info(f"   Agent {agent.agent_id}: {status} "
                                   f"(VRAM: {agent.free_vram_mb}MB, Jobs: {agent.running_jobs})")
                
                self.logger.info(f"✅ Circuit breaker states: {circuit_states}")
                return response.total_agents > 0
                
        except Exception as e:
            self.logger.error(f"❌ System status test failed: {e}")
            return False

    async def test_gguf_quantization(self):
        """Test M2 GGUF quantization by requesting different model variants"""
        self.logger.info("🔧 Testing GGUF quantization support...")
        
        test_models = [
            "llama3.1:8b-instruct-q8_0",      # Premium quality
            "llama3.1:8b-instruct-q4_k_m",    # High quality (default)
            "llama3.1:8b-instruct-q2_k",      # Emergency fallback
        ]
        
        try:
            async with aio.insecure_channel(self.coordinator_endpoint) as channel:
                stub = pb2_grpc.CoordinatorServiceStub(channel)
                
                successful_models = []
                
                for model in test_models:
                    try:
                        request = pb2.InferenceRequest(
                            client_id="gguf-test-client",
                            model=model,
                            prompt="Test quantization support",
                            max_tokens=20
                        )
                        
                        response = await stub.RequestInference(request)
                        successful_models.append(model)
                        self.logger.info(f"✅ Model {model} accepted by agent {response.agent_id}")
                        
                    except grpc.RpcError as e:
                        self.logger.warning(f"⚠️ Model {model} not available: {e.code()}")
                
                self.logger.info(f"✅ GGUF quantization test: {len(successful_models)}/{len(test_models)} variants supported")
                return len(successful_models) > 0
                
        except Exception as e:
            self.logger.error(f"❌ GGUF quantization test failed: {e}")
            return False

    async def test_jwt_authentication(self):
        """Test M2 JWT authentication flow"""
        self.logger.info("🔐 Testing JWT authentication...")
        
        try:
            async with aio.insecure_channel(self.coordinator_endpoint) as channel:
                stub = pb2_grpc.CoordinatorServiceStub(channel)
                
                # Get a JWT token by requesting inference
                request = pb2.InferenceRequest(
                    client_id="jwt-test-client",
                    model="llama3.1:8b-instruct-q4_k_m",
                    prompt="Test JWT authentication",
                    max_tokens=10
                )
                
                response = await stub.RequestInference(request)
                jwt_token = response.session_token
                
                # Validate JWT token format
                try:
                    # Decode without verification to check structure
                    decoded = jwt.decode(jwt_token, options={"verify_signature": False})
                    
                    required_claims = ["agent_id", "client_id", "model", "iat", "exp", "iss"]
                    missing_claims = [claim for claim in required_claims if claim not in decoded]
                    
                    if missing_claims:
                        self.logger.warning(f"⚠️ JWT missing claims: {missing_claims}")
                        return False
                    
                    self.logger.info(f"✅ JWT token structure valid:")
                    self.logger.info(f"   Agent: {decoded.get('agent_id')}")
                    self.logger.info(f"   Client: {decoded.get('client_id')}")
                    self.logger.info(f"   Model: {decoded.get('model')}")
                    self.logger.info(f"   Issuer: {decoded.get('iss')}")
                    self.logger.info(f"   Expires: {decoded.get('exp')}")
                    
                    # Test agent validation by trying to connect
                    agent_endpoint = response.agent_endpoint
                    async with aio.insecure_channel(agent_endpoint) as agent_channel:
                        agent_stub = pb2_grpc.AgentServiceStub(agent_channel)
                        
                        stream_request = pb2.StreamRequest(
                            session_token=jwt_token,
                            model="llama3.1:8b-instruct-q4_k_m",
                            prompt="JWT validation test",
                            max_tokens=5
                        )
                        
                        # Try to get at least one response
                        async for response_chunk in agent_stub.StreamInference(stream_request):
                            self.logger.info("✅ Agent accepted JWT token")
                            break
                    
                    return True
                    
                except jwt.InvalidTokenError as e:
                    self.logger.error(f"❌ Invalid JWT token: {e}")
                    return False
                    
        except Exception as e:
            self.logger.error(f"❌ JWT authentication test failed: {e}")
            return False

    async def run_all_tests(self):
        """Run complete test suite"""
        print("🚀 TitanCompute Comprehensive Test Suite")
        print("=" * 60)
        
        results = {}
        
        # M1 Tests
        print("\n📍 M1 MVP Tests")
        print("-" * 30)
        results['basic_inference'] = await self.test_basic_inference()
        results['system_status'] = await self.test_system_status()
        
        # M2 Tests  
        print("\n📍 M2 Production Tests")
        print("-" * 30)
        results['mcda_scheduling'] = await self.test_mcda_scheduling()
        results['gguf_quantization'] = await self.test_gguf_quantization()
        results['jwt_authentication'] = await self.test_jwt_authentication()
        
        # Summary
        print("\n📊 Test Results Summary")
        print("=" * 30)
        passed = sum(1 for result in results.values() if result)
        total = len(results)
        
        for test_name, result in results.items():
            status = "✅ PASS" if result else "❌ FAIL"
            print(f"{test_name:20} {status}")
        
        print(f"\nOverall: {passed}/{total} tests passed")
        
        if passed == total:
            print("🎉 All tests passed! TitanCompute is fully functional.")
        else:
            print("⚠️ Some tests failed. Check logs for details.")
        
        return passed == total


async def main():
    """Main test function"""
    logging.basicConfig(
        level=logging.INFO,
        format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
    )
    
    test_suite = TitanComputeTestSuite()
    success = await test_suite.run_all_tests()
    
    return 0 if success else 1


if __name__ == "__main__":
    exit_code = asyncio.run(main())
    sys.exit(exit_code)
