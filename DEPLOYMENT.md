# StealthVPN Deployment Guide

This guide will walk you through setting up a complete StealthVPN system that can bypass deep packet inspection (DPI) and firewall restrictions.

## 🎯 Overview

StealthVPN disguises VPN traffic as regular HTTPS/WebSocket traffic, making it undetectable by most firewalls and DPI systems. It includes:

- **Stealth Protocol**: Traffic obfuscation and timing randomization
- **Multi-layer Encryption**: ChaCha20-Poly1305 + AES-256-GCM
- **WebSocket over TLS**: Mimics regular web traffic
- **Cross-platform Support**: Windows, Linux, macOS, Android

## 📋 Prerequisites

### Server Requirements
- Linux VPS with root access (Ubuntu 20.04+ recommended)
- Public IP address
- Domain name (optional but recommended)
- 1GB+ RAM, 10GB+ storage

### Client Requirements
- **Windows**: Windows 10+ with TAP-Windows adapter
- **Android**: Android 5.0+ (API 21+)
- **Linux/macOS**: Modern kernel with TUN/TAP support

### Development Requirements
- Go 1.21+
- Git
- OpenSSL (for certificates)

## 🚀 Quick Start

### 1. Server Setup

```bash
# Clone the repository
git clone <your-repo-url>
cd stealthvpn

# Run server setup script (as root)
sudo ./scripts/setup-server.sh
```

The setup script will:
- Install Go and dependencies
- Generate TLS certificates
- Create a random pre-shared key
- Configure firewall and routing
- Start the VPN service

**Important**: Save the pre-shared key output - you'll need it for clients!

### 2. Build Clients

```bash
# Build all clients
./scripts/build-clients.sh
```

This creates:
- `build/stealthvpn-windows-amd64.exe` - Windows client
- `build/stealthvpn-linux-amd64` - Linux client
- `build/stealthvpn-macos-amd64` - macOS client
- `build/stealthvpn.aar` - Android library

### 3. Configure Clients

Edit the configuration files in `build/`:
- Replace `YOUR_SERVER_IP` with your server's IP
- Replace `YOUR_PRE_SHARED_KEY_HERE` with the key from server setup

### 4. Connect

**Windows:**
```cmd
# Run as Administrator
run-windows-client.bat
```

**Linux/macOS:**
```bash
sudo ./run-linux-client.sh
```

**Android:** See `client/android/README.md` for integration guide.

## 🔧 Detailed Setup

### Server Configuration

#### Custom Domain Setup
1. Point your domain to the server IP
2. Get a Let's Encrypt certificate:
```bash
sudo apt install certbot
sudo certbot certonly --standalone -d your-domain.com
```

3. Update `/etc/stealthvpn/config.json`:
```json
{
    "tls_cert_file": "/etc/letsencrypt/live/your-domain.com/fullchain.pem",
    "tls_key_file": "/etc/letsencrypt/live/your-domain.com/privkey.pem",
    "fake_domain_name": "your-domain.com"
}
```

4. Restart the service:
```bash
sudo systemctl restart stealthvpn
```

#### Advanced Configuration

Edit `/etc/stealthvpn/config.json`:

```json
{
    "host": "0.0.0.0",
    "port": 443,
    "tls_cert_file": "/etc/stealthvpn/server.crt",
    "tls_key_file": "/etc/stealthvpn/server.key",
    "pre_shared_key": "your-generated-key",
    "max_clients": 100,
    "tunnel_interface": "tun0",
    "dns_servers": ["8.8.8.8", "1.1.1.1"],
    "allowed_ips": ["0.0.0.0/0"],
    "fake_domain_name": "api.cloudsync-enterprise.com",
    "enable_domain_fronting": true
}
```

### Client Configuration

#### Windows Client

1. Install TAP-Windows adapter from OpenVPN
2. Edit `windows-config.json`:
```json
{
    "server_url": "wss://your-server.com:443/ws",
    "pre_shared_key": "your-key-here",
    "dns_servers": ["8.8.8.8", "8.8.4.4"],
    "local_ip": "10.8.0.2",
    "auto_connect": true,
    "reconnect_delay": 5,
    "health_check_interval": 30,
    "fake_domain_name": "api.cloudsync-enterprise.com"
}
```

3. Run as Administrator:
```cmd
stealthvpn-windows-amd64.exe -config windows-config.json
```

#### Linux Client

1. Ensure TUN/TAP support:
```bash
sudo modprobe tun
```

2. Configure and run:
```bash
sudo ./stealthvpn-linux-amd64 -config linux-config.json
```

#### Android Client

See detailed integration guide in `client/android/README.md`.

## 🛡️ Security Features

### Traffic Obfuscation
- **HTTP Header Mimicry**: Uses realistic browser headers
- **WebSocket Disguise**: Appears as legitimate WebSocket traffic
- **Packet Size Randomization**: Varies packet sizes to avoid patterns
- **Timing Jitter**: Random delays to prevent traffic analysis

### Encryption
- **Perfect Forward Secrecy**: X25519 key exchange
- **Multi-layer Encryption**: ChaCha20-Poly1305 + AES-256-GCM
- **TLS 1.3**: Modern cipher suites for transport security

