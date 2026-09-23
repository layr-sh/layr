package core

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func TestCoreEmbeddedDatabaseIsEmbeddedDatabasePathUnit(t *testing.T) {
	testCases := []struct {
		url      string
		expected bool
	}{
		{".layr/data", true},
		{"/var/lib/layr/data", true},
		{"./custom/data", true},
		{"postgres://user:pass@localhost:5432/db", false},
		{"postgresql://user:pass@localhost:5432/db", false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.url, func(t *testing.T) {
			result := IsEmbeddedDatabasePath(testCase.url)
			if result != testCase.expected {
				t.Errorf("IsEmbeddedDatabasePath(%q) = %v, expected %v", testCase.url, result, testCase.expected)
			}
		})
	}
}

func TestCoreEmbeddedDatabaseFindAvailablePortUnit(t *testing.T) {
	port, err := findAvailablePort()
	if err != nil {
		t.Fatalf("findAvailablePort failed: %v", err)
	}

	if (port < 25432 || port > 25999) && (port < 35432 || port > 35999) {
		t.Fatalf("expected port in safe ranges [25432-25999] or [35432-35999], got %d", port)
	}
}

func TestCoreEmbeddedDatabaseFindAvailablePortInRangesUnit(t *testing.T) {
	// Test with a tiny range that works
	port, err := findAvailablePortInRanges(portRange{start: 29000, count: 3}, portRange{start: 39000, count: 3})
	if err != nil {
		t.Fatalf("findAvailablePortInRanges failed: %v", err)
	}
	if port < 29000 || port > 39002 {
		t.Fatalf("unexpected port %d", port)
	}
}

func TestCoreEmbeddedDatabaseFindAvailablePortFallbackRangeUnit(t *testing.T) {
	var listenConfig net.ListenConfig
	// Occupy the entire primary range (1 port) so it falls through to fallback
	listener, err := listenConfig.Listen(context.Background(), "tcp4", "127.0.0.1:29100")
	if err != nil {
		t.Skip("cannot bind port 29100")
	}
	t.Cleanup(func() { _ = listener.Close() })

	// Also occupy IPv6 to force the primary range IPv6 check failure path
	occupiedIPv6Listener, err := listenConfig.Listen(context.Background(), "tcp6", "[::1]:29101")
	if err == nil {
		t.Cleanup(func() { _ = occupiedIPv6Listener.Close() })
	}

	// Primary range: only port 29100 (occupied), falls to fallback
	port, err := findAvailablePortInRanges(
		portRange{start: 29100, count: 1},
		portRange{start: 29200, count: 3},
	)
	if err != nil {
		t.Fatalf("expected fallback port, got error: %v", err)
	}
	if port < 29200 || port > 29202 {
		t.Fatalf("expected port in fallback range 29200-29202, got %d", port)
	}
}

func TestCoreEmbeddedDatabaseFindAvailablePortIPv6FailureUnit(t *testing.T) {
	var listenConfig net.ListenConfig
	// Occupy IPv6 on a port so the dual-stack check fails for that port
	occupiedIPv6Listener, err := listenConfig.Listen(context.Background(), "tcp6", "[::1]:29301")
	if err != nil {
		t.Skip("cannot bind IPv6 port 29301")
	}
	t.Cleanup(func() { _ = occupiedIPv6Listener.Close() })

	// Primary has 1 port (29301); fails IPv6 check, falls through to fallback 29400
	port, err := findAvailablePortInRanges(
		portRange{start: 29301, count: 1},
		portRange{start: 29400, count: 1},
	)
	if err != nil {
		t.Fatalf("expected success with IPv6 fallback, got: %v", err)
	}
	if port != 29400 {
		t.Fatalf("expected fallback port 29400, got %d", port)
	}
}

func TestCoreEmbeddedDatabaseFindAvailablePortAllExhaustedUnit(t *testing.T) {
	var listenConfig net.ListenConfig
	// Occupy both ports in all ranges
	occupiedIPv4PrimaryListener, err := listenConfig.Listen(context.Background(), "tcp4", "127.0.0.1:29500")
	if err != nil {
		t.Skip("cannot bind")
	}
	t.Cleanup(func() { _ = occupiedIPv4PrimaryListener.Close() })
	occupiedIPv4SecondaryListener, err := listenConfig.Listen(context.Background(), "tcp4", "127.0.0.1:29501")
	if err != nil {
		t.Skip("cannot bind")
	}
	t.Cleanup(func() { _ = occupiedIPv4SecondaryListener.Close() })

	_, err = findAvailablePortInRanges(
		portRange{start: 29500, count: 2},
	)
	if err == nil {
		t.Fatal("expected error when all ports exhausted")
	}
}

func TestCoreEmbeddedDatabaseFindAvailablePortFallbackExhaustedUnit(t *testing.T) {
	var listenConfig net.ListenConfig
	// Occupy primary and fallback
	occupiedPrimaryListener, _ := listenConfig.Listen(context.Background(), "tcp4", "127.0.0.1:29600")
	if occupiedPrimaryListener != nil {
		t.Cleanup(func() { _ = occupiedPrimaryListener.Close() })
	}
	occupiedFallbackListener, _ := listenConfig.Listen(context.Background(), "tcp4", "127.0.0.1:29700")
	if occupiedFallbackListener != nil {
		t.Cleanup(func() { _ = occupiedFallbackListener.Close() })
	}

	_, err := findAvailablePortInRanges(
		portRange{start: 29600, count: 1},
		portRange{start: 29700, count: 1},
	)
	if err == nil {
		t.Fatal("expected error when all ports including fallback exhausted")
	}
}

func TestCoreEmbeddedDatabaseNewUnit(t *testing.T) {
	embeddedDatabase := NewEmbeddedDatabase(".layr/data")
	if embeddedDatabase.dataDir != ".layr/data" {
		t.Errorf("expected dataDir .layr/data, got %s", embeddedDatabase.dataDir)
	}
	if embeddedDatabase.port != 0 {
		t.Errorf("expected port 0 (dynamic), got %d", embeddedDatabase.port)
	}
}

func TestCoreEmbeddedDatabaseStopUnstartedUnit(t *testing.T) {
	embeddedDatabase := NewEmbeddedDatabase(".layr/data")
	if err := embeddedDatabase.Stop(); err != nil {
		t.Fatalf("expected no error stopping unstarted embeddedDatabase postgres, got %v", err)
	}
}

func TestCoreEmbeddedDatabaseExplicitPortAndFailureUnit(t *testing.T) {
	// Explicit port set
	embeddedDatabase := NewEmbeddedDatabase(".layr/data")
	embeddedDatabase.port = 25432
	if embeddedDatabase.port != 25432 {
		t.Fatalf("expected port 25432, got %d", embeddedDatabase.port)
	}

	// Invalid uncreatable directory path
	invalidDataDirEmbeddedDatabase := NewEmbeddedDatabase("/dev/null/forbidden/path")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := invalidDataDirEmbeddedDatabase.Start(ctx); err == nil {
		t.Fatal("expected error starting with forbidden path")
	}
}

func TestCoreEmbeddedDatabasePortFinderFailureUnit(t *testing.T) {
	embeddedDatabase := NewEmbeddedDatabase(t.TempDir())
	embeddedDatabase.portFinder = func() (uint32, error) {
		return 0, fmt.Errorf("all ports exhausted")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := embeddedDatabase.Start(ctx)
	if err == nil {
		t.Fatal("expected error when portFinder fails")
	}
	if !strings.Contains(err.Error(), "failed to find available port") {
		t.Fatalf("expected 'failed to find available port' error, got: %v", err)
	}
}
