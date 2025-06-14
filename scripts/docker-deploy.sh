#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}🐳 StealthVPN Docker Deployment Script${NC}"
echo "======================================"

# Function to display usage
usage() {
    echo "Usage: $0 [command] [options]"
    echo ""
    echo "Commands:"
    echo "  build       Build the Docker image"
    echo "  start       Start the VPN server"
    echo "  stop        Stop the VPN server"
    echo "  restart     Restart the VPN server"
    echo "  logs        Show server logs"
    echo "  status      Show server status"
    echo "  generate    Generate new pre-shared key"
    echo "  clean       Clean up containers and images"
    echo ""
    echo "Options:"
    echo "  --prod      Use production configuration"
    echo "  --dev       Use development configuration (default)"
    echo ""
    exit 1
}

# Check if Docker is installed
check_docker() {
    if ! command -v docker &> /dev/null; then
        echo -e "${RED}❌ Docker is not installed${NC}"
        echo "Please install Docker first: https://docs.docker.com/get-docker/"
        exit 1
    fi
    
    if ! command -v docker-compose &> /dev/null; then
        echo -e "${RED}❌ Docker Compose is not installed${NC}"
        echo "Please install Docker Compose first: https://docs.docker.com/compose/install/"
        exit 1
    fi
}

# Generate pre-shared key
generate_psk() {
    PSK=$(openssl rand -base64 32 | tr -d "=+/" | cut -c1-32)
    echo -e "${GREEN}🔑 Generated Pre-Shared Key:${NC}"
    echo "$PSK"
    echo ""
    echo "Add this to your .env file:"
    echo "STEALTHVPN_PSK=$PSK"
}

# Build Docker image
build_image() {
    echo -e "${YELLOW}🔨 Building StealthVPN Docker image...${NC}"
    docker build -t stealthvpn:latest .
    echo -e "${GREEN}✅ Image built successfully${NC}"
}

# Start server
start_server() {
    local compose_file="docker-compose.yml"
    if [ "$1" == "--prod" ]; then
        compose_file="docker-compose.prod.yml"
        
        # Check for required environment variables in production
        if [ -z "$STEALTHVPN_PSK" ]; then
            echo -e "${RED}❌ Error: STEALTHVPN_PSK environment variable not set${NC}"
            echo "Generate one with: $0 generate"
            echo "Then export it: export STEALTHVPN_PSK=your-key-here"
            exit 1
        fi
    fi
    
    # Create necessary directories
    mkdir -p docker/config docker/logs certs config
    
    echo -e "${YELLOW}🚀 Starting StealthVPN server...${NC}"
    docker-compose -f "$compose_file" up -d
    
    # Wait for container to start
    echo -n "Waiting for server to start..."
    sleep 5
    
    # Check if running
    if docker-compose -f "$compose_file" ps | grep -q "Up"; then
        echo -e " ${GREEN}✅ Started${NC}"
        
        # Show generated PSK if first run
        if docker-compose -f "$compose_file" logs | grep -q "Generated pre-shared key:"; then
            echo ""
            echo -e "${YELLOW}📋 First-time setup detected!${NC}"
            docker-compose -f "$compose_file" logs | grep "Generated pre-shared key:" -A 2
        fi
        
        # Show connection info
        echo ""
        echo -e "${GREEN}📊 Server Information:${NC}"
        echo "===================="
        if [ -f /.dockerenv ]; then
            # Running inside Docker
            SERVER_IP=$(hostname -I | awk '{print $1}')
        else
            # Running on host
            SERVER_IP=$(curl -s ifconfig.me 2>/dev/null || echo "YOUR_SERVER_IP")
        fi
        echo "Server URL: wss://$SERVER_IP:443/ws"
        echo "Status URL: https://$SERVER_IP/api/status"
        echo ""
        echo "View logs: $0 logs"
    else
        echo -e " ${RED}❌ Failed to start${NC}"
        echo "Check logs: docker-compose -f $compose_file logs"
        exit 1
    fi
}

# Stop server
stop_server() {
    local compose_file="docker-compose.yml"
    [ "$1" == "--prod" ] && compose_file="docker-compose.prod.yml"
    
    echo -e "${YELLOW}🛑 Stopping StealthVPN server...${NC}"
    docker-compose -f "$compose_file" down
    echo -e "${GREEN}✅ Server stopped${NC}"
}

# Restart server
restart_server() {
    stop_server "$1"
    sleep 2
    start_server "$1"
}

# Show logs
show_logs() {
    local compose_file="docker-compose.yml"
    [ "$1" == "--prod" ] && compose_file="docker-compose.prod.yml"
    
    docker-compose -f "$compose_file" logs -f --tail=100
}

# Show status
show_status() {
    local compose_file="docker-compose.yml"
    [ "$1" == "--prod" ] && compose_file="docker-compose.prod.yml"
    
    echo -e "${GREEN}📊 StealthVPN Server Status${NC}"
    echo "=========================="
    
    # Container status
    echo -e "\n${YELLOW}Container Status:${NC}"
    docker-compose -f "$compose_file" ps
    
    # Health check
    echo -e "\n${YELLOW}Health Check:${NC}"
    container_id=$(docker-compose -f "$compose_file" ps -q stealthvpn 2>/dev/null)
    if [ -n "$container_id" ]; then
        health=$(docker inspect --format='{{.State.Health.Status}}' "$container_id" 2>/dev/null || echo "unknown")
        echo "Health: $health"
        
        # Try to access status endpoint
        if command -v curl &> /dev/null; then
            echo -e "\n${YELLOW}API Status:${NC}"
            curl -sk https://localhost/api/status 2>/dev/null | jq . 2>/dev/null || echo "Unable to connect to API"
        fi
    else
        echo "Container not running"
    fi
    
    # Resource usage
    echo -e "\n${YELLOW}Resource Usage:${NC}"
    docker stats --no-stream --format "table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}" stealthvpn-server 2>/dev/null || true
}

# Clean up
clean_up() {
    echo -e "${YELLOW}🧹 Cleaning up Docker resources...${NC}"
    
    # Stop containers
    docker-compose -f docker-compose.yml down 2>/dev/null || true
    docker-compose -f docker-compose.prod.yml down 2>/dev/null || true
    
    # Remove image
    docker rmi stealthvpn:latest 2>/dev/null || true
    
    # Clean volumes (with confirmation)
    read -p "Remove persistent volumes? (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        docker volume rm stealthvpn_stealthvpn-logs 2>/dev/null || true
        rm -rf docker/config docker/logs
    fi
    
    echo -e "${GREEN}✅ Cleanup complete${NC}"
}

# Main script logic
check_docker

case "$1" in
    build)
        build_image
        ;;
    start)
        build_image
        start_server "$2"
        ;;
    stop)
        stop_server "$2"
        ;;
    restart)
        restart_server "$2"
        ;;
    logs)
        show_logs "$2"
        ;;
    status)
        show_status "$2"
        ;;
    generate)
        generate_psk
        ;;
    clean)
        clean_up
        ;;
    *)
        usage
        ;;
esac 