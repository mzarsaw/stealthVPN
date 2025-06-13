#!/bin/bash
set -e

echo "🔧 StealthVPN Client Build Script"
echo "================================="

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go 1.21 or later."
    exit 1
fi

# Create build directory
mkdir -p build

echo "📦 Installing dependencies..."
go mod download

# Build Windows client
echo "🪟 Building Windows client..."
cd client/windows
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o ../../build/stealthvpn-windows-amd64.exe .
echo "✅ Windows client built: build/stealthvpn-windows-amd64.exe"

# Build Linux client
echo "🐧 Building Linux client..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ../../build/stealthvpn-linux-amd64 .
echo "✅ Linux client built: build/stealthvpn-linux-amd64"

# Build macOS client
echo "🍎 Building macOS client..."
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o ../../build/stealthvpn-macos-amd64 .
echo "✅ macOS client built: build/stealthvpn-macos-amd64"

cd ../..

# Build Android client (requires gomobile)
echo "📱 Building Android client..."
if command -v gomobile &> /dev/null; then
    cd client/android
    gomobile bind -target=android -o ../../build/stealthvpn.aar .
    echo "✅ Android library built: build/stealthvpn.aar"
    cd ../..
else
    echo "⚠️  gomobile not found. Installing..."
    go install golang.org/x/mobile/cmd/gomobile@latest
    gomobile init
    
    cd client/android
    gomobile bind -target=android -o ../../build/stealthvpn.aar .
    echo "✅ Android library built: build/stealthvpn.aar"
    cd ../..
fi

# Create client configuration templates
echo "⚙️  Creating client configuration templates..."

# Windows client config
cat > build/windows-config.json << EOF
{
    "server_url": "wss://YOUR_SERVER_IP:443/ws",
    "pre_shared_key": "YOUR_PRE_SHARED_KEY_HERE",
    "dns_servers": ["8.8.8.8", "8.8.4.4"],
    "local_ip": "10.8.0.2",
    "auto_connect": true,
    "reconnect_delay": 5,
    "health_check_interval": 30,
    "fake_domain_name": "api.cloudsync-enterprise.com"
}
EOF

# Android client config
cat > build/android-config.json << EOF
{
    "server_url": "wss://YOUR_SERVER_IP:443/ws",
    "pre_shared_key": "YOUR_PRE_SHARED_KEY_HERE",
    "dns_servers": ["8.8.8.8", "8.8.4.4"],
    "local_ip": "10.8.0.3",
    "auto_connect": true,
    "reconnect_delay": 5,
    "health_check_interval": 30,
    "fake_domain_name": "api.cloudsync-enterprise.com"
}
EOF

# Create Windows batch file for easy running
cat > build/run-windows-client.bat << 'EOF'
@echo off
echo Starting StealthVPN Windows Client...
echo ===================================

if not exist "windows-config.json" (
    echo Error: windows-config.json not found
    echo Please edit windows-config.json with your server details
    pause
    exit /b 1
)

echo Configuration found. Starting client...
stealthvpn-windows-amd64.exe -config windows-config.json

pause
EOF

# Create Linux/macOS shell script for easy running
cat > build/run-linux-client.sh << 'EOF'
#!/bin/bash
echo "Starting StealthVPN Linux Client..."
echo "==================================="

if [ ! -f "linux-config.json" ]; then
    echo "Error: linux-config.json not found"
    echo "Please create linux-config.json with your server details"
    exit 1
fi

echo "Configuration found. Starting client..."
sudo ./stealthvpn-linux-amd64 -config linux-config.json
EOF

chmod +x build/run-linux-client.sh

# Create installation instructions
cat > build/INSTALLATION.md << 'EOF'
# StealthVPN Client Installation

## Windows Client

1. Download the Windows client files:
   - `stealthvpn-windows-amd64.exe`
   - `windows-config.json`
   - `run-windows-client.bat`

2. Edit `windows-config.json`:
   - Replace `YOUR_SERVER_IP` with your server's IP address
   - Replace `YOUR_PRE_SHARED_KEY_HERE` with the key from server setup

3. Install TAP-Windows adapter:
   - Download from: https://openvpn.net/community-downloads/
   - Install the TAP-Windows adapter component

4. Run as Administrator:
   - Right-click `run-windows-client.bat`
   - Select "Run as administrator"

## Linux Client

1. Download the Linux client files:
   - `stealthvpn-linux-amd64`
   - `linux-config.json` (copy from windows-config.json)
   - `run-linux-client.sh`

2. Edit configuration and run:
   ```bash
   chmod +x stealthvpn-linux-amd64
   sudo ./run-linux-client.sh
   ```

## Android Client

1. Use the `stealthvpn.aar` file in your Android Studio project
2. See `client/android/README.md` for integration details

## Troubleshooting

- **Connection fails**: Check server IP and port
- **Permission denied**: Run as administrator/root
- **TUN interface error**: Install TAP adapter (Windows) or check permissions (Linux)
- **Certificate errors**: Ensure server certificate is valid

For support, check the logs and server status.
EOF

# Create checksums
echo "🔐 Creating checksums..."
cd build
sha256sum * > checksums.txt
cd ..

echo ""
echo "✅ Build Complete!"
echo "=================="
echo "Built files in ./build/:"
echo "  📁 Windows: stealthvpn-windows-amd64.exe"
echo "  📁 Linux:   stealthvpn-linux-amd64"
echo "  📁 macOS:   stealthvpn-macos-amd64"
echo "  📁 Android: stealthvpn.aar"
echo ""
echo "📋 Configuration templates:"
echo "  📁 windows-config.json"
echo "  📁 android-config.json"
echo ""
echo "🚀 Ready-to-run scripts:"
echo "  📁 run-windows-client.bat"
echo "  📁 run-linux-client.sh"
echo ""
echo "📖 Installation guide: build/INSTALLATION.md"
echo "🔐 Checksums: build/checksums.txt"
echo ""
echo "⚠️  Remember to:"
echo "1. Update server IP in config files"
echo "2. Use the pre-shared key from server setup"
echo "3. Run clients as administrator/root"
EOF 