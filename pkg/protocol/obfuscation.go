package protocol

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"
)

// StealthProtocol handles traffic obfuscation to bypass DPI
type StealthProtocol struct {
	userAgents    []string
	hostHeaders   []string
	fakeDomains   []string
	tlsConfig     *tls.Config
	minPadding    int
	maxPadding    int
}

// NewStealthProtocol creates a new stealth protocol instance
func NewStealthProtocol() *StealthProtocol {
	return &StealthProtocol{
		userAgents: []string{
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		},
		hostHeaders: []string{
			"cloudflare.com",
			"amazonaws.com", 
			"googleapis.com",
			"microsoft.com",
			"apple.com",
		},
		fakeDomains: []string{
			"api.example.com",
			"cdn.website.com",
			"static.service.com",
			"assets.platform.com",
		},
		tlsConfig: &tls.Config{
			MinVersion:               tls.VersionTLS10,
			MaxVersion:               tls.VersionTLS13,
			CurvePreferences:         []tls.CurveID{tls.X25519, tls.CurveP256, tls.CurveP384, tls.CurveP521},
			PreferServerCipherSuites: true,
			CipherSuites: []uint16{
				tls.TLS_AES_128_GCM_SHA256,
				tls.TLS_AES_256_GCM_SHA384,
				tls.TLS_CHACHA20_POLY1305_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
			},
			SessionTicketsDisabled: false,
			ClientSessionCache:     tls.NewLRUClientSessionCache(128),
		},
		minPadding: 16,
		maxPadding: 1024,
	}
}

// ObfuscatePacket disguises VPN data as regular HTTPS traffic
func (sp *StealthProtocol) ObfuscatePacket(data []byte) ([]byte, error) {
	// Add random padding to vary packet sizes
	paddingSize := sp.randomInt(sp.minPadding, sp.maxPadding)
	padding := make([]byte, paddingSize)
	rand.Read(padding)
	
	// Create fake HTTP-like header
	header := sp.createFakeHTTPHeader()
	
	// Encode length and add magic bytes to look like WebSocket frame
	lengthBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBytes, uint32(len(data)))
	
	// WebSocket-like frame structure with obfuscation
	var buffer bytes.Buffer
	buffer.Write([]byte(header))
	buffer.Write([]byte("\r\n\r\n"))
	
	// Add fake WebSocket handshake response
	buffer.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
	buffer.WriteString("Upgrade: websocket\r\n")
	buffer.WriteString("Connection: Upgrade\r\n")
	buffer.WriteString(fmt.Sprintf("Sec-WebSocket-Accept: %s\r\n", sp.generateFakeKey()))
	buffer.WriteString("\r\n")
	
	// Add obfuscated payload
	buffer.Write(lengthBytes)
	buffer.Write(data)
	buffer.Write(padding)
	
	return buffer.Bytes(), nil
}

// DeobfuscatePacket extracts original data from obfuscated packet
func (sp *StealthProtocol) DeobfuscatePacket(obfuscated []byte) ([]byte, error) {
	// Find the end of HTTP headers
	headerEnd := bytes.Index(obfuscated, []byte("\r\n\r\n"))
	if headerEnd == -1 {
		return nil, fmt.Errorf("invalid packet format")
	}
	
	// Skip HTTP headers and WebSocket handshake
	payload := obfuscated[headerEnd+4:]
	
	// Find actual WebSocket upgrade response end
	wsEnd := bytes.Index(payload, []byte("\r\n\r\n"))
	if wsEnd == -1 {
		return nil, fmt.Errorf("invalid WebSocket format")
	}
	
	payload = payload[wsEnd+4:]
	
	// Extract length
	if len(payload) < 4 {
		return nil, fmt.Errorf("packet too short")
	}
	
	length := binary.BigEndian.Uint32(payload[:4])
	payload = payload[4:]
	
	if len(payload) < int(length) {
		return nil, fmt.Errorf("incomplete packet")
	}
	
	return payload[:length], nil
}

// createFakeHTTPHeader generates realistic HTTP headers
func (sp *StealthProtocol) createFakeHTTPHeader() string {
	userAgent := sp.userAgents[sp.randomInt(0, len(sp.userAgents)-1)]
	host := sp.hostHeaders[sp.randomInt(0, len(sp.hostHeaders)-1)]
	
	headers := []string{
		"GET /api/v1/data HTTP/1.1",
		fmt.Sprintf("Host: %s", host),
		fmt.Sprintf("User-Agent: %s", userAgent),
		"Accept: text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
		"Accept-Language: en-US,en;q=0.5",
		"Accept-Encoding: gzip, deflate, br",
		"DNT: 1",
		"Connection: keep-alive",
		"Upgrade-Insecure-Requests: 1",
		"Pragma: no-cache",
		"Cache-Control: no-cache",
	}
	
	return strings.Join(headers, "\r\n")
}

// generateFakeKey creates a fake WebSocket accept key
func (sp *StealthProtocol) generateFakeKey() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	key := make([]byte, 24)
	for i := range key {
		key[i] = charset[sp.randomInt(0, len(charset)-1)]
	}
	return string(key) + "="
}

// randomInt generates a random integer between min and max (inclusive)
func (sp *StealthProtocol) randomInt(min, max int) int {
	if max <= min {
		return min
	}
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(max-min+1)))
	return min + int(n.Int64())
}

// AddTimingJitter adds random delays to avoid traffic analysis
func (sp *StealthProtocol) AddTimingJitter() {
	jitter := time.Duration(sp.randomInt(10, 100)) * time.Millisecond
	time.Sleep(jitter)
}

// GetTLSConfig returns optimized TLS configuration for stealth
func (sp *StealthProtocol) GetTLSConfig() *tls.Config {
	return sp.tlsConfig
}

// CreateWebSocketUpgradeRequest creates a legitimate-looking WebSocket upgrade request
func (sp *StealthProtocol) CreateWebSocketUpgradeRequest(host string) *http.Request {
	req := &http.Request{
		Method: "GET",
		URL:    nil, // Will be set by caller
		Proto:  "HTTP/1.1",
		Header: make(http.Header),
		Host:   host,
	}
	
	req.Header.Set("User-Agent", sp.userAgents[sp.randomInt(0, len(sp.userAgents)-1)])
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", sp.generateFakeKey())
	req.Header.Set("Origin", fmt.Sprintf("https://%s", host))
	
	return req
} 