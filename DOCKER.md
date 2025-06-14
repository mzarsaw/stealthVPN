# StealthVPN Docker Deployment Guide

This guide covers deploying StealthVPN server using Docker for easy setup and management.

## 🐳 Quick Start

### 1. Clone the Repository
```bash
git clone <your-repo>
cd stealthvpn
```

### 2. Deploy with Docker
```bash
# Quick start (development mode)
./scripts/docker-deploy.sh start

# Production deployment
export STEALTHVPN_PSK=$(./scripts/docker-deploy.sh generate | grep -A1 "Generated" | tail -1)
./scripts/docker-deploy.sh start --prod
```

The script will:
- Build the Docker image
- Generate a pre-shared key (if not provided)
- Start the VPN server
- Show connection details

## 📦 Docker Images

### Pre-built Image (if available)
```bash
docker pull stealthvpn/server:latest
```

### Build from Source
```bash
docker build -t stealthvpn:latest .
```

## 🚀 Deployment Methods

### Method 1: Docker Compose (Recommended)

#### Development Mode
```bash
docker-compose up -d
```

#### Production Mode
```bash
# Create .env file
cat > .env << EOF
STEALTHVPN_PSK=your-32-character-key
STEALTHVPN_DOMAIN=api.your-domain.com
EOF

# Deploy
docker-compose -f docker-compose.prod.yml up -d
```

### Method 2: Docker Run

#### Basic Deployment
```bash
docker run -d \
  --name stealthvpn \
  --cap-add NET_ADMIN \
  --device /dev/net/tun \
  --sysctl net.ipv4.ip_forward=1 \
  -p 443:443 \
  -v $(pwd)/config:/etc/stealthvpn \
  -v $(pwd)/logs:/var/log/stealthvpn \
  stealthvpn:latest
```

#### Production Deployment
```bash
docker run -d \
  --name stealthvpn \
  --restart always \
  --cap-add NET_ADMIN \
  --cap-add NET_RAW \
  --cap-drop ALL \
  --security-opt no-new-privileges:true \
  --read-only \
  --tmpfs /tmp \
  --tmpfs /var/run \
  --device /dev/net/tun \
  --sysctl net.ipv4.ip_forward=1 \
  --sysctl net.ipv6.conf.all.disable_ipv6=1 \
  -p 443:443/tcp \
  -e PRE_SHARED_KEY="your-32-character-key" \
  -e FAKE_DOMAIN="api.your-domain.com" \
  -v $(pwd)/certs/fullchain.pem:/etc/stealthvpn/server.crt:ro \
  -v $(pwd)/certs/privkey.pem:/etc/stealthvpn/server.key:ro \
  -v stealthvpn-logs:/var/log/stealthvpn \
  --memory="1g" \
  --memory-reservation="256m" \
  --cpus="2" \
  stealthvpn:latest
```

## ⚙️ Configuration

### Environment Variables
- `PRE_SHARED_KEY`: 32-character pre-shared key for clients
- `FAKE_DOMAIN`: Domain name for stealth (default: api.cloudsync-enterprise.com)
- `PORT`: Server port (default: 443)
- `PRIVILEGED_MODE`: Enable privileged mode for full functionality

### Volumes
- `/etc/stealthvpn`: Configuration and certificates
- `/var/log/stealthvpn`: Server logs

### Required Capabilities
- `NET_ADMIN`: For network configuration
- `NET_RAW`: For packet manipulation (production)

## 🔒 SSL/TLS Certificates

### Option 1: Auto-generated (Development)
The container will generate a self-signed certificate automatically.

### Option 2: Let's Encrypt (Production)
```bash
# On host machine
certbot certonly --standalone -d your-domain.com

# Mount in container
-v /etc/letsencrypt/live/your-domain.com/fullchain.pem:/etc/stealthvpn/server.crt:ro
-v /etc/letsencrypt/live/your-domain.com/privkey.pem:/etc/stealthvpn/server.key:ro
```

### Option 3: Custom Certificates
Place your certificates in the `certs/` directory:
- `certs/fullchain.pem`: Certificate chain
- `certs/privkey.pem`: Private key

