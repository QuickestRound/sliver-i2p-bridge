package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// isExpectedCloseError returns true if the error is a normal connection close
// that shouldn't be logged as an error (e.g., when peer closes cleanly)
func isExpectedCloseError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "use of closed network connection") ||
		strings.Contains(errStr, "connection reset by peer") ||
		err == io.EOF
}

// Forwarder handles bidirectional traffic forwarding between I2P and Sliver
type Forwarder struct {
	sliverHost    string
	sliverPort    int
	skipTLSVerify bool
	rootCAs       *x509.CertPool // Pre-loaded CA pool for efficiency

	activeConns sync.WaitGroup
	shutdown    chan struct{}
	closed      atomic.Bool
}

// IdleTimeout is the maximum time a connection can be idle before being closed
// This prevents ghost connections from exhausting the connection pool
const IdleTimeout = 5 * time.Minute

// NewForwarder creates a new traffic forwarder
func NewForwarder(sliverHost string, sliverPort int, skipTLSVerify bool, caPath string) *Forwarder {
	f := &Forwarder{
		sliverHost:    sliverHost,
		sliverPort:    sliverPort,
		skipTLSVerify: skipTLSVerify,
		shutdown:      make(chan struct{}),
	}

	// Pre-load CA certificate if provided (avoids disk I/O on every connection)
	if caPath != "" {
		caCert, err := os.ReadFile(caPath)
		if err == nil {
			pool := x509.NewCertPool()
			if pool.AppendCertsFromPEM(caCert) {
				f.rootCAs = pool
				f.skipTLSVerify = false // Enable verification when CA is loaded
			}
		}
	}

	return f
}

// copyWithTimeout copies data with an idle timeout to detect ghost connections
// This prevents connection pool exhaustion from stalled I2P connections
func copyWithTimeout(dst io.Writer, src net.Conn, timeout time.Duration) error {
	buffer := make([]byte, 32*1024)
	for {
		// Set read deadline before every read
		src.SetReadDeadline(time.Now().Add(timeout))
		n, err := src.Read(buffer)
		if n > 0 {
			if _, wErr := dst.Write(buffer[:n]); wErr != nil {
				return wErr
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
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

	// Use pre-loaded CA if available (cached in NewForwarder)
	if f.rootCAs != nil {
		tlsConfig.RootCAs = f.rootCAs
		tlsConfig.InsecureSkipVerify = false
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

	// Enable TCP KeepAlive to detect dead/ghost connections
	// This helps release semaphore slots when peers disappear without FIN/RST
	if tcpConn, ok := sliverConn.NetConn().(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(3 * time.Minute)
	}

	// Note: We intentionally do NOT set connection deadlines here.
	// SetDeadline() sets an absolute wall-clock time, not an idle timeout.
	// For C2 over I2P (high latency), it's better to let the application
	// layer (Sliver) handle timeouts and reconnection logic.
	// TCP KeepAlive above handles detection of truly dead peers.

	// Use a done channel to signal when either copy completes
	done := make(chan struct{})
	var copyErr error
	var errMu sync.Mutex

	// I2P -> Sliver (with idle timeout to detect ghost connections)
	go func() {
		err := copyWithTimeout(sliverConn, i2pConn, IdleTimeout)
		errMu.Lock()
		// Only capture real errors, not expected close errors
		if copyErr == nil && err != nil && !isExpectedCloseError(err) {
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

	// Sliver -> I2P (with idle timeout to detect ghost connections)
	go func() {
		err := copyWithTimeout(i2pConn, sliverConn, IdleTimeout)
		errMu.Lock()
		// Only capture real errors, not expected close errors
		if copyErr == nil && err != nil && !isExpectedCloseError(err) {
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