### Anti-Detection
- **Domain Fronting**: Can use legitimate domains as fronts
- **Port 443**: Uses standard HTTPS port
- **Fake Web Service**: Server responds like a real API
- **No VPN Signatures**: No detectable VPN protocol patterns

## 🔍 Monitoring & Troubleshooting

### Server Monitoring

```bash
# Check service status
sudo systemctl status stealthvpn

# View logs
sudo journalctl -u stealthvpn -f

# Check active connections
sudo netstat -an | grep :443

# Monitor traffic
sudo tcpdump -i any port 443
```

### Client Troubleshooting

#### Windows Issues
- **TAP adapter not found**: Install TAP-Windows adapter
- **Permission denied**: Run as Administrator
- **DNS not working**: Check DNS configuration
- **No internet**: Verify server routing

#### Linux Issues
- **TUN device error**: `sudo modprobe tun`
- **Permission denied**: Run as root
- **Routing issues**: Check default gateway

#### Connection Issues
- **Connection refused**: Check server IP and port
- **TLS errors**: Verify certificate configuration
- **Authentication failed**: Check pre-shared key
- **Timeouts**: Check firewall settings

### Performance Optimization

#### Server Optimization
```bash
# Increase file limits
echo 'fs.file-max = 100000' >> /etc/sysctl.conf

# Optimize network
echo 'net.core.rmem_max = 134217728' >> /etc/sysctl.conf
echo 'net.core.wmem_max = 134217728' >> /etc/sysctl.conf

sudo sysctl -p
```

#### Client Optimization
- Use fastest DNS servers for your region
- Adjust MTU size if experiencing issues
- Enable compression if bandwidth is limited

## 🔒 Production Security

### Server Hardening
1. **Firewall Configuration**:
```bash
sudo ufw default deny incoming
sudo ufw allow 22/tcp  # SSH
sudo ufw allow 443/tcp # HTTPS/VPN
sudo ufw enable
```

2. **Fail2Ban Protection**:
```bash
sudo apt install fail2ban
sudo systemctl enable fail2ban
```

3. **Regular Updates**:
```bash
sudo apt update && sudo apt upgrade
```

### Certificate Management
1. **Automatic Renewal**:
```bash
sudo crontab -e
# Add: 0 3 * * * certbot renew --quiet
```

2. **Certificate Pinning**: Implement in clients for additional security

### Key Management
- Rotate pre-shared keys regularly
- Use different keys for different client groups
- Store keys securely (not in version control)

## 📊 Scaling & High Availability

### Load Balancing
Use multiple servers with different IPs:
```json
{
    "servers": [
        "wss://server1.com:443/ws",
        "wss://server2.com:443/ws",
        "wss://server3.com:443/ws"
    ]
}
```

### Geographic Distribution
Deploy servers in different countries:
- Reduces latency
- Provides redundancy
- Bypasses regional restrictions

### Monitoring Setup
```bash
# Install monitoring tools
sudo apt install htop iotop nethogs

# Set up log rotation
sudo logrotate -f /etc/logrotate.conf
```

## 🌐 Advanced Configurations

### Domain Fronting
Configure CDN fronting for maximum stealth:

1. **CloudFlare Setup**:
   - Point subdomain to your server
   - Use CloudFlare's IP as connection target
   - Set Host header to your domain

2. **Client Configuration**:
```json
{
    "server_url": "wss://cloudflare-ip:443/ws",
    "fake_domain_name": "your-subdomain.cloudflare-domain.com"
}
```

### Multi-Protocol Support
Support multiple disguise protocols:
- WebSocket (current)
- HTTP/2 with long polling
- WebRTC data channels
- DNS over HTTPS tunneling

### Custom Obfuscation
Implement application-specific obfuscation:
- Mimic specific applications (Zoom, Teams, etc.)
- Use legitimate API endpoints
- Implement custom packet structures

## 📱 Mobile Deployment

### iOS Support
To add iOS support:
1. Use gomobile for iOS binding
2. Implement Network Extension
3. Handle iOS-specific VPN APIs

### Cross-Platform Apps
Consider frameworks like:
- Flutter with native plugins
- React Native with native modules
- Xamarin with platform-specific code

## 🚨 Legal Considerations

- **Compliance**: Ensure compliance with local laws
- **Terms of Service**: Don't violate service provider terms
- **Responsible Use**: Intended for privacy protection and bypassing censorship
- **Logging**: Implement no-logs policy
- **Data Protection**: Follow GDPR/privacy regulations

## 🔄 Updates & Maintenance

### Regular Maintenance
- Monitor server performance
- Update dependencies
- Rotate certificates
- Review security logs
- Test client connectivity

### Version Updates
- Maintain backward compatibility
- Implement gradual rollouts
- Provide migration guides
- Test thoroughly before release

## 📞 Support & Community

### Getting Help
- Check logs first (`journalctl -u stealthvpn -f`)
- Review configuration files
- Test with different clients
- Check network connectivity

### Contributing
- Report issues with detailed logs
- Submit feature requests
- Contribute code improvements
- Help with documentation

---

**Remember**: This VPN is designed for legitimate privacy protection and bypassing censorship. Always comply with local laws and regulations when using this software. 