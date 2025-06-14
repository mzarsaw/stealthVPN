# Multi-stage build for StealthVPN Server
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git build-base

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the server
RUN cd server && go build -ldflags="-w -s" -o stealthvpn-server .

# Final stage - minimal Alpine image
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    iptables \
    iproute2 \
    openssl \
    bash \
    su-exec \
    wget

# Create non-root user
RUN addgroup -g 1000 stealthvpn && \
    adduser -u 1000 -G stealthvpn -s /bin/sh -D stealthvpn

# Create necessary directories
RUN mkdir -p /etc/stealthvpn /var/log/stealthvpn && \
    chown -R stealthvpn:stealthvpn /etc/stealthvpn /var/log/stealthvpn

# Copy binary from builder
COPY --from=builder /build/server/stealthvpn-server /usr/local/bin/
RUN chmod +x /usr/local/bin/stealthvpn-server

# Copy entrypoint script
COPY docker-entrypoint.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

# Set working directory
WORKDIR /etc/stealthvpn

# Expose port
EXPOSE 443

# Set entrypoint
ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["stealthvpn-server", "--config", "config.json"]

# Volume for configuration and certificates
VOLUME ["/etc/stealthvpn", "/var/log/stealthvpn"]

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider https://localhost/api/status || exit 1 