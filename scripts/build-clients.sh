#!/bin/bash
set -e

echo "🔧 StealthVPN Client Build Script"
echo "================================="

# Create build directory if it doesn't exist
mkdir -p build

# Get absolute paths
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
BUILD_DIR="$PROJECT_ROOT/build"
CLIENT_DIR="$PROJECT_ROOT/client"

# Download dependencies for protocol package
echo "📦 Setting up protocol package..."
cd "$PROJECT_ROOT/pkg/protocol"
go mod tidy
cd "$PROJECT_ROOT"

# Build Windows client
echo "🪟 Building Windows client..."
cd "$CLIENT_DIR/windows"
go mod tidy
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o "$BUILD_DIR/stealthvpn-windows-amd64.exe" .
cd "$PROJECT_ROOT"
echo "✅ Windows client built: build/stealthvpn-windows-amd64.exe"

# Build Linux client
echo "🐧 Building Linux client..."
cd "$CLIENT_DIR/linux"
go mod tidy
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o "$BUILD_DIR/stealthvpn-linux-amd64" .
cd "$PROJECT_ROOT"
echo "✅ Linux client built: build/stealthvpn-linux-amd64"

# Build macOS Intel client
echo "🍎 Building macOS Intel client..."
cd "$CLIENT_DIR/macos"
go mod tidy
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o "$BUILD_DIR/stealthvpn-macos-amd64" .
cd "$PROJECT_ROOT"
echo "✅ macOS Intel client built: build/stealthvpn-macos-amd64"

# Build macOS ARM64 (Apple Silicon) client
echo "🍎 Building macOS Apple Silicon client..."
cd "$CLIENT_DIR/macos"
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o "$BUILD_DIR/stealthvpn-macos-arm64" .
cd "$PROJECT_ROOT"
echo "✅ macOS Apple Silicon client built: build/stealthvpn-macos-arm64"

# Build Android client
echo "📱 Building Android client..."
if ! command -v gomobile &> /dev/null; then
    echo "⚠️  gomobile not found. Installing..."
    go install golang.org/x/mobile/cmd/gomobile@latest
    go install golang.org/x/mobile/cmd/gobind@latest
    export PATH=$PATH:$(go env GOPATH)/bin
    echo "🔧 Initializing gomobile..."
    gomobile init
fi

cd "$CLIENT_DIR/android"
go mod tidy
gomobile bind -target=android/arm64 -o "$BUILD_DIR/stealthvpn.aar" .
cd "$PROJECT_ROOT"
echo "✅ Android client built: build/stealthvpn.aar"

# Create client configuration templates
echo "📝 Creating client configuration templates..."
cat > "$BUILD_DIR/config.example.json" << EOL
{
    "server": "example.com:8080",
    "psk": "your-pre-shared-key"
}
EOL

echo ""
echo "✨ All clients built successfully!"
echo "📁 Binaries are in the build/ directory" 