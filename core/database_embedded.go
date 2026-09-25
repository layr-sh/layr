package core

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
)

const (
	// EmbeddedDatabaseUsername is the standardized PostgreSQL username for Layr.
	EmbeddedDatabaseUsername = "layr"
	// EmbeddedDatabasePassword is the standardized PostgreSQL password for Layr.
	EmbeddedDatabasePassword = "layr"
	// EmbeddedDatabaseName is the standardized PostgreSQL database name for Layr.
	EmbeddedDatabaseName = "layr"
	// EmbeddedDatabasePostgresVersion is the frozen PostgreSQL version used by the embedded engine.
	EmbeddedDatabasePostgresVersion = embeddedpostgres.V18
)

const (
	defaultDirectoryPermission   = 0755
	embeddedPostgresStartTimeout = 15 * time.Second
)

// IsEmbeddedDatabasePath returns true if the database URL is a directory path (not a postgres:// URI).
func IsEmbeddedDatabasePath(databaseURL string) bool {
	return !strings.HasPrefix(databaseURL, "postgres://") && !strings.HasPrefix(databaseURL, "postgresql://")
}

// EmbeddedDatabase manages an embedded PostgreSQL instance.
type EmbeddedDatabase struct {
	dataDir    string
	postgres   *embeddedpostgres.EmbeddedPostgres //nolint:namingclarity // prefer simple name postgres over embeddedPostgres
	port       uint32
	portFinder func() (uint32, error) // injectable for testing
}

// NewEmbeddedDatabase creates a new embedded postgres manager.
func NewEmbeddedDatabase(dataDir string) *EmbeddedDatabase {
	return &EmbeddedDatabase{
		dataDir:    dataDir,
		port:       0, // Dynamic allocation
		portFinder: findAvailablePort,
	}
}

// Start initializes and starts embedded PostgreSQL.
func (embeddedDatabase *EmbeddedDatabase) Start(ctx context.Context) (string, error) {
	absDataDir, _ := filepath.Abs(embeddedDatabase.dataDir)

	// Dynamically acquire available local TCP port
	if embeddedDatabase.port == 0 {
		freePort, err := embeddedDatabase.portFinder()
		if err != nil {
			return "", fmt.Errorf("failed to find available port for embedded postgres: %w", err)
		}
		embeddedDatabase.port = freePort
	}

	cachePath, _ := filepath.Abs(filepath.Join(".layr", "cache", "pg"))
	_ = os.MkdirAll(cachePath, defaultDirectoryPermission)
	_ = os.MkdirAll(absDataDir, defaultDirectoryPermission)
	runtimePath := filepath.Join(absDataDir, "runtime")
	dataPath := filepath.Join(absDataDir, "pgdata")
	binariesPath := filepath.Join(absDataDir, "binaries")

	log.Debugf("configuring embedded PostgreSQL on port %d", embeddedDatabase.port)
	log.Tracef("embedded PostgreSQL paths: data=%s, runtime=%s", dataPath, runtimePath)

	embeddedDatabaseConfig := embeddedpostgres.DefaultConfig().
		Username(EmbeddedDatabaseUsername).
		Password(EmbeddedDatabasePassword).
		Database(EmbeddedDatabaseName).
		Version(EmbeddedDatabasePostgresVersion).
		Encoding("UTF8").
		Locale("C").
		CachePath(cachePath).
		RuntimePath(runtimePath).
		DataPath(dataPath).
		BinariesPath(binariesPath).
		BinaryRepositoryURL("https://repo1.maven.org/maven2").
		Port(embeddedDatabase.port).
		StartTimeout(embeddedPostgresStartTimeout).
		StartParameters(map[string]string{
			"max_connections": "101",
		})

	embeddedDatabase.postgres = embeddedpostgres.NewDatabase(embeddedDatabaseConfig)

	log.Infof("Starting Embedded PostgreSQL %s on port %d (dataDir: %s)...", EmbeddedDatabasePostgresVersion, embeddedDatabase.port, absDataDir)
	if err := embeddedDatabase.postgres.Start(); err != nil {
		return "", fmt.Errorf("failed to start embedded postgres: %w", err)
	}

	databaseURL := fmt.Sprintf("postgres://%s:%s@127.0.0.1:%d/%s?sslmode=disable", EmbeddedDatabaseUsername, EmbeddedDatabasePassword, embeddedDatabase.port, EmbeddedDatabaseName)
	log.Infof("PostgreSQL ready at %s", databaseURL)

	return databaseURL, nil
}

// Stop gracefully terminates embedded postgres.
func (embeddedDatabase *EmbeddedDatabase) Stop() error {
	if embeddedDatabase.postgres == nil {
		return nil
	}
	log.Tracef("Shutting down embedded PostgreSQL...")
	err := embeddedDatabase.postgres.Stop()
	embeddedDatabase.postgres = nil
	return err
}

// findAvailablePort scans dedicated high user ranges with a randomized offset
// and dynamically verifies socket availability via net.Listen on both IPv4 and IPv6.
//
// Rationale for chosen port ranges:
//  1. Avoids well-known/registered service ports (0-10000, e.g. Postgres 5432, Redis 6379, HTTP 8080).
//  2. Avoids default OS ephemeral port allocation ranges (49152-65535, Linux ip_local_port_range 32768-60999)
//     to prevent collisions with outbound client sockets.
//  3. 25000-25999 is predominantly unassigned in the IANA port registry.
//  4. Mnemonic convention: 25432 (2 + 5432) and 35432 (3 + 5432) maintain Postgres recognizable identifiers.
func findAvailablePort() (uint32, error) {
	return findAvailablePortInRanges(
		portRange{start: 25432, count: 568},
		portRange{start: 35432, count: 568},
	)
}

// portRange defines a contiguous port range for scanning.
type portRange struct {
	start int
	count int
}

// findAvailablePortInRanges scans given port ranges for an available port.
// Primary range uses randomized offset and dual-stack probing.
// Fallback ranges use sequential IPv4-only probing.
func findAvailablePortInRanges(primaryPortRange portRange, fallbacks ...portRange) (uint32, error) {
	// Randomize starting index within primary port range
	offsetBigInt, _ := rand.Int(rand.Reader, big.NewInt(int64(primaryPortRange.count)))
	offset := int(offsetBigInt.Int64())

	var listenConfig net.ListenConfig

	for i := 0; i < primaryPortRange.count; i++ {
		port := primaryPortRange.start + ((offset + i) % primaryPortRange.count)

		tcp4Listener, tcp4ListenErr := listenConfig.Listen(context.Background(), "tcp4", fmt.Sprintf("127.0.0.1:%d", port))
		if tcp4ListenErr != nil {
			continue
		}
		tcp6Listener, tcp6ListenErr := listenConfig.Listen(context.Background(), "tcp6", fmt.Sprintf("[::1]:%d", port))
		if tcp6ListenErr != nil {
			_ = tcp4Listener.Close()
			continue
		}

		_ = tcp4Listener.Close()
		_ = tcp6Listener.Close()
		return uint32(port), nil
	}

	// Scan fallback ranges sequentially
	for _, fallback := range fallbacks {
		for i := 0; i < fallback.count; i++ {
			port := fallback.start + i
			tcp4Listener, tcp4ListenErr := listenConfig.Listen(context.Background(), "tcp4", fmt.Sprintf("127.0.0.1:%d", port))
			if tcp4ListenErr != nil {
				continue
			}
			_ = tcp4Listener.Close()
			return uint32(port), nil
		}
	}

	return 0, fmt.Errorf("could not find an available safe port for embedded PostgreSQL")
}
