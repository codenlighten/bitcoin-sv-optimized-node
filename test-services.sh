#!/bin/bash

# Test script for Metamorph services
set -e

echo "🚀 Starting Metamorph services test..."

# Build and start services
echo "📦 Building and starting services with Docker Compose..."
docker-compose up -d --build

# Wait for services to be ready
echo "⏳ Waiting for services to start..."
sleep 10

# Test Ledger service health
echo "🔍 Testing Ledger service..."
if docker-compose exec ledger sh -c 'echo "Ledger service is running"'; then
    echo "✅ Ledger service is healthy"
else
    echo "❌ Ledger service failed"
    exit 1
fi

# Test Engine service health
echo "🔍 Testing Engine service..."
if docker-compose exec engine sh -c 'echo "Engine service is running"'; then
    echo "✅ Engine service is healthy"
else
    echo "❌ Engine service failed"
    exit 1
fi

# Test Conductor service health
echo "🔍 Testing Conductor service..."
if docker-compose exec conductor sh -c 'echo "Conductor service is running"'; then
    echo "✅ Conductor service is healthy"
else
    echo "❌ Conductor service failed"
    exit 1
fi

echo "🎉 All services are running successfully!"
echo ""
echo "📋 Service endpoints:"
echo "  - Ledger gRPC:  localhost:50051"
echo "  - Engine gRPC:  localhost:50052"
echo "  - gRPC UI:      http://localhost:8080"
echo ""
echo "🔧 To stop services: docker-compose down"
echo "📊 To view logs:     docker-compose logs -f [service-name]"
