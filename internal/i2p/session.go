package i2p

import (
	"fmt"
	"net"
	"os"

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
// Uses the built-in i2pkeys method to avoid calculation errors
func (s *Session) GetB32Address() string {
	// Use the built-in method from i2pkeys package
	// This is more reliable than manual calculation
	return s.keys.Addr().Base32()
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
// CRITICAL: If key file exists but fails to load, returns error to prevent
// silently switching to a new identity (which would orphan deployed implants)
func (s *Session) loadOrGenerateKeys(keyPath string) (sam3.I2PKeys, error) {
	// Try to load existing keys
	if _, err := os.Stat(keyPath); err == nil {
		fmt.Printf("[*] Loading existing keys from %s\n", keyPath)
		
		keys, err := i2pkeys.LoadKeys(keyPath)
		if err != nil {
			// CRITICAL: Do NOT silently fall back to new keys!
			// This would change our B32 address and orphan all deployed implants
			return sam3.I2PKeys{}, fmt.Errorf("CRITICAL: key file exists but failed to load: %w (check permissions or delete file to generate new keys)", err)
		}
		fmt.Printf("[+] Keys loaded successfully!\n")
		fmt.Printf("[+] Your B32 address is preserved: %s.b32.i2p\n", keys.Addr().Base32())
		return sam3.I2PKeys(keys), nil
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
		// SECURITY: Ensure key file is only readable by owner (0600)
		if err := os.Chmod(keyPath, 0600); err != nil {
			fmt.Printf("[!] Warning: could not set key file permissions: %v\n", err)
		}
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

	// SECURITY: Ensure key file is only readable by owner (0600)
	// This prevents other users from stealing the I2P identity
	if err := os.Chmod(keyPath, 0600); err != nil {
		return "", fmt.Errorf("failed to set key file permissions: %w", err)
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
