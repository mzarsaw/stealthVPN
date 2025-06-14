package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/songgao/water"
	"stealthvpn/pkg/protocol"
)

// ClientConfig holds client configuration
type ClientConfig struct {
	ServerURL        string   `json:"server_url"`
	PreSharedKey     string   `json:"pre_shared_key"`
	DNSServers       []string `json:"dns_servers"`
	LocalIP          string   `json:"local_ip"`
	AutoConnect      bool     `json:"auto_connect"`
	ReconnectDelay   int      `json:"reconnect_delay"`
	HealthCheckInterval int   `json:"health_check_interval"`
	FakeDomainName   string   `json:"fake_domain_name"`
}

// VPNClient represents the stealth VPN client
type VPNClient struct {
	config       *ClientConfig
	stealth      *protocol.StealthProtocol
	encryption   *protocol.MultiLayerEncryption
	conn         *websocket.Conn
	tunInterface *water.Interface
	keyExchange  *protocol.KeyExchange
	connected    bool
}

// NewVPNClient creates a new stealth VPN client
func NewVPNClient(config *ClientConfig) (*VPNClient, error) {
	stealth := protocol.NewStealthProtocol()
	
	// Initialize pre-shared key encryption
	encryption, err := protocol.NewMultiLayerEncryption([]byte(config.PreSharedKey))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize encryption: %v", err)
	}
	
	return &VPNClient{
		config:     config,
		stealth:    stealth,
		encryption: encryption,
		connected:  false,
	}, nil
}

// Connect establishes connection to the VPN server
func (c *VPNClient) Connect() error {
	log.Println("Connecting to stealth VPN server...")
	
	// Create TUN interface
	if err := c.createTunInterface(); err != nil {
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

// createTunInterface creates and configures the TUN interface
func (c *VPNClient) createTunInterface() error {
	// Create TUN interface
	config := water.Config{
		DeviceType: water.TUN,
	}
	
	// Platform-specific configuration
	if runtime.GOOS == "windows" {
		config.PlatformSpecificParams = water.PlatformSpecificParams{
			ComponentID:   "tap0901",
			InterfaceName: "StealthVPN",
		}
	}
	
	iface, err := water.New(config)
	if err != nil {
		return err
	}
	
	c.tunInterface = iface
	
	// Configure interface IP
	if err := c.configureTunInterface(); err != nil {
		return err
	}
	
	log.Printf("Created TUN interface: %s", iface.Name())
	return nil
}

// configureTunInterface configures the TUN interface with IP settings
func (c *VPNClient) configureTunInterface() error {
	if runtime.GOOS == "windows" {
		// Windows-specific configuration using netsh
		return c.configureWindowsInterface()
	}
	
	// Linux/Unix configuration would go here
	return nil
}

// configureWindowsInterface configures the interface on Windows
func (c *VPNClient) configureWindowsInterface() error {
	// This would typically use Windows API calls or netsh commands
	// For now, we'll provide instructions to the user
	log.Printf("Please configure the network interface manually:")
	log.Printf("IP Address: %s", c.config.LocalIP)
	log.Printf("Subnet Mask: 255.255.255.0")
	log.Printf("DNS Servers: %v", c.config.DNSServers)
	
	return nil
}

// connectToServer establishes WebSocket connection to server
func (c *VPNClient) connectToServer() error {
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
		TLSClientConfig: tlsConfig,
		HandshakeTimeout: 15 * time.Second,
	}
	
	// Create fake WebSocket upgrade request
	header := make(http.Header)
	header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
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
func (c *VPNClient) performKeyExchange() error {
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
		"type": "key_exchange",
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
func (c *VPNClient) forwardPacketsToServer() {
	buffer := make([]byte, 1500) // Standard MTU
	
	for c.connected {
		// Read packet from TUN interface
		n, err := c.tunInterface.Read(buffer)
		if err != nil {
			log.Printf("Error reading from TUN: %v", err)
			continue
		}
		
		packet := make([]byte, n)
		copy(packet, buffer[:n])
		
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
func (c *VPNClient) forwardPacketsFromServer() {
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
		
		// Write to TUN interface
		if _, err := c.tunInterface.Write(decrypted); err != nil {
			log.Printf("Failed to write to TUN: %v", err)
			continue
		}
	}
}

// healthCheckRoutine periodically checks connection health
func (c *VPNClient) healthCheckRoutine() {
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
func (c *VPNClient) handleDisconnection() {
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
func (c *VPNClient) Disconnect() {
	c.connected = false
	
	if c.conn != nil {
		c.conn.Close()
	}
	
	if c.tunInterface != nil {
		c.tunInterface.Close()
	}
	
	log.Println("Disconnected from VPN server")
}

// GetStats returns connection statistics
func (c *VPNClient) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"connected": c.connected,
		"server_url": c.config.ServerURL,
		"local_ip": c.config.LocalIP,
	}
}

// loadConfig loads client configuration from file
func loadConfig(filename string) (*ClientConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	
	var config ClientConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	
	return &config, nil
}

func main() {
	var (
		configFile = flag.String("config", "client-config.json", "Configuration file path")
		serverURL  = flag.String("server", "", "VPN server URL (overrides config)")
		gui        = flag.Bool("gui", false, "Start with GUI (Windows only)")
	)
	flag.Parse()
	
	// Load configuration
	config, err := loadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	
	// Override server URL if provided
	if *serverURL != "" {
		config.ServerURL = *serverURL
	}
	
	// Create client
	client, err := NewVPNClient(config)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	
	// Start GUI if requested
	if *gui && runtime.GOOS == "windows" {
		log.Println("Starting GUI mode...")
		// TODO: Implement Windows GUI
		log.Println("GUI mode not implemented yet, falling back to CLI")
	}
	
	// Connect to VPN
	if err := client.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	
	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		<-sigChan
		log.Println("Shutting down client...")
		client.Disconnect()
		os.Exit(0)
	}()
	
	// Keep running
	log.Println("VPN client is running. Press Ctrl+C to exit.")
	select {}
} 