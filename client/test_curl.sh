#!/bin/bash
# TitanCompute REST API Testing Script
# Tests the built-in REST API endpoints in the coordinator

API_BASE="http://localhost:8080"

echo "🚀 TitanCompute REST API Tests"
echo "=============================="
echo "Testing coordinator REST API at: $API_BASE"
echo

# Test 1: Health check
echo
echo "📍 Test 1: Health Check"
echo "curl GET $API_BASE/api/v1/health"
curl -s -X GET "$API_BASE/api/v1/health" | jq '.' || echo "❌ Health check failed"

# Test 2: System Status
echo
echo "📍 Test 2: System Status"
echo "curl GET $API_BASE/api/v1/status"
curl -s -X GET "$API_BASE/api/v1/status" | jq '.' || echo "❌ Status check failed"

# Test 3: Inference Request (Get Token + Agent Info)
echo
echo "📍 Test 3: Inference Request"
echo "curl POST $API_BASE/api/v1/inference/request"
curl -s -X POST "$API_BASE/api/v1/inference/request" \
  -H "Content-Type: application/json" \
  -d '{
    "client_id": "curl-test-client",
    "model": "llama3.1:8b-instruct-q4_k_m",
    "prompt": "What is artificial intelligence?",
    "max_tokens": 50,
    "temperature": 0.7
  }' | jq '.' || echo "❌ Inference request failed"

# Test 4: System Status with Agent Details
echo
echo "📍 Test 4: System Status with Agent Details"
echo "curl GET $API_BASE/api/v1/status?include_agents=true"
curl -s -X GET "$API_BASE/api/v1/status?include_agents=true" | jq '.' || echo "❌ Detailed status failed"

# Test 5: Invalid Request (Testing Error Handling)
echo
echo "📍 Test 5: Invalid Request (Testing Error Handling)"
echo "curl POST $API_BASE/api/v1/inference/request (missing fields)"
curl -s -X POST "$API_BASE/api/v1/inference/request" \
  -H "Content-Type: application/json" \
  -d '{"client_id": "test"}' | jq '.' || echo "❌ Error handling test failed"

echo
echo "✅ curl tests complete!"
echo
echo "💡 The coordinator serves REST API on port 8080"
echo "   No additional setup needed!"
echo
echo "🔧 Python REST client testing:"
echo "   python rest_client.py"
