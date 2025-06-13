# StealthVPN Makefile
.PHONY: help build-server build-clients build-all clean test server-setup client-setup install-deps

# Default target
help:
	@echo "StealthVPN Build System"
	@echo "======================="
	@echo ""
	@echo "Available targets:"
	@echo "  build-all       - Build server and all clients"
	@echo "  build-server    - Build server only"
	@echo "  build-clients   - Build all clients"
	@echo "  build-windows   - Build Windows client"
	@echo "  build-linux     - Build Linux client"
	@echo "  build-android   - Build Android library"
	@echo "  install-deps    - Install Go dependencies"
	@echo "  test           - Run tests"
	@echo "  clean          - Clean build artifacts"
	@echo "  server-setup   - Set up server (requires root)"
	@echo "  help           - Show this help"
	@echo ""
	@echo "Quick start:"
	@echo "  make install-deps"
	@echo "  make build-all"
	@echo "  sudo make server-setup  # On server machine"

# Variables
BUILD_DIR := build
GO_VERSION := 1.21
LDFLAGS := -s -w -X main.version=$(shell git describe --tags --always --dirty)

# Ensure build directory exists
$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

# Install dependencies
install-deps:
	@echo "Installing Go dependencies..."
	go mod download
	go mod tidy
	@echo "Dependencies installed!"

# Build server
build-server: $(BUILD_DIR)
	@echo "Building StealthVPN server..."
	cd server && go build -ldflags="$(LDFLAGS)" -o ../$(BUILD_DIR)/stealthvpn-server .
	@echo "Server built: $(BUILD_DIR)/stealthvpn-server"

# Build Windows client
build-windows: $(BUILD_DIR)
	@echo "Building Windows client..."
	cd client/windows && GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o ../../$(BUILD_DIR)/stealthvpn-windows-amd64.exe .
	@echo "Windows client built: $(BUILD_DIR)/stealthvpn-windows-amd64.exe"

# Build Linux client
build-linux: $(BUILD_DIR)
	@echo "Building Linux client..."
	cd client/windows && GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o ../../$(BUILD_DIR)/stealthvpn-linux-amd64 .
	@echo "Linux client built: $(BUILD_DIR)/stealthvpn-linux-amd64"

# Build macOS client
build-macos: $(BUILD_DIR)
	@echo "Building macOS client..."
	cd client/windows && GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o ../../$(BUILD_DIR)/stealthvpn-macos-amd64 .
	@echo "macOS client built: $(BUILD_DIR)/stealthvpn-macos-amd64"

# Build Android library
build-android: $(BUILD_DIR)
	@echo "Building Android library..."
	@if ! command -v gomobile > /dev/null; then \
		echo "Installing gomobile..."; \
		go install golang.org/x/mobile/cmd/gomobile@latest; \
		gomobile init; \
	fi
	cd client/android && gomobile bind -target=android -o ../../$(BUILD_DIR)/stealthvpn.aar .
	@echo "Android library built: $(BUILD_DIR)/stealthvpn.aar"

# Build all clients
build-clients: build-windows build-linux build-macos build-android
	@echo "Creating client configuration templates..."
	@# Windows config
	@echo '{\n    "server_url": "wss://YOUR_SERVER_IP:443/ws",\n    "pre_shared_key": "YOUR_PRE_SHARED_KEY_HERE",\n    "dns_servers": ["8.8.8.8", "8.8.4.4"],\n    "local_ip": "10.8.0.2",\n    "auto_connect": true,\n    "reconnect_delay": 5,\n    "health_check_interval": 30,\n    "fake_domain_name": "api.cloudsync-enterprise.com"\n}' > $(BUILD_DIR)/windows-config.json
	@# Linux config
	@cp $(BUILD_DIR)/windows-config.json $(BUILD_DIR)/linux-config.json
	@sed -i 's/10.8.0.2/10.8.0.3/' $(BUILD_DIR)/linux-config.json 2>/dev/null || sed -i '' 's/10.8.0.2/10.8.0.3/' $(BUILD_DIR)/linux-config.json
	@# Android config
	@cp $(BUILD_DIR)/linux-config.json $(BUILD_DIR)/android-config.json
	@sed -i 's/10.8.0.3/10.8.0.4/' $(BUILD_DIR)/android-config.json 2>/dev/null || sed -i '' 's/10.8.0.3/10.8.0.4/' $(BUILD_DIR)/android-config.json
	@echo "Client configuration templates created!"

