#!/bin/bash
set -e

echo "🔐 StealthVPN Server Setup Script"
echo "================================="

# Check if running as root
if [[ $EUID -ne 0 ]]; then
   echo "This script must be run as root" 
   exit 1
fi

# Update system packages
echo "📦 Updating system packages..."
apt update && apt upgrade -y

# Install required packages
echo "📦 Installing required packages..."
apt install -y wget curl git build-essential unzip

# Install Go if not present
if ! command -v go &> /dev/null; then
    echo "📦 Installing Go..."
    wget https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
    tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz
    echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile
    export PATH=$PATH:/usr/local/go/bin
    rm go1.21.5.linux-amd64.tar.gz
fi

# Create stealthvpn user
echo "👤 Creating stealthvpn user..."
if ! id "stealthvpn" &>/dev/null; then
    useradd -r -m -s /bin/bash stealthvpn
fi

# Create directories
echo "📁 Creating directories..."
mkdir -p /opt/stealthvpn
mkdir -p /etc/stealthvpn
mkdir -p /var/log/stealthvpn
chown -R stealthvpn:stealthvpn /opt/stealthvpn
chown -R stealthvpn:stealthvpn /var/log/stealthvpn

# Generate self-signed certificate for testing
echo "🔒 Generating TLS certificate..."
openssl req -x509 -newkey rsa:4096 -keyout /etc/stealthvpn/server.key -out /etc/stealthvpn/server.crt -days 365 -nodes -subj "/C=US/ST=State/L=City/O=Organization/CN=api.cloudsync-enterprise.com"
chmod 600 /etc/stealthvpn/server.key
chown stealthvpn:stealthvpn /etc/stealthvpn/server.key /etc/stealthvpn/server.crt

# Generate random pre-shared key
echo "🔑 Generating pre-shared key..."
PSK=$(openssl rand -base64 32 | tr -d "=+/" | cut -c1-32)
echo "Pre-shared key: $PSK"

# Create server configuration
echo "⚙️  Creating server configuration..."
cat > /etc/stealthvpn/config.json << EOF
{
    "host": "0.0.0.0",
    "port": 443,
    "tls_cert_file": "/etc/stealthvpn/server.crt",
    "tls_key_file": "/etc/stealthvpn/server.key",
    "pre_shared_key": "$PSK",
    "max_clients": 100,
    "tunnel_interface": "tun0",
    "dns_servers": ["8.8.8.8", "8.8.4.4", "1.1.1.1"],
    "allowed_ips": ["0.0.0.0/0"],
    "fake_domain_name": "api.cloudsync-enterprise.com",
    "enable_domain_fronting": true
}
EOF

# Copy source code if in development directory
if [ -f "server/main.go" ]; then
    echo "📋 Copying server code..."
    cp -r . /opt/stealthvpn/
    cd /opt/stealthvpn
    chown -R stealthvpn:stealthvpn .
else
    echo "⚠️  Server code not found. Please copy the source code to /opt/stealthvpn/"
fi

# Build server
echo "🔨 Building server..."
cd /opt/stealthvpn
sudo -u stealthvpn go mod tidy
sudo -u stealthvpn go build -o stealthvpn-server server/main.go

# Create systemd service
echo "⚙️  Creating systemd service..."
cat > /etc/systemd/system/stealthvpn.service << EOF
[Unit]
Description=StealthVPN Server
After=network.target

[Service]
Type=simple
User=stealthvpn
Group=stealthvpn
WorkingDirectory=/opt/stealthvpn
ExecStart=/opt/stealthvpn/stealthvpn-server -config /etc/stealthvpn/config.json
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=stealthvpn

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/log/stealthvpn

[Install]
WantedBy=multi-user.target
EOF

# Enable IP forwarding
echo "🌐 Enabling IP forwarding..."
echo 'net.ipv4.ip_forward=1' >> /etc/sysctl.conf
sysctl -p

# Configure firewall
echo "🛡️  Configuring firewall..."
ufw allow 443/tcp
ufw allow 80/tcp
ufw --force enable

# Set up NAT rules for VPN traffic
echo "🔀 Setting up NAT rules..."
iptables -t nat -A POSTROUTING -s 10.8.0.0/24 -o eth0 -j MASQUERADE
iptables -A FORWARD -s 10.8.0.0/24 -j ACCEPT
iptables -A FORWARD -d 10.8.0.0/24 -j ACCEPT

# Save iptables rules
iptables-save > /etc/iptables/rules.v4

# Start and enable service
echo "🚀 Starting StealthVPN service..."
systemctl daemon-reload
systemctl enable stealthvpn
systemctl start stealthvpn

# Show status
echo ""
echo "✅ StealthVPN Server Setup Complete!"
echo "=================================="
echo "Server Status: $(systemctl is-active stealthvpn)"
echo "Configuration: /etc/stealthvpn/config.json"
echo "Logs: journalctl -u stealthvpn -f"
echo ""
echo "🔑 IMPORTANT: Save this pre-shared key for clients:"
echo "Pre-shared key: $PSK"
echo ""
echo "🌐 Server URL for clients:"
echo "wss://$(curl -s ifconfig.me):443/ws"
echo ""
echo "🔒 To use a real domain and certificate:"
echo "1. Point your domain to this server's IP"
echo "2. Get a Let's Encrypt certificate"
echo "3. Update the certificate paths in config.json"
echo ""
echo "📊 Check server status: systemctl status stealthvpn"
echo "📋 View logs: journalctl -u stealthvpn -f" 