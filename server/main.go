package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"stealthvpn/pkg/protocol"
)

// ServerConfig holds server configuration
type ServerConfig struct {
	Host              string `json:"host"`
	Port              int    `json:"port"`
	TLSCertFile       string `json:"tls_cert_file"`
	TLSKeyFile        string `json:"tls_key_file"`
	PreSharedKey      string `json:"pre_shared_key"`
	MaxClients        int    `json:"max_clients"`
	TunnelInterface   string `json:"tunnel_interface"`
	DNSServers        []string `json:"dns_servers"`
	AllowedIPs        []string `json:"allowed_ips"`
	FakeDomainName    string `json:"fake_domain_name"`
	EnableDomainFronting bool `json:"enable_domain_fronting"`
}

// VPNServer represents the stealth VPN server
type VPNServer struct {
	config       *ServerConfig
	stealth      *protocol.StealthProtocol
	encryption   *protocol.MultiLayerEncryption
	clients      map[string]*ClientSession
	upgrader     websocket.Upgrader
	tunInterface *TunnelInterface
}

// ClientSession represents a connected client
type ClientSession struct {
	conn         *websocket.Conn
	clientIP     net.IP
	keyExchange  *protocol.KeyExchange
	encryption   *protocol.MultiLayerEncryption
	lastActivity time.Time
	bytesIn      uint64
	bytesOut     uint64
}

// TunnelInterface manages the TUN interface
type TunnelInterface struct {
	name   string
	subnet *net.IPNet
}

// NewVPNServer creates a new stealth VPN server
func NewVPNServer(config *ServerConfig) (*VPNServer, error) {
	stealth := protocol.NewStealthProtocol()
	
	// Initialize pre-shared key encryption
	encryption, err := protocol.NewMultiLayerEncryption([]byte(config.PreSharedKey))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize encryption: %v", err)
	}
	
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for now
		},
		Subprotocols: []string{"chat", "echo"}, // Fake subprotocols to look legitimate
	}
	
	return &VPNServer{
		config:     config,
		stealth:    stealth,
		encryption: encryption,
		clients:    make(map[string]*ClientSession),
		upgrader:   upgrader,
	}, nil
}

// Start starts the VPN server
func (s *VPNServer) Start() error {
	// Setup HTTP handlers to mimic a real web service
	s.setupFakeWebHandlers()
	
	// Setup WebSocket handler for VPN traffic
	http.HandleFunc("/ws", s.handleWebSocket)
	http.HandleFunc("/api/status", s.handleStatus)
	
	// Create TLS configuration
	tlsConfig := s.stealth.GetTLSConfig()
	tlsConfig.Certificates = make([]tls.Certificate, 1)
	
	cert, err := tls.LoadX509KeyPair(s.config.TLSCertFile, s.config.TLSKeyFile)
	if err != nil {
		return fmt.Errorf("failed to load TLS certificate: %v", err)
	}
	tlsConfig.Certificates[0] = cert
	
	// Create server
	server := &http.Server{
		Addr:      fmt.Sprintf("%s:%d", s.config.Host, s.config.Port),
		TLSConfig: tlsConfig,
		Handler:   nil, // Use default ServeMux
	}
	
	log.Printf("Starting StealthVPN server on %s:%d", s.config.Host, s.config.Port)
	log.Printf("Fake domain: %s", s.config.FakeDomainName)
	
	// Start cleanup routine
	go s.cleanupRoutine()
	
	return server.ListenAndServeTLS("", "")
}