# Build everything
build-all: install-deps build-server build-clients
	@echo "Creating run scripts..."
	@# Windows run script
	@echo '@echo off\necho Starting StealthVPN Windows Client...\necho ===================================\n\nif not exist "windows-config.json" (\n    echo Error: windows-config.json not found\n    echo Please edit windows-config.json with your server details\n    pause\n    exit /b 1\n)\n\necho Configuration found. Starting client...\nstealthvpn-windows-amd64.exe -config windows-config.json\n\npause' > $(BUILD_DIR)/run-windows-client.bat
	@# Linux/macOS run script
	@echo '#!/bin/bash\necho "Starting StealthVPN Linux Client..."\necho "==================================="\n\nif [ ! -f "linux-config.json" ]; then\n    echo "Error: linux-config.json not found"\n    echo "Please create linux-config.json with your server details"\n    exit 1\nfi\n\necho "Configuration found. Starting client..."\nsudo ./stealthvpn-linux-amd64 -config linux-config.json' > $(BUILD_DIR)/run-linux-client.sh
	@chmod +x $(BUILD_DIR)/run-linux-client.sh 2>/dev/null || true
	@# Create checksums
	@cd $(BUILD_DIR) && (sha256sum * > checksums.txt 2>/dev/null || shasum -a 256 * > checksums.txt)
	@echo ""
	@echo "✅ Build Complete!"
	@echo "=================="
	@echo "Built files in ./$(BUILD_DIR)/:"
	@echo "  📁 Server:   stealthvpn-server"
	@echo "  📁 Windows:  stealthvpn-windows-amd64.exe"
	@echo "  📁 Linux:    stealthvpn-linux-amd64"
	@echo "  📁 macOS:    stealthvpn-macos-amd64"
	@echo "  📁 Android:  stealthvpn.aar"
	@echo ""
	@echo "Next steps:"
	@echo "1. Deploy server: sudo make server-setup"
	@echo "2. Configure clients with server IP and key"
	@echo "3. Run clients as administrator/root"

# Run tests
test:
	@echo "Running tests..."
	go test ./...
	@echo "Tests completed!"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	go clean
	@echo "Clean completed!"

# Set up server (must run as root)
server-setup:
	@if [ "$$(id -u)" != "0" ]; then \
		echo "Error: Server setup must be run as root"; \
		echo "Run: sudo make server-setup"; \
		exit 1; \
	fi
	@echo "Setting up StealthVPN server..."
	@chmod +x scripts/setup-server.sh
	./scripts/setup-server.sh

# Development targets
dev-server: build-server
	@echo "Starting development server..."
	./$(BUILD_DIR)/stealthvpn-server -config server/config.json

dev-client-windows: build-windows
	@echo "Starting development Windows client..."
	./$(BUILD_DIR)/stealthvpn-windows-amd64.exe -config client/windows/client-config.json

dev-client-linux: build-linux
	@echo "Starting development Linux client..."
	sudo ./$(BUILD_DIR)/stealthvpn-linux-amd64 -config client/windows/client-config.json

# Check system requirements
check-deps:
	@echo "Checking system requirements..."
	@go version || (echo "❌ Go not found. Please install Go $(GO_VERSION)+" && exit 1)
	@echo "✅ Go is installed"
	@git --version > /dev/null || (echo "❌ Git not found" && exit 1)
	@echo "✅ Git is installed"
	@openssl version > /dev/null || (echo "⚠️ OpenSSL not found (needed for certificates)")
	@echo "✅ System requirements check completed"

# Generate certificates for development
dev-certs:
	@echo "Generating development certificates..."
	@mkdir -p certs
	openssl req -x509 -newkey rsa:4096 -keyout certs/server.key -out certs/server.crt -days 365 -nodes -subj "/C=US/ST=State/L=City/O=Organization/CN=localhost"
	@echo "Development certificates generated in ./certs/"

# Package for distribution
package: build-all
	@echo "Creating distribution packages..."
	@mkdir -p dist
	@# Create Windows package
	cd $(BUILD_DIR) && zip -r ../dist/stealthvpn-windows.zip stealthvpn-windows-amd64.exe windows-config.json run-windows-client.bat
	@# Create Linux package  
	cd $(BUILD_DIR) && tar -czf ../dist/stealthvpn-linux.tar.gz stealthvpn-linux-amd64 linux-config.json run-linux-client.sh
	@# Create macOS package
	cd $(BUILD_DIR) && tar -czf ../dist/stealthvpn-macos.tar.gz stealthvpn-macos-amd64 linux-config.json run-linux-client.sh
	@# Create Android package
	cd $(BUILD_DIR) && zip -r ../dist/stealthvpn-android.zip stealthvpn.aar android-config.json
	@# Create server package
	cd $(BUILD_DIR) && tar -czf ../dist/stealthvpn-server.tar.gz stealthvpn-server
	@echo "Distribution packages created in ./dist/"

# Install (for system-wide installation)
install: build-all
	@echo "Installing StealthVPN..."
	@if [ "$$(id -u)" != "0" ]; then \
		echo "Error: Installation requires root privileges"; \
		echo "Run: sudo make install"; \
		exit 1; \
	fi
	cp $(BUILD_DIR)/stealthvpn-server /usr/local/bin/
	cp $(BUILD_DIR)/stealthvpn-linux-amd64 /usr/local/bin/stealthvpn-client
	@echo "StealthVPN installed to /usr/local/bin/"

# Uninstall
uninstall:
	@echo "Uninstalling StealthVPN..."
	@if [ "$$(id -u)" != "0" ]; then \
		echo "Error: Uninstallation requires root privileges"; \
		echo "Run: sudo make uninstall"; \
		exit 1; \
	fi
	rm -f /usr/local/bin/stealthvpn-server
	rm -f /usr/local/bin/stealthvpn-client
	systemctl stop stealthvpn 2>/dev/null || true
	systemctl disable stealthvpn 2>/dev/null || true
	rm -f /etc/systemd/system/stealthvpn.service
	@echo "StealthVPN uninstalled!"

# Show build info
info:
	@echo "StealthVPN Build Information"
	@echo "============================"
	@echo "Go version: $$(go version)"
	@echo "Git commit: $$(git describe --tags --always --dirty)"
	@echo "Build time: $$(date)"
	@echo "Build dir:  $(BUILD_DIR)" 