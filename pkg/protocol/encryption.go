package protocol

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

// EncryptionEngine provides custom encryption on top of TLS
type EncryptionEngine struct {
	aead cipher.AEAD
	key  []byte
}

// NewEncryptionEngine creates a new encryption engine with ChaCha20-Poly1305
func NewEncryptionEngine(key []byte) (*EncryptionEngine, error) {
	if len(key) != 32 {
		return nil, errors.New("key must be 32 bytes")
	}
	
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, err
	}
	
	return &EncryptionEngine{
		aead: aead,
		key:  key,
	}, nil
}

// Encrypt encrypts data with ChaCha20-Poly1305
func (e *EncryptionEngine) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, e.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	
	ciphertext := e.aead.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts data with ChaCha20-Poly1305
func (e *EncryptionEngine) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < e.aead.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}
	
	nonce, ciphertext := ciphertext[:e.aead.NonceSize()], ciphertext[e.aead.NonceSize():]
	plaintext, err := e.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	
	return plaintext, nil
}

// KeyExchange implements X25519 key exchange for perfect forward secrecy
type KeyExchange struct {
	privateKey []byte
	publicKey  []byte
}

// NewKeyExchange creates a new key exchange instance
func NewKeyExchange() (*KeyExchange, error) {
	privateKey := make([]byte, 32)
	if _, err := rand.Read(privateKey); err != nil {
		return nil, err
	}
	
	publicKey, err := curve25519.X25519(privateKey, curve25519.Basepoint)
	if err != nil {
		return nil, err
	}
	
	return &KeyExchange{
		privateKey: privateKey,
		publicKey:  publicKey,
	}, nil
}

// GetPublicKey returns the public key for exchange
func (kx *KeyExchange) GetPublicKey() []byte {
	return kx.publicKey
}

// ComputeSharedSecret computes shared secret from peer's public key
func (kx *KeyExchange) ComputeSharedSecret(peerPublicKey []byte) ([]byte, error) {
	if len(peerPublicKey) != 32 {
		return nil, errors.New("invalid peer public key length")
	}
	
	sharedSecret, err := curve25519.X25519(kx.privateKey, peerPublicKey)
	if err != nil {
		return nil, err
	}
	
	// Derive encryption key using HKDF
	salt := []byte("StealthVPN-2024")
	info := []byte("session-key")
	
	kdf := hkdf.New(sha256.New, sharedSecret, salt, info)
	key := make([]byte, 32)
	if _, err := io.ReadFull(kdf, key); err != nil {
		return nil, err
	}
	
	return key, nil
}

// AESEngine provides AES-256-GCM encryption as fallback
type AESEngine struct {
	aead cipher.AEAD
	key  []byte
}

// NewAESEngine creates a new AES-256-GCM encryption engine
func NewAESEngine(key []byte) (*AESEngine, error) {
	if len(key) != 32 {
		return nil, errors.New("key must be 32 bytes")
	}
	
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	
	return &AESEngine{
		aead: aead,
		key:  key,
	}, nil
}

// Encrypt encrypts data with AES-256-GCM
func (a *AESEngine) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, a.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	
	ciphertext := a.aead.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts data with AES-256-GCM
func (a *AESEngine) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < a.aead.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}
	
	nonce, ciphertext := ciphertext[:a.aead.NonceSize()], ciphertext[a.aead.NonceSize():]
	plaintext, err := a.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	
	return plaintext, nil
}

// MultiLayerEncryption combines multiple encryption algorithms for defense in depth
type MultiLayerEncryption struct {
	chacha *EncryptionEngine
	aes    *AESEngine
}

// NewMultiLayerEncryption creates encryption with multiple algorithms
func NewMultiLayerEncryption(key []byte) (*MultiLayerEncryption, error) {
	// Derive two keys from the master key
	salt1 := []byte("StealthVPN-ChaCha20")
	salt2 := []byte("StealthVPN-AES256")
	
	kdf1 := hkdf.New(sha256.New, key, salt1, []byte("layer1"))
	key1 := make([]byte, 32)
	if _, err := io.ReadFull(kdf1, key1); err != nil {
		return nil, err
	}
	
	kdf2 := hkdf.New(sha256.New, key, salt2, []byte("layer2"))
	key2 := make([]byte, 32)
	if _, err := io.ReadFull(kdf2, key2); err != nil {
		return nil, err
	}
	
	chacha, err := NewEncryptionEngine(key1)
	if err != nil {
		return nil, err
	}
	
	aes, err := NewAESEngine(key2)
	if err != nil {
		return nil, err
	}
	
	return &MultiLayerEncryption{
		chacha: chacha,
		aes:    aes,
	}, nil
}

// Encrypt applies multiple layers of encryption
func (m *MultiLayerEncryption) Encrypt(plaintext []byte) ([]byte, error) {
	// First layer: ChaCha20-Poly1305
	encrypted1, err := m.chacha.Encrypt(plaintext)
	if err != nil {
		return nil, err
	}
	
	// Second layer: AES-256-GCM
	encrypted2, err := m.aes.Encrypt(encrypted1)
	if err != nil {
		return nil, err
	}
	
	return encrypted2, nil
}

// Decrypt removes multiple layers of encryption
func (m *MultiLayerEncryption) Decrypt(ciphertext []byte) ([]byte, error) {
	// Remove second layer: AES-256-GCM
	decrypted1, err := m.aes.Decrypt(ciphertext)
	if err != nil {
		return nil, err
	}
	
	// Remove first layer: ChaCha20-Poly1305
	decrypted2, err := m.chacha.Decrypt(decrypted1)
	if err != nil {
		return nil, err
	}
	
	return decrypted2, nil
} 