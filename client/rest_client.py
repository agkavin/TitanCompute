#!/usr/bin/env python3
"""
TitanCompute REST API Client Example
Demonstrates how to use the TitanCompute REST API to get tokens and connect to agents
"""

import requests
import json
import time
import grpc
import asyncio
import sys
import os

# Add the agent proto path for gRPC communication with agents
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..', 'agent', 'src'))

from proto import titancompute_pb2 as pb2
from proto import titancompute_pb2_grpc as pb2_grpc


class TitanComputeRESTClient:
    """Example client using TitanCompute REST API"""
    
    def __init__(self, coordinator_url: str = "http://localhost:8080"):
        self.coordinator_url = coordinator_url
        self.api_base = f"{coordinator_url}/api/v1"
        
    def health_check(self):
        """Check if the coordinator is healthy"""
        try:
            response = requests.get(f"{self.api_base}/health", timeout=5)
            response.raise_for_status()
            return response.json()
        except Exception as e:
            print(f"❌ Health check failed: {e}")
            return None
    
    def get_system_status(self, include_agents=True):
        """Get system status"""
        try:
            params = {"include_agents": "true"} if include_agents else {}
            response = requests.get(f"{self.api_base}/status", params=params, timeout=5)
            response.raise_for_status()
            return response.json()
        except Exception as e:
            print(f"❌ System status failed: {e}")
            return None
    
    def request_inference(self, client_id: str, model: str, prompt: str, max_tokens: int = 100, temperature: float = 0.7):
        """Request inference routing - gets JWT token and agent endpoint"""
        try:
            payload = {
                "client_id": client_id,
                "model": model,
                "prompt": prompt,
                "max_tokens": max_tokens,
                "temperature": temperature
            }
            
            response = requests.post(
                f"{self.api_base}/inference/request",
                json=payload,
                headers={"Content-Type": "application/json"},
                timeout=10
            )
            response.raise_for_status()
            return response.json()
        except Exception as e:
            print(f"❌ Inference request failed: {e}")
            return None
    
    async def stream_inference_from_agent(self, agent_endpoint: str, session_token: str, prompt: str, max_tokens: int = 100):
        """Connect directly to agent using JWT token and stream inference"""
        try:
            # Parse agent endpoint
            if ":" not in agent_endpoint:
                print(f"❌ Invalid agent endpoint: {agent_endpoint}")
                return None
                
            # Create gRPC channel to agent
            channel = grpc.aio.insecure_channel(agent_endpoint)
            stub = pb2_grpc.AgentServiceStub(channel)
            
            # Create stream request with JWT token
            request = pb2.StreamRequest(
                session_token=session_token,
                prompt=prompt,
                max_tokens=max_tokens,
                temperature=0.7
            )
            
            print(f"🔗 Connecting to agent: {agent_endpoint}")
            print(f"🎫 Using token: {session_token[:20]}...")
            print(f"💬 Prompt: {prompt}")
            print("📡 Streaming response:")
            
            response_text = ""
            async for response in stub.StreamInference(request):
                if response.content:
                    print(response.content, end="", flush=True)
                    response_text += response.content
                    
                if response.done:
                    print(f"\n✅ Streaming complete")
                    break
            
            await channel.close()
            return response_text
            
        except Exception as e:
            print(f"❌ Agent streaming failed: {e}")
            return None


def main():
    """Example usage of TitanCompute REST API"""
    client = TitanComputeRESTClient()
    
    print("🚀 TitanCompute REST API Client Example")
    print("=" * 50)
    
    # 1. Health check
    print("\n📍 Step 1: Health Check")
    health = client.health_check()
    if health:
        print(f"✅ Service: {health.get('service')}")
        print(f"✅ Status: {health.get('status')}")
    else:
        print("❌ Coordinator not available")
        return
    
    # 2. System status
    print("\n📍 Step 2: System Status")
    status = client.get_system_status()
    if status:
        print(f"✅ Total agents: {status.get('total_agents')}")
        print(f"✅ Healthy agents: {status.get('healthy_agents')}")
        
        if status.get('total_agents', 0) == 0:
            print("⚠️  No agents available. Start agents first to test full flow.")
            return
    else:
        print("❌ Could not get system status")
        return
    
    # 3. Request inference routing
    print("\n📍 Step 3: Request Inference Routing")
    routing = client.request_inference(
        client_id="rest-example-client",
        model="llama3.1:8b-instruct-q4_k_m",
        prompt="What is machine learning in simple terms?",
        max_tokens=150,
        temperature=0.7
    )
    
    if routing:
        print(f"✅ Agent endpoint: {routing.get('agent_endpoint')}")
        print(f"✅ Job ID: {routing.get('job_id')}")
        print(f"✅ Token expires at: {routing.get('expires_at')}")
        print(f"✅ Estimated RTT: {routing.get('estimated_rtt_ms')}ms")
        
        # 4. Connect to agent and stream inference
        print("\n📍 Step 4: Direct Agent Streaming")
        asyncio.run(client.stream_inference_from_agent(
            agent_endpoint=routing.get('agent_endpoint'),
            session_token=routing.get('session_token'),
            prompt="What is machine learning in simple terms?",
            max_tokens=150
        ))
    else:
        print("❌ Could not get inference routing")
    
    print("\n🎉 REST API Client Example Complete!")


if __name__ == "__main__":
    main()
