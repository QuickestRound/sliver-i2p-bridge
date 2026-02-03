package i2p

import (
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/go-i2p/i2pkeys"
	sam3 "github.com/go-i2p/go-sam-go"
)

// Session manages an I2P streaming session via SAM
type Session struct {
	samAddr     string
	sam         *sam3.SAM
	session     *sam3.StreamSession
	destination string
	keys        sam3.I2PKeys
}

// NewSession creates a new I2P session
func NewSession(samHost string, samPort int, keyPath string, persistKeys bool) (*Session, error) {
	samAddr := fmt.Sprintf("%s:%d", samHost, samPort)

	s := &Session{
		samAddr: samAddr,
	}

	// Connect to SAM bridge
	samConn, err := sam3.NewSAM(samAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SAM at %s: %w", samAddr, err)
	}
	s.sam = samConn

	// Load or generate keys
	if persistKeys && keyPath != "" {
		keys, err := s.loadOrGenerateKeys(keyPath)
		if err != nil {
			samConn.Close()
			return nil, fmt.Errorf("failed to handle keys: %w", err)
		}
		s.keys = keys
	} else {
		// Generate new keys (ephemeral - new address each time)
		keys, err := samConn.NewKeys()
		if err != nil {
			samConn.Close()
			return nil, fmt.Errorf("failed to generate keys: %w", err)
		}
		s.keys = keys
	}

	return s, nil
}

// Start creates the streaming session and listener
func (s *Session) Start() error {
	var err error

	// Create stream session with our keys
	sessionID := fmt.Sprintf("sliver-i2p-bridge-%s", sam3.RandString())
	session, err := s.sam.NewStreamSession(sessionID, s.keys, sam3.Options_Default)
	if err != nil {
		return fmt.Errorf("failed to create streaming session: %w", err)
	}
	s.session = session

	// Get the destination from the keys
	s.destination = s.keys.Addr().Base64()

	return nil
}

// Accept waits for and returns the next incoming I2P connection
func (s *Session) Accept() (net.Conn, error) {
	if s.session == nil {
		return nil, fmt.Errorf("session not started")
	}
	// Use the session's Accept method for incoming connections
	return s.session.Accept()
}

// GetDestination returns the full base64 destination
func (s *Session) GetDestination() string {
	return s.destination
}

// GetB32Address returns the base32 address (without .b32.i2p suffix)
// The B32 address is base32(SHA256(raw_destination_bytes)), lowercase, no padding
func (s *Session) GetB32Address() string {
	if s.destination == "" {
		return ""
	}
	return calculateB32(s.destination)
}

// calculateB32 computes the B32 address from a base64 destination
func calculateB32(destBase64 string) string {
	// Decode the base64 destination to get raw bytes
	destBytes, err := base64.StdEncoding.DecodeString(destBase64)
	if err != nil {
		// I2P uses a modified base64 alphabet, try URL-safe
		destBytes, err = base64.URLEncoding.DecodeString(destBase64)
		if err != nil {
			// Last resort: hash the string directly
			destBytes = []byte(destBase64)
		}
	}

	// SHA256 hash of raw destination bytes
	hash := sha256.Sum256(destBytes)

	// Base32 encode and lowercase (I2P uses lowercase base32)
	b32 := base32.StdEncoding.EncodeToString(hash[:])
	return strings.ToLower(strings.TrimRight(b32, "="))
}

// Close shuts down the I2P session
func (s *Session) Close() error {
	if s.session != nil {
		s.session.Close()
	}
	if s.sam != nil {
		s.sam.Close()
	}
	return nil
}

// loadOrGenerateKeys loads existing keys from disk or generates new ones
// Uses the i2pkeys package's LoadKeys/StoreKeys for proper persistence
func (s *Session) loadOrGenerateKeys(keyPath string) (sam3.I2PKeys, error) {
	// Try to load existing keys
	if _, err := os.Stat(keyPath); err == nil {
		fmt.Printf("[*] Loading existing keys from %s\n", keyPath)
		
		keys, err := i2pkeys.LoadKeys(keyPath)
		if err != nil {
			fmt.Printf("[!] Failed to load keys: %v\n", err)
			fmt.Printf("[*] Generating new keys instead...\n")
		} else {
			fmt.Printf("[+] Keys loaded successfully!\n")
			fmt.Printf("[+] Your B32 address is preserved: %s.b32.i2p\n", keys.Addr().Base32())
			return sam3.I2PKeys(keys), nil
		}
	}

	// Generate new keys
	fmt.Printf("[*] Generating new I2P destination keys...\n")
	keys, err := s.sam.NewKeys()
	if err != nil {
		return sam3.I2PKeys{}, fmt.Errorf("failed to generate keys: %w", err)
	}

	// Save keys using i2pkeys.StoreKeys for proper format
	if err := i2pkeys.StoreKeys(i2pkeys.I2PKeys(keys), keyPath); err != nil {
		fmt.Printf("[!] Warning: failed to save keys to %s: %v\n", keyPath, err)
		fmt.Printf("[!] Keys will not persist across restarts!\n")
	} else {
		fmt.Printf("[+] Keys saved to %s\n", keyPath)
		fmt.Printf("[+] Your B32 address: %s.b32.i2p\n", keys.Addr().Base32())
	}

	return keys, nil
}

// GenerateDestinationKeys generates and saves new I2P keys
func GenerateDestinationKeys(samHost string, samPort int, keyPath string) (string, error) {
	samAddr := fmt.Sprintf("%s:%d", samHost, samPort)

	samConn, err := sam3.NewSAM(samAddr)
	if err != nil {
		return "", fmt.Errorf("failed to connect to SAM: %w", err)
	}
	defer samConn.Close()

	keys, err := samConn.NewKeys()
	if err != nil {
		return "", fmt.Errorf("failed to generate keys: %w", err)
	}

	// Save keys using i2pkeys.StoreKeys for proper format
	if err := i2pkeys.StoreKeys(i2pkeys.I2PKeys(keys), keyPath); err != nil {
		return "", fmt.Errorf("failed to save keys: %w", err)
	}

	// Return B32 address using the built-in method
	return keys.Addr().Base32(), nil
}

// CheckSAMStatus tests connectivity to the SAM bridge
func CheckSAMStatus(samHost string, samPort int) (bool, error) {
	samAddr := fmt.Sprintf("%s:%d", samHost, samPort)

	samConn, err := sam3.NewSAM(samAddr)
	if err != nil {
		return false, err
	}
	defer samConn.Close()

	return true, nil
}
