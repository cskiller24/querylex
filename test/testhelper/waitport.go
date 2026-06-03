package testhelper

import (
	"fmt"
	"net"
	"testing"
	"time"
)

// WaitForPort performs TCP dial retry with exponential backoff until the
// specified host:port becomes reachable or the timeout is exceeded.
// Backoff parameters: initial delay 500ms, multiplier 1.5x.
func WaitForPort(t *testing.T, host string, port int, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	backoff := 500 * time.Millisecond
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))

	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
		if err == nil {
			conn.Close()
			return
		}

		time.Sleep(backoff)
		backoff = time.Duration(float64(backoff) * 1.5)
	}

	t.Fatalf("timed out waiting for %s after %v", addr, timeout)
}
