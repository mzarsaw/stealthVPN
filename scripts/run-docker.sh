#!/bin/bash
# Quick Docker run script for StealthVPN

echo "🐳 Starting StealthVPN Server in Docker..."

# Check if docker is installed
if ! command -v docker &> /dev/null; then
    echo "❌ Docker is not installed. Please install Docker first."
    exit 1
fi

# Generate PSK if not set
if [ -z "$STEALTHVPN_PSK" ]; then
    echo "🔑 Generating pre-shared key..."
    export STEALTHVPN_PSK=$(openssl rand -base64 32 | tr -d "=+/" | cut -c1-32)
    echo "Pre-shared key: $STEALTHVPN_PSK"
    echo ""
    echo "⚠️  Save this key for your clients!"
    echo ""
fi

# Build and run
echo "🔨 Building Docker image..."
docker build -t stealthvpn:latest .

echo "🚀 Starting container..."
docker-compose up -d

echo ""
echo "✅ StealthVPN is running!"
echo "===================="
echo "Server URL: wss://$(curl -s ifconfig.me 2>/dev/null || echo "YOUR_SERVER_IP"):443/ws"
echo "Logs: docker-compose logs -f"
echo "Stop: docker-compose down" 