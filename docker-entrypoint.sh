#!/bin/bash
set -e

echo "🔐 StealthVPN Server Docker Container"
echo "===================================="

# Function to generate self-signed certificate if not provided
generate_self_signed_cert() {
    if [ ! -f "/etc/stealthvpn/server.key" ] || [ ! -f "/etc/stealthvpn/server.crt" ]; then
        echo "🔒 Generating self-signed certificate..."
        openssl req -x509 -newkey rsa:4096 -keyout /etc/stealthvpn/server.key -out /etc/stealthvpn/server.crt -days 365 -nodes -subj "/CN=${FAKE_DOMAIN:-localhost}"
        chmod 600 /etc/stealthvpn/server.key
        chown stealthvpn:stealthvpn /etc/stealthvpn/server.key /etc/stealthvpn/server.crt
    fi
}

# Function to generate pre-shared key
generate_psk() {
    PSK=$(openssl rand -base64 32 | tr -d "=+/" | cut -c1-32)
    echo "🔑 Generated pre-shared key: $PSK"
    echo ""
    echo "⚠️  IMPORTANT: Save this key for your clients!"
    echo ""
    export GENERATED_PSK=$PSK
}

# Function to setup network
setup_network() {
    echo "🌐 Enabling IP forwarding..."
    echo 1 > /proc/sys/net/ipv4/ip_forward
    
    echo "🔄 Setting up NAT rules..."
    iptables -t nat -A POSTROUTING -s 10.8.0.0/24 -o eth0 -j MASQUERADE
    
    # Allow forwarding
    iptables -A FORWARD -s 10.8.0.0/24 -j ACCEPT
    iptables -A FORWARD -m state --state RELATED,ESTABLISHED -j ACCEPT
}

# Check if running for the first time
if [ ! -f "/etc/stealthvpn/config.json" ]; then
    echo "📋 First run detected, setting up configuration..."
    
    # Copy default config
    cp /etc/stealthvpn/config.default.json /etc/stealthvpn/config.json
    
    # Generate PSK if not provided
    if [ -z "$PRE_SHARED_KEY" ]; then
        generate_psk
        PRE_SHARED_KEY=$GENERATED_PSK
    fi
    
    # Update configuration with environment variables
    if [ -n "$PRE_SHARED_KEY" ]; then
        sed -i "s/your-32-byte-pre-shared-key-here!!/$PRE_SHARED_KEY/" /etc/stealthvpn/config.json
    fi
    
    if [ -n "$FAKE_DOMAIN" ]; then
        sed -i "s/api.cloudsync-enterprise.com/$FAKE_DOMAIN/" /etc/stealthvpn/config.json
    fi
    
    if [ -n "$PORT" ]; then
        sed -i "s/\"port\": 443/\"port\": $PORT/" /etc/stealthvpn/config.json
    fi
    
    chown stealthvpn:stealthvpn /etc/stealthvpn/config.json
fi

# Generate certificates if needed
generate_self_signed_cert

# Setup network if running in privileged mode
if [ "${PRIVILEGED_MODE}" = "true" ]; then
    setup_network
fi

# Show configuration summary
echo ""
echo "📊 Configuration Summary:"
echo "========================"
echo "Port: $(grep -o '"port": [0-9]*' /etc/stealthvpn/config.json | grep -o '[0-9]*')"
echo "Fake Domain: $(grep -o '"fake_domain_name": "[^"]*"' /etc/stealthvpn/config.json | cut -d'"' -f4)"
echo "Certificate: $([ -f /etc/stealthvpn/server.crt ] && echo "✅ Present" || echo "❌ Missing")"
echo ""

# Handle graceful shutdown
trap 'echo "Shutting down..."; kill -TERM $PID' TERM INT

# Drop privileges if running as root
if [ "$(id -u)" = "0" ]; then
    echo "🚀 Starting StealthVPN server as user 'stealthvpn'..."
    cd /etc/stealthvpn
    exec su-exec stealthvpn "$@" &
else
    echo "🚀 Starting StealthVPN server..."
    cd /etc/stealthvpn
    exec "$@" &
fi

PID=$!
wait $PID 