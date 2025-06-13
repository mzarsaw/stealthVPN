package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"stealthvpn/pkg/protocol"
)

// AndroidVPNClient represents the Android VPN client
type AndroidVPNClient struct {
	config       *ClientConfig
	stealth      *protocol.StealthProtocol
	encryption   *protocol.MultiLayerEncryption
	conn         *websocket.Conn
	keyExchange  *protocol.KeyExchange
	connected    bool
	vpnService   VPNService // Android VPN service interface
}

// VPNService interface for Android VPN service
type VPNService interface {
	CreateTunInterface(ip string, dns []string) error
	WritePacket(data []byte) error
	ReadPacket() ([]byte, error)
	CloseTunInterface() error
	IsConnected() bool
}

// ClientConfig holds Android client configuration
type ClientConfig struct {
	ServerURL           string   `json:"server_url"`
	PreSharedKey        string   `json:"pre_shared_key"`
	DNSServers          []string `json:"dns_servers"`
	LocalIP             string   `json:"local_ip"`
	AutoConnect         bool     `json:"auto_connect"`
	ReconnectDelay      int      `json:"reconnect_delay"`
	HealthCheckInterval int      `json:"health_check_interval"`
	FakeDomainName      string   `json:"fake_domain_name"`
}

// NewAndroidVPNClient creates a new Android VPN client
func NewAndroidVPNClient(configJSON string, vpnService VPNService) (*AndroidVPNClient, error) {
	var config ClientConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %v", err)
	}
	
	stealth := protocol.NewStealthProtocol()
	
	// Initialize pre-shared key encryption
	encryption, err := protocol.NewMultiLayerEncryption([]byte(config.PreSharedKey))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize encryption: %v", err)
	}
	
	return &AndroidVPNClient{
		config:     &config,
		stealth:    stealth,
		encryption: encryption,
		connected:  false,
		vpnService: vpnService,
	}, nil
}

// Connect establishes connection to the VPN server
func (c *AndroidVPNClient) Connect() error {
	log.Println("Android VPN connecting to stealth server...")
	
	// Create TUN interface through Android VPN service
	if err := c.vpnService.CreateTunInterface(c.config.LocalIP, c.config.DNSServers); err != nil {
		return fmt.Errorf("failed to create TUN interface: %v", err)
	}
	
	// Connect to server
	if err := c.connectToServer(); err != nil {
		return fmt.Errorf("failed to connect to server: %v", err)
	}
	
	// Perform key exchange
	if err := c.performKeyExchange(); err != nil {
		return fmt.Errorf("key exchange failed: %v", err)
	}
	
	c.connected = true
	log.Println("Successfully connected to VPN server")
	
	// Start packet forwarding
	go c.forwardPacketsToServer()
	go c.forwardPacketsFromServer()
	
	// Start health check
	if c.config.HealthCheckInterval > 0 {
		go c.healthCheckRoutine()
	}
	
	return nil
}

// connectToServer establishes WebSocket connection to server
func (c *AndroidVPNClient) connectToServer() error {
	// Parse server URL
	u, err := url.Parse(c.config.ServerURL)
	if err != nil {
		return err
	}
	
	// Create TLS config for stealth
	tlsConfig := c.stealth.GetTLSConfig()
	tlsConfig.ServerName = c.config.FakeDomainName
	tlsConfig.InsecureSkipVerify = true // For testing - remove in production
	
	// Create WebSocket dialer
	dialer := websocket.Dialer{
		TLSClientConfig:  tlsConfig,
		HandshakeTimeout: 15 * time.Second,
	}
	
	// Create fake WebSocket upgrade request
	header := make(http.Header)
	header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 10; SM-G973F) AppleWebKit/537.36")
	header.Set("Accept-Language", "en-US,en;q=0.9")
	header.Set("Accept-Encoding", "gzip, deflate, br")
	header.Set("Origin", fmt.Sprintf("https://%s", c.config.FakeDomainName))
	header.Set("Sec-WebSocket-Protocol", "chat")
	
	// Add timing jitter
	c.stealth.AddTimingJitter()
	
	// Connect
	conn, _, err := dialer.Dial(u.String(), header)
	if err != nil {
		return err
	}
	
	c.conn = conn
	log.Printf("Connected to server: %s", u.String())
	return nil
}

// performKeyExchange performs X25519 key exchange with server
func (c *AndroidVPNClient) performKeyExchange() error {
	// Create key exchange
	kx, err := protocol.NewKeyExchange()
	if err != nil {
		return err
	}
	c.keyExchange = kx
	
	// Receive server's public key
	var serverKeyMsg map[string]interface{}
	if err := c.conn.ReadJSON(&serverKeyMsg); err != nil {
		return err
	}
	
	serverPublicKey, ok := serverKeyMsg["public_key"].([]byte)
	if !ok {
		return fmt.Errorf("invalid server public key")
	}
	
	// Send our public key
	clientKeyMsg := map[string]interface{}{
		"type":       "key_exchange",
		"public_key": kx.GetPublicKey(),
	}
	
	if err := c.conn.WriteJSON(clientKeyMsg); err != nil {
		return err
	}
	
	// Compute shared secret
	sharedSecret, err := kx.ComputeSharedSecret(serverPublicKey)
	if err != nil {
		return err
	}
	
	// Create session encryption
	sessionEncryption, err := protocol.NewMultiLayerEncryption(sharedSecret)
	if err != nil {
		return err
	}
	
	c.encryption = sessionEncryption
	log.Println("Key exchange completed successfully")
	return nil
}

// forwardPacketsToServer forwards packets from TUN to server
func (c *AndroidVPNClient) forwardPacketsToServer() {
	for c.connected {
		// Read packet from Android VPN service
		packet, err := c.vpnService.ReadPacket()
		if err != nil {
			log.Printf("Error reading packet: %v", err)
			continue
		}
		
		// Encrypt packet
		encrypted, err := c.encryption.Encrypt(packet)
		if err != nil {
			log.Printf("Failed to encrypt packet: %v", err)
			continue
		}
		
		// Obfuscate packet
		obfuscated, err := c.stealth.ObfuscatePacket(encrypted)
		if err != nil {
			log.Printf("Failed to obfuscate packet: %v", err)
			continue
		}
		
		// Add timing jitter
		c.stealth.AddTimingJitter()
		
		// Send to server
		if err := c.conn.WriteMessage(websocket.BinaryMessage, obfuscated); err != nil {
			log.Printf("Failed to send packet to server: %v", err)
			c.handleDisconnection()
			return
		}
	}
}

// forwardPacketsFromServer forwards packets from server to TUN
func (c *AndroidVPNClient) forwardPacketsFromServer() {
	for c.connected {
		// Read message from server
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("Error reading from server: %v", err)
			c.handleDisconnection()
			return
		}
		
		// Deobfuscate packet
		deobfuscated, err := c.stealth.DeobfuscatePacket(message)
		if err != nil {
			log.Printf("Failed to deobfuscate packet: %v", err)
			continue
		}
		
		// Decrypt packet
		decrypted, err := c.encryption.Decrypt(deobfuscated)
		if err != nil {
			log.Printf("Failed to decrypt packet: %v", err)
			continue
		}
		
		// Write to Android VPN service
		if err := c.vpnService.WritePacket(decrypted); err != nil {
			log.Printf("Failed to write packet: %v", err)
			continue
		}
	}
}

// healthCheckRoutine periodically checks connection health
func (c *AndroidVPNClient) healthCheckRoutine() {
	ticker := time.NewTicker(time.Duration(c.config.HealthCheckInterval) * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		if !c.connected {
			continue
		}
		
		// Send ping to server
		ping := []byte("ping")
		encrypted, err := c.encryption.Encrypt(ping)
		if err != nil {
			continue
		}
		
		obfuscated, err := c.stealth.ObfuscatePacket(encrypted)
		if err != nil {
			continue
		}
		
		if err := c.conn.WriteMessage(websocket.BinaryMessage, obfuscated); err != nil {
			log.Println("Health check failed, attempting reconnection...")
			c.handleDisconnection()
		}
	}
}

// handleDisconnection handles connection loss and reconnection
func (c *AndroidVPNClient) handleDisconnection() {
	c.connected = false
	
	if c.conn != nil {
		c.conn.Close()
	}
	
	if c.config.AutoConnect {
		log.Printf("Reconnecting in %d seconds...", c.config.ReconnectDelay)
		time.Sleep(time.Duration(c.config.ReconnectDelay) * time.Second)
		
		if err := c.Connect(); err != nil {
			log.Printf("Reconnection failed: %v", err)
		}
	}
}

// Disconnect closes the VPN connection
func (c *AndroidVPNClient) Disconnect() {
	c.connected = false
	
	if c.conn != nil {
		c.conn.Close()
	}
	
	if c.vpnService != nil {
		c.vpnService.CloseTunInterface()
	}
	
	log.Println("Disconnected from VPN server")
}

// IsConnected returns connection status
func (c *AndroidVPNClient) IsConnected() bool {
	return c.connected && c.vpnService.IsConnected()
}

// GetStats returns connection statistics
func (c *AndroidVPNClient) GetStats() string {
	stats := map[string]interface{}{
		"connected":  c.connected,
		"server_url": c.config.ServerURL,
		"local_ip":   c.config.LocalIP,
	}
	
	statsJSON, _ := json.Marshal(stats)
	return string(statsJSON)
}

// SetConfig updates client configuration
func (c *AndroidVPNClient) SetConfig(configJSON string) error {
	var config ClientConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return fmt.Errorf("failed to parse config: %v", err)
	}
	
	c.config = &config
	
	// Reinitialize encryption with new key
	encryption, err := protocol.NewMultiLayerEncryption([]byte(config.PreSharedKey))
	if err != nil {
		return fmt.Errorf("failed to initialize encryption: %v", err)
	}
	
	c.encryption = encryption
	return nil
}

// StartVPN starts the VPN connection (called from Android)
func (c *AndroidVPNClient) StartVPN() error {
	return c.Connect()
}

// StopVPN stops the VPN connection (called from Android)
func (c *AndroidVPNClient) StopVPN() {
	c.Disconnect()
}

// GetConnectionStatus returns connection status for Android UI
func (c *AndroidVPNClient) GetConnectionStatus() string {
	status := map[string]interface{}{
		"connected":    c.connected,
		"server_url":   c.config.ServerURL,
		"local_ip":     c.config.LocalIP,
		"fake_domain":  c.config.FakeDomainName,
		"auto_connect": c.config.AutoConnect,
	}
	
	statusJSON, _ := json.Marshal(status)
	return string(statusJSON)
}

// Export for Android (gomobile)
func init() {
	// This will be called when the library is loaded
	log.Println("StealthVPN Android client library loaded")
}

// Example usage for Android integration:
// go:generate gomobile bind -target=android -o stealthvpn.aar .

func main() {
	// This is not used in mobile builds
	log.Println("StealthVPN Android client")
} 