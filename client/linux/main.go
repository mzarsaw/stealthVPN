package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/gorilla/websocket"
	"github.com/songgao/water"
	"stealthvpn/pkg/protocol"
)

type Client struct {
	serverURL    string
	presharedKey string
	tunInterface *water.Interface
	wsConn       *websocket.Conn
}

func NewClient(serverURL, presharedKey string) *Client {
	return &Client{
		serverURL:    serverURL,
		presharedKey: presharedKey,
	}
}

func (c *Client) configureTunInterface() error {
	name := c.tunInterface.Name()
	
	commands := [][]string{
		{"ip", "addr", "add", "10.8.0.2/24", "dev", name},
		{"ip", "link", "set", name, "up"},
		{"ip", "route", "add", "0.0.0.0/1", "dev", name},
		{"ip", "route", "add", "128.0.0.0/1", "dev", name},
	}

	for _, cmd := range commands {
		if err := exec.Command(cmd[0], cmd[1:]...).Run(); err != nil {
			return fmt.Errorf("failed to run %v: %v", cmd, err)
		}
	}

	return nil
}

func (c *Client) Start() error {
	config := water.Config{
		DeviceType: water.TUN,
	}

	iface, err := water.New(config)
	if err != nil {
		return err
	}

	c.tunInterface = iface

	if err := c.configureTunInterface(); err != nil {
		return err
	}

	log.Printf("Created TUN interface: %s", iface.Name())

	u := url.URL{Scheme: "ws", Host: c.serverURL, Path: "/vpn"}
	headers := http.Header{
		"X-PSK": []string{c.presharedKey},
	}

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), headers)
	if err != nil {
		return err
	}
	c.wsConn = conn

	go c.tunToWs()
	go c.wsToTun()

	return nil
}

func (c *Client) tunToWs() {
	packet := make([]byte, 2048)
	for {
		n, err := c.tunInterface.Read(packet)
		if err != nil {
			log.Printf("Error reading from TUN: %v", err)
			continue
		}

		msg := protocol.Message{
			Type: protocol.PacketType,
			Data: packet[:n],
		}

		data, err := json.Marshal(msg)
		if err != nil {
			log.Printf("Error marshaling packet: %v", err)
			continue
		}

		if err := c.wsConn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("Error writing to websocket: %v", err)
			return
		}
	}
}

func (c *Client) wsToTun() {
	for {
		_, data, err := c.wsConn.ReadMessage()
		if err != nil {
			log.Printf("Error reading from websocket: %v", err)
			return
		}

		var msg protocol.Message
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("Error unmarshaling message: %v", err)
			continue
		}

		if msg.Type != protocol.PacketType {
			continue
		}

		if _, err := c.tunInterface.Write(msg.Data); err != nil {
			log.Printf("Error writing to TUN: %v", err)
			continue
		}
	}
}

func (c *Client) Stop() {
	if c.wsConn != nil {
		c.wsConn.Close()
	}
	if c.tunInterface != nil {
		c.tunInterface.Close()
	}
}

func main() {
	serverURL := flag.String("server", "", "VPN server URL (e.g. example.com:8080)")
	presharedKey := flag.String("psk", "", "Pre-shared key")
	flag.Parse()

	if *serverURL == "" || *presharedKey == "" {
		flag.Usage()
		os.Exit(1)
	}

	client := NewClient(*serverURL, *presharedKey)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down...")
		client.Stop()
		os.Exit(0)
	}()

	log.Printf("Connecting to %s...", *serverURL)
	if err := client.Start(); err != nil {
		log.Fatalf("Error starting client: %v", err)
	}

	select {}
} 