package bridge

import (
	"fmt"
	"sync"
	"time"

	"sliver-i2p-bridge/internal/config"
	"sliver-i2p-bridge/internal/i2p"
	"sliver-i2p-bridge/internal/proxy"
)

// Bridge orchestrates the I2P session and proxy forwarding
type Bridge struct {
	cfg       *config.Config
	session   *i2p.Session
	forwarder *proxy.Forwarder

	running  bool
	mu       sync.Mutex
	shutdown chan struct{}
}

// New creates a new Bridge instance
func New(cfg *config.Config) (*Bridge, error) {
	return &Bridge{
		cfg:      cfg,
		shutdown: make(chan struct{}),
	}, nil
}

// Start initializes the I2P session and begins accepting connections
func (b *Bridge) Start() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.running {
		return fmt.Errorf("bridge already running")
	}

	// Create I2P session
	session, err := i2p.NewSession(
		b.cfg.SAMHost,
		b.cfg.SAMPort,
		b.cfg.KeyPath,
		b.cfg.PersistKeys,
	)
	if err != nil {
		return fmt.Errorf("failed to create I2P session: %w", err)
	}
	b.session = session

	// Start the session (creates destination and listener)
	if err := session.Start(); err != nil {
		session.Close()
		return fmt.Errorf("failed to start I2P session: %w", err)
	}

	// Create forwarder
	b.forwarder = proxy.NewForwarder(
		b.cfg.SliverHost,
		b.cfg.SliverPort,
		b.cfg.SkipTLSVerify,
		b.cfg.SliverCA,
	)

	b.running = true

	// Start accept loop in goroutine
	go b.acceptLoop()

	return nil
}

// acceptLoop handles incoming I2P connections with auto-reconnection
func (b *Bridge) acceptLoop() {
	consecutiveErrors := 0
	const maxConsecutiveErrors = 5

	for {
		select {
		case <-b.shutdown:
			return
		default:
			conn, err := b.session.Accept()
			if err != nil {
				// Check if we're shutting down
				select {
				case <-b.shutdown:
					return
				default:
				}

				consecutiveErrors++
				fmt.Printf("[!] Accept error (%d/%d): %v\n", consecutiveErrors, maxConsecutiveErrors, err)

				// If too many consecutive errors, try to reconnect to SAM
				if consecutiveErrors >= maxConsecutiveErrors {
					fmt.Printf("[!] Too many errors, attempting SAM reconnection...\n")
					if b.tryReconnect() {
						fmt.Printf("[+] SAM reconnection successful!\n")
						consecutiveErrors = 0
					} else {
						fmt.Printf("[!] SAM reconnection failed, will retry in 10s\n")
						time.Sleep(10 * time.Second)
					}
				} else {
					// Backoff to prevent CPU exhaustion
					time.Sleep(1 * time.Second)
				}
				continue
			}

			// Reset error counter on successful accept
			consecutiveErrors = 0

			// Handle connection in goroutine
			go func() {
				if err := b.forwarder.Forward(conn); err != nil {
					fmt.Printf("[!] Forward error: %v\n", err)
				}
			}()
		}
	}
}

// tryReconnect attempts to reinitialize the SAM session
func (b *Bridge) tryReconnect() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Close existing session
	if b.session != nil {
		b.session.Close()
	}

	// Create new session
	session, err := i2p.NewSession(
		b.cfg.SAMHost,
		b.cfg.SAMPort,
		b.cfg.KeyPath,
		b.cfg.PersistKeys,
	)
	if err != nil {
		fmt.Printf("[!] Failed to create new session: %v\n", err)
		return false
	}

	// Start the session
	if err := session.Start(); err != nil {
		session.Close()
		fmt.Printf("[!] Failed to start new session: %v\n", err)
		return false
	}

	b.session = session
	fmt.Printf("[+] Reconnected with B32: %s.b32.i2p\n", session.GetB32Address())
	return true
}

// Stop gracefully shuts down the bridge
func (b *Bridge) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.running {
		return
	}

	close(b.shutdown)

	if b.forwarder != nil {
		b.forwarder.Stop()
	}

	if b.session != nil {
		b.session.Close()
	}

	b.running = false
}

// GetDestination returns the full I2P destination
func (b *Bridge) GetDestination() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.session == nil {
		return ""
	}
	return b.session.GetDestination()
}

// GetB32Address returns the base32 address
func (b *Bridge) GetB32Address() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.session == nil {
		return ""
	}
	return b.session.GetB32Address()
}

// Status represents the current bridge status
type Status struct {
	SAMStatus     string
	SessionActive bool
	Destination   string
}

// CheckStatus checks the status of the SAM bridge
func CheckStatus(cfg *config.Config) (*Status, error) {
	status := &Status{
		SAMStatus:     "DISCONNECTED",
		SessionActive: false,
	}

	connected, err := i2p.CheckSAMStatus(cfg.SAMHost, cfg.SAMPort)
	if err != nil {
		return status, nil // Return status showing disconnected
	}

	if connected {
		status.SAMStatus = "CONNECTED"
	}

	return status, nil
}

// GenerateKeys generates new I2P destination keys
func GenerateKeys(cfg *config.Config) (string, error) {
	return i2p.GenerateDestinationKeys(cfg.SAMHost, cfg.SAMPort, cfg.KeyPath)
}
