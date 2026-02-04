package i2p

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/go-i2p/i2pkeys"
	sam3 "github.com/go-i2p/go-sam-go"
)

// storeKeysSecure writes keys to file with 0600 permissions from the start
// Uses atomic write (temp file + rename) to prevent corruption on crash
func storeKeysSecure(keys sam3.I2PKeys, keyPath string) error {
	// Create directory if it doesn't exist (with secure 0700 permissions)
	dir := filepath.Dir(keyPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create key directory: %w", err)
	}

	// Enforce directory permissions in case it already existed with weaker perms
	if err := os.Chmod(dir, 0700); err != nil {
		return fmt.Errorf("failed to secure key directory: %w", err)
	}

	// Use atomic write: temp file -> sync -> rename
	// This prevents key file corruption if crash/power loss during write
	tmpFile := keyPath + ".tmp"
	file, err := os.OpenFile(tmpFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create temp key file: %w", err)
	}

	// Write keys
	if _, err := file.WriteString(keys.String()); err != nil {
		file.Close()
		os.Remove(tmpFile)
		return fmt.Errorf("failed to write keys: %w", err)
	}

	// Sync to disk before rename (ensures data durability)
	if err := file.Sync(); err != nil {
		file.Close()
		os.Remove(tmpFile)
		return fmt.Errorf("failed to sync keys: %w", err)
	}
	file.Close()

	// Atomic rename (on POSIX this is atomic within same filesystem)
	if err := os.Rename(tmpFile, keyPath); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to finalize key file: %w", err)
	}

	return nil
}

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
	// Check if key file exists
	_, err := os.Stat(keyPath)
	if err == nil {
		// File exists - try to load it
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
	} else if !os.IsNotExist(err) {
		// CRITICAL FIX: Error is NOT "file not found" (e.g., permission denied)
		// Fail hard to prevent silent key rotation and implant orphaning
		return sam3.I2PKeys{}, fmt.Errorf("CRITICAL: cannot access key file %s: %w (fix permissions or run as correct user)", keyPath, err)
	}

	// Generate new keys
	fmt.Printf("[*] Generating new I2P destination keys...\n")
	keys, err := s.sam.NewKeys()
	if err != nil {
		return sam3.I2PKeys{}, fmt.Errorf("failed to generate keys: %w", err)
	}

	// Save keys securely (file created with 0600 permissions from the start)
	if err := storeKeysSecure(keys, keyPath); err != nil {
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

	// Save keys securely (file created with 0600 permissions from the start)
	if err := storeKeysSecure(keys, keyPath); err != nil {
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
