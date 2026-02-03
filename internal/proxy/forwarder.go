package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// Forwarder handles bidirectional traffic forwarding between I2P and Sliver
type Forwarder struct {
	sliverHost    string
	sliverPort    int
	skipTLSVerify bool
	caPath        string // Optional CA certificate path

	activeConns sync.WaitGroup
	shutdown    chan struct{}
	closed      atomic.Bool
}

// NewForwarder creates a new traffic forwarder
func NewForwarder(sliverHost string, sliverPort int, skipTLSVerify bool, caPath string) *Forwarder {
	return &Forwarder{
		sliverHost:    sliverHost,
		sliverPort:    sliverPort,
		skipTLSVerify: skipTLSVerify,
		caPath:        caPath,
		shutdown:      make(chan struct{}),
	}
}

// Forward handles a single I2P connection by forwarding to Sliver
func (f *Forwarder) Forward(i2pConn net.Conn) error {
	f.activeConns.Add(1)
	defer f.activeConns.Done()
	defer i2pConn.Close()

	// Check if we're already shut down
	if f.closed.Load() {
		return nil
	}

	// Connect to Sliver HTTPS listener
	sliverAddr := fmt.Sprintf("%s:%d", f.sliverHost, f.sliverPort)

	tlsConfig := &tls.Config{
		InsecureSkipVerify: f.skipTLSVerify, // Sliver uses self-signed certs
	}

	// Load CA certificate if provided for proper TLS verification
	if f.caPath != "" {
		caCert, err := os.ReadFile(f.caPath)
		if err != nil {
			return fmt.Errorf("failed to read CA certificate: %w", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return fmt.Errorf("failed to parse CA certificate")
		}
		tlsConfig.RootCAs = caCertPool
		tlsConfig.InsecureSkipVerify = false // Override skip if CA is provided
	}

	sliverConn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 30 * time.Second},
		"tcp",
		sliverAddr,
		tlsConfig,
	)
	if err != nil {
		return fmt.Errorf("failed to connect to Sliver at %s: %w", sliverAddr, err)
	}
	defer sliverConn.Close()

	// Note: We intentionally do NOT set connection deadlines here.
	// SetDeadline() sets an absolute wall-clock time, not an idle timeout.
	// For C2 over I2P (high latency), it's better to let the application
	// layer (Sliver) handle timeouts and reconnection logic.

	// Use a done channel to signal when either copy completes
	done := make(chan struct{})
	var copyErr error
	var errMu sync.Mutex

	// I2P -> Sliver
	go func() {
		_, err := io.Copy(sliverConn, i2pConn)
		errMu.Lock()
		if copyErr == nil && err != nil {
			copyErr = err
		}
		errMu.Unlock()
		// Close connections to unblock the other goroutine
		// Note: tls.Conn doesn't support CloseWrite, so we use Close
		sliverConn.Close()
		i2pConn.Close()
		select {
		case done <- struct{}{}:
		default:
		}
	}()

	// Sliver -> I2P
	go func() {
		_, err := io.Copy(i2pConn, sliverConn)
		errMu.Lock()
		if copyErr == nil && err != nil {
			copyErr = err
		}
		errMu.Unlock()
		// Close connections to unblock the other goroutine
		i2pConn.Close()
		sliverConn.Close()
		select {
		case done <- struct{}{}:
		default:
		}
	}()

	// Wait for either direction to complete or shutdown
	select {
	case <-f.shutdown:
		// Force close connections
		i2pConn.Close()
		sliverConn.Close()
		return nil
	case <-done:
		// Wait for the other goroutine to finish cleanup
		// Use 1 second timeout for high-latency I2P connections
		select {
		case <-done:
		case <-time.After(1 * time.Second):
		}
		errMu.Lock()
		err := copyErr
		errMu.Unlock()
		return err
	}
}

// Stop signals the forwarder to shutdown
func (f *Forwarder) Stop() {
	if f.closed.Swap(true) {
		return // Already closed
	}
	close(f.shutdown)
	f.activeConns.Wait()
}

// ProxyStats holds connection statistics
type ProxyStats struct {
	TotalConnections  int64
	ActiveConnections int64
	BytesForwarded    int64
	FailedConnections int64
}