## 📊 Management Commands

### Using the Deploy Script
```bash
# View logs
./scripts/docker-deploy.sh logs

# Check status
./scripts/docker-deploy.sh status

# Restart server
./scripts/docker-deploy.sh restart

# Stop server
./scripts/docker-deploy.sh stop

# Clean up
./scripts/docker-deploy.sh clean
```

### Using Docker Compose
```bash
# View logs
docker-compose logs -f

# Check status
docker-compose ps

# Restart
docker-compose restart

# Stop
docker-compose down
```

### Using Docker CLI
```bash
# View logs
docker logs -f stealthvpn

# Check health
docker inspect stealthvpn --format='{{.State.Health.Status}}'

# Shell access
docker exec -it stealthvpn sh

# Stats
docker stats stealthvpn
```

## 🔍 Health Checks

The container includes health checks that verify:
- Server is responding
- HTTPS endpoint is accessible
- VPN service is running

Check health status:
```bash
docker inspect stealthvpn --format='{{json .State.Health}}' | jq
```

## 🛡️ Security Considerations

### Production Hardening
1. **Read-only Root Filesystem**: Enabled in production compose
2. **Dropped Capabilities**: Only essential capabilities retained
3. **No New Privileges**: Prevents privilege escalation
4. **Resource Limits**: CPU and memory limits enforced
5. **Network Isolation**: Custom bridge network

### Firewall Rules
Ensure these ports are open:
```bash
# Allow HTTPS
ufw allow 443/tcp

# Allow Docker to manage iptables
ufw allow in on docker0
```

## 🚨 Troubleshooting

### Container Won't Start
```bash
# Check logs
docker logs stealthvpn

# Verify TUN device
docker exec stealthvpn ls -la /dev/net/tun

# Check capabilities
docker exec stealthvpn capsh --print
```

### Network Issues
```bash
# Verify IP forwarding
docker exec stealthvpn sysctl net.ipv4.ip_forward

# Check iptables rules
docker exec stealthvpn iptables -t nat -L

# Test connectivity
docker exec stealthvpn wget -O- https://localhost/api/status
```

### Permission Errors
```bash
# Run with required capabilities
docker run --cap-add NET_ADMIN --device /dev/net/tun ...

# Or use privileged mode (development only)
docker run --privileged ...
```

## 📈 Monitoring

### Prometheus Metrics (Optional)
Add to docker-compose.yml:
```yaml
prometheus:
  image: prom/prometheus
  volumes:
    - ./prometheus.yml:/etc/prometheus/prometheus.yml
  ports:
    - "9090:9090"
```

### Log Aggregation
```yaml
# Add to docker-compose.yml
logging:
  driver: "json-file"
  options:
    max-size: "10m"
    max-file: "3"
    labels: "service=stealthvpn"
```

## 🔄 Updates

### Update Container
```bash
# Pull latest image
docker pull stealthvpn:latest

# Recreate container
docker-compose up -d --force-recreate
```

### Backup Configuration
```bash
# Backup
tar -czf stealthvpn-backup.tar.gz docker/config docker/logs

# Restore
tar -xzf stealthvpn-backup.tar.gz
```

## 🌐 Kubernetes Deployment

For Kubernetes deployment, see `k8s/` directory (if available) or use:
```bash
kubectl create secret generic stealthvpn-psk \
  --from-literal=psk=your-32-character-key

kubectl apply -f k8s/deployment.yaml
```

## 📝 Environment-Specific Configs

### Development
- Self-signed certificates
- Verbose logging
- All capabilities enabled

### Staging
- Let's Encrypt staging certificates
- Moderate logging
- Some security restrictions

### Production
- Valid SSL certificates
- Minimal logging
- Maximum security restrictions
- Resource limits enforced

## 🆘 Support

If you encounter issues:
1. Check container logs: `docker logs stealthvpn`
2. Verify Docker version: `docker --version` (requires 19.03+)
3. Ensure kernel modules: `lsmod | grep tun`
4. Check SELinux/AppArmor policies if applicable

For more help, see the main documentation or file an issue. 