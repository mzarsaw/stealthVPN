package protocol

// MessageType represents the type of message being sent
type MessageType string

const (
	// PacketType represents a VPN packet message
	PacketType MessageType = "packet"
)

// Message represents a message sent between client and server
type Message struct {
	Type MessageType `json:"type"`
	Data []byte     `json:"data"`
} 