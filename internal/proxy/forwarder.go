package proxy

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// Forwarder handles bidirectional traffic forwarding between I2P and Sliver
type Forwarder struct {
	sliverHost    string
	sliverPort    int
	skipTLSVerify bool

	activeConns sync.WaitGroup
	shutdown    chan struct{}
	closed      atomic.Bool
}

// NewForwarder creates a new traffic forwarder
func NewForwarder(sliverHost string, sliverPort int, skipTLSVerify bool) *Forwarder {
	return &Forwarder{
		sliverHost:    sliverHost,
		sliverPort:    sliverPort,
		skipTLSVerify: skipTLSVerify,
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

	// Set idle timeout to prevent zombie connections (10 minutes)
	// This ensures hung connections are killed if no data flows
	idleTimeout := 10 * time.Minute
	i2pConn.SetDeadline(time.Now().Add(idleTimeout))
	sliverConn.SetDeadline(time.Now().Add(idleTimeout))

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