// setupFakeWebHandlers creates fake web endpoints to look like a real service
func (s *VPNServer) setupFakeWebHandlers() {
	// Fake landing page
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Add timing jitter
		s.stealth.AddTimingJitter()
		
		html := `<!DOCTYPE html>
<html>
<head>
    <title>CloudSync API Gateway</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .container { max-width: 800px; margin: 0 auto; }
        .header { border-bottom: 1px solid #eee; padding-bottom: 20px; }
        .api-info { background: #f5f5f5; padding: 20px; margin: 20px 0; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>CloudSync API Gateway</h1>
            <p>Enterprise-grade cloud synchronization services</p>
        </div>
        <div class="api-info">
            <h2>API Status</h2>
            <p>Service: <span style="color: green;">Online</span></p>
            <p>Version: 2.4.1</p>
            <p>Uptime: 99.99%</p>
        </div>
        <p>For API documentation, visit <a href="/docs">/docs</a></p>
    </div>
</body>
</html>`
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Server", "nginx/1.18.0")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(html))
	})
	
	// Fake API endpoints
	http.HandleFunc("/api/v1/sync", func(w http.ResponseWriter, r *http.Request) {
		s.stealth.AddTimingJitter()
		response := map[string]interface{}{
			"status": "success",
			"data":   map[string]string{"message": "Sync completed"},
			"timestamp": time.Now().Unix(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Server", "nginx/1.18.0")
		json.NewEncoder(w).Encode(response)
	})
	
	http.HandleFunc("/docs", func(w http.ResponseWriter, r *http.Request) {
		s.stealth.AddTimingJitter()
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Server", "nginx/1.18.0")
		w.Write([]byte("<h1>API Documentation</h1><p>Documentation coming soon...</p>"))
	})
}

// handleWebSocket handles WebSocket connections (actual VPN traffic)
func (s *VPNServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Verify this looks like a legitimate WebSocket upgrade
	if r.Header.Get("Upgrade") != "websocket" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	
	// Add timing jitter to avoid traffic analysis
	s.stealth.AddTimingJitter()
	
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()
	
	// Perform key exchange
	session, err := s.performKeyExchange(conn, r.RemoteAddr)
	if err != nil {
		log.Printf("Key exchange failed: %v", err)
		return
	}
	
	// Add client to active sessions
	clientID := r.RemoteAddr
	s.clients[clientID] = session
	defer delete(s.clients, clientID)
	
	log.Printf("Client connected: %s", clientID)
	
	// Handle client session
	s.handleClientSession(session)
}

// performKeyExchange performs X25519 key exchange with the client
func (s *VPNServer) performKeyExchange(conn *websocket.Conn, remoteAddr string) (*ClientSession, error) {
	// Create key exchange
	kx, err := protocol.NewKeyExchange()
	if err != nil {
		return nil, err
	}
	
	// Send our public key
	publicKeyMsg := map[string]interface{}{
		"type": "key_exchange",
		"public_key": kx.GetPublicKey(),
	}
	
	if err := conn.WriteJSON(publicKeyMsg); err != nil {
		return nil, err
	}
	
	// Receive client's public key
	var clientKeyMsg map[string]interface{}
	if err := conn.ReadJSON(&clientKeyMsg); err != nil {
		return nil, err
	}
	
	clientPublicKey, ok := clientKeyMsg["public_key"].([]byte)
	if !ok {
		return nil, fmt.Errorf("invalid client public key")
	}
	
	// Compute shared secret
	sharedSecret, err := kx.ComputeSharedSecret(clientPublicKey)
	if err != nil {
		return nil, err
	}
	
	// Create session encryption
	sessionEncryption, err := protocol.NewMultiLayerEncryption(sharedSecret)
	if err != nil {
		return nil, err
	}
	
	// Parse client IP
	host, _, _ := net.SplitHostPort(remoteAddr)
	clientIP := net.ParseIP(host)
	
	return &ClientSession{
		conn:         conn,
		clientIP:     clientIP,
		keyExchange:  kx,
		encryption:   sessionEncryption,
		lastActivity: time.Now(),
	}, nil
}

// handleClientSession handles an active client session
func (s *VPNServer) handleClientSession(session *ClientSession) {
	for {
		// Read message from client
		_, message, err := session.conn.ReadMessage()
		if err != nil {
			log.Printf("Error reading from client: %v", err)
			break
		}
		
		session.lastActivity = time.Now()
		session.bytesIn += uint64(len(message))
		
		// Deobfuscate the packet
		deobfuscated, err := s.stealth.DeobfuscatePacket(message)
		if err != nil {
			log.Printf("Failed to deobfuscate packet: %v", err)
			continue
		}
		
		// Decrypt the packet
		decrypted, err := session.encryption.Decrypt(deobfuscated)
		if err != nil {
			log.Printf("Failed to decrypt packet: %v", err)
			continue
		}
		
		// Process the decrypted VPN packet
		s.processVPNPacket(session, decrypted)
	}
}

// processVPNPacket processes a decrypted VPN packet
func (s *VPNServer) processVPNPacket(session *ClientSession, packet []byte) {
	// TODO: Implement actual packet routing logic
	// This would typically involve:
	// 1. Parsing the IP packet
	// 2. Routing to the appropriate destination
	// 3. Handling return traffic
	
	log.Printf("Processing VPN packet of %d bytes from %s", len(packet), session.clientIP)
	
	// For now, just echo back a response to keep the connection alive
	response := []byte("VPN packet processed")
	
	// Encrypt response
	encrypted, err := session.encryption.Encrypt(response)
	if err != nil {
		log.Printf("Failed to encrypt response: %v", err)
		return
	}
	
	// Obfuscate response
	obfuscated, err := s.stealth.ObfuscatePacket(encrypted)
	if err != nil {
		log.Printf("Failed to obfuscate response: %v", err)
		return
	}
	
	// Send response
	if err := session.conn.WriteMessage(websocket.BinaryMessage, obfuscated); err != nil {
		log.Printf("Failed to send response: %v", err)
		return
	}
	
	session.bytesOut += uint64(len(obfuscated))
}

// handleStatus provides server status (fake endpoint)
func (s *VPNServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.stealth.AddTimingJitter()
	
	status := map[string]interface{}{
		"status": "healthy",
		"version": "2.4.1",
		"uptime": time.Now().Unix(),
		"active_connections": len(s.clients),
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Server", "nginx/1.18.0")
	json.NewEncoder(w).Encode(status)
}

// cleanupRoutine periodically cleans up inactive sessions
func (s *VPNServer) cleanupRoutine() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		now := time.Now()
		for id, session := range s.clients {
			if now.Sub(session.lastActivity) > 5*time.Minute {
				log.Printf("Cleaning up inactive session: %s", id)
				session.conn.Close()
				delete(s.clients, id)
			}
		}
	}
}

// loadConfig loads server configuration from file
func loadConfig(filename string) (*ServerConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	
	var config ServerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	
	return &config, nil
}

func main() {
	var configFile = flag.String("config", "config.json", "Configuration file path")
	flag.Parse()
	
	// Load configuration
	config, err := loadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	
	// Create server
	server, err := NewVPNServer(config)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}
	
	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		<-sigChan
		log.Println("Shutting down server...")
		os.Exit(0)
	}()
	
	// Start server
	if err := server.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
} 