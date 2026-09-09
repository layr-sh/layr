package core

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DatabasePool wraps the pgxpool connection pool.
type DatabasePool struct {
	*pgxpool.Pool
	url string
}

// DatabasePoolOptions defines configurable connection pool parameters.
type DatabasePoolOptions struct {
	MaxConns            int32
	MinConns            int32
	ConnectionTimeoutMs int
	MaxConnLifetime     time.Duration
	MaxConnIdleTime     time.Duration
	HealthCheckPeriod   time.Duration
	SSLMode             string
	SSLRootCert         string
	SSLCert             string
	SSLKey              string
}

// DefaultDatabasePoolOptions returns the standard pool configuration options.
func DefaultDatabasePoolOptions() DatabasePoolOptions {
	return DatabasePoolOptions{
		MaxConns:            25,
		MinConns:            2,
		ConnectionTimeoutMs: 5000,
		MaxConnLifetime:     30 * time.Minute,
		MaxConnIdleTime:     5 * time.Minute,
		HealthCheckPeriod:   15 * time.Second,
	}
}

// NewDatabasePool initializes a connection pool to PostgreSQL with optional configuration.
func NewDatabasePool(ctx context.Context, databaseURL string, databasePoolOptions ...DatabasePoolOptions) (*DatabasePool, error) {
	databaseConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid database url: %w", err)
	}

	effectiveDatabasePoolOptions := DefaultDatabasePoolOptions()
	if len(databasePoolOptions) > 0 {
		providedDatabasePoolOptions := databasePoolOptions[0]
		if providedDatabasePoolOptions.MaxConns > 0 {
			effectiveDatabasePoolOptions.MaxConns = providedDatabasePoolOptions.MaxConns
		}
		if providedDatabasePoolOptions.MinConns >= 0 {
			effectiveDatabasePoolOptions.MinConns = providedDatabasePoolOptions.MinConns
		}
		if providedDatabasePoolOptions.ConnectionTimeoutMs > 0 {
			effectiveDatabasePoolOptions.ConnectionTimeoutMs = providedDatabasePoolOptions.ConnectionTimeoutMs
		}
		if providedDatabasePoolOptions.MaxConnLifetime > 0 {
			effectiveDatabasePoolOptions.MaxConnLifetime = providedDatabasePoolOptions.MaxConnLifetime
		}
		if providedDatabasePoolOptions.MaxConnIdleTime > 0 {
			effectiveDatabasePoolOptions.MaxConnIdleTime = providedDatabasePoolOptions.MaxConnIdleTime
		}
		if providedDatabasePoolOptions.HealthCheckPeriod > 0 {
			effectiveDatabasePoolOptions.HealthCheckPeriod = providedDatabasePoolOptions.HealthCheckPeriod
		}
		if providedDatabasePoolOptions.SSLMode != "" {
			effectiveDatabasePoolOptions.SSLMode = providedDatabasePoolOptions.SSLMode
		}
		if providedDatabasePoolOptions.SSLRootCert != "" {
			effectiveDatabasePoolOptions.SSLRootCert = providedDatabasePoolOptions.SSLRootCert
		}
		if providedDatabasePoolOptions.SSLCert != "" {
			effectiveDatabasePoolOptions.SSLCert = providedDatabasePoolOptions.SSLCert
		}
		if providedDatabasePoolOptions.SSLKey != "" {
			effectiveDatabasePoolOptions.SSLKey = providedDatabasePoolOptions.SSLKey
		}
	}

	log.Debugf("initializing database connection pool")
	log.Tracef("database pool configured (maxConns: %d, minConns: %d)", databaseConfig.MaxConns, databaseConfig.MinConns)
	databaseConfig.MaxConns = effectiveDatabasePoolOptions.MaxConns
	databaseConfig.MinConns = effectiveDatabasePoolOptions.MinConns
	databaseConfig.MaxConnLifetime = effectiveDatabasePoolOptions.MaxConnLifetime
	databaseConfig.MaxConnIdleTime = effectiveDatabasePoolOptions.MaxConnIdleTime
	databaseConfig.HealthCheckPeriod = effectiveDatabasePoolOptions.HealthCheckPeriod

	initDatabaseTLS(databaseConfig, effectiveDatabasePoolOptions)

	pool, _ := pgxpool.NewWithConfig(ctx, databaseConfig) // Infallible with valid parsed config

	// Verify connection
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, time.Duration(effectiveDatabasePoolOptions.ConnectionTimeoutMs)*time.Millisecond)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Tracef("database ping succeeded, pool ready")
	return &DatabasePool{
		Pool: pool,
		url:  databaseURL,
	}, nil
}

func initDatabaseTLS(databaseConfig *pgxpool.Config, databasePoolOptions DatabasePoolOptions) {
	if databasePoolOptions.SSLMode == "disable" {
		databaseConfig.ConnConfig.TLSConfig = nil
		return
	}
	if databasePoolOptions.SSLMode != "" || databasePoolOptions.SSLRootCert != "" || (databasePoolOptions.SSLCert != "" && databasePoolOptions.SSLKey != "") {
		if databaseConfig.ConnConfig.TLSConfig == nil {
			databaseConfig.ConnConfig.TLSConfig = &tls.Config{}
		}
		if databasePoolOptions.SSLRootCert != "" {
			caCertPool := x509.NewCertPool()
			caCert, _ := os.ReadFile(databasePoolOptions.SSLRootCert)
			if len(caCert) == 0 {
				caCert = []byte(databasePoolOptions.SSLRootCert)
			}
			caCertPool.AppendCertsFromPEM(caCert)
			databaseConfig.ConnConfig.TLSConfig.RootCAs = caCertPool
		}
		if databasePoolOptions.SSLCert != "" && databasePoolOptions.SSLKey != "" {
			certPEM, _ := os.ReadFile(databasePoolOptions.SSLCert)
			if len(certPEM) == 0 {
				certPEM = []byte(databasePoolOptions.SSLCert)
			}
			keyPEM, _ := os.ReadFile(databasePoolOptions.SSLKey)
			if len(keyPEM) == 0 {
				keyPEM = []byte(databasePoolOptions.SSLKey)
			}
			certificate, _ := tls.X509KeyPair(certPEM, keyPEM)
			databaseConfig.ConnConfig.TLSConfig.Certificates = []tls.Certificate{certificate}
		}
	}
}
