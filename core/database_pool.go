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

	options := DefaultDatabasePoolOptions()
	if len(databasePoolOptions) > 0 {
		providedOptions := databasePoolOptions[0]
		if providedOptions.MaxConns > 0 {
			options.MaxConns = providedOptions.MaxConns
		}
		if providedOptions.MinConns >= 0 {
			options.MinConns = providedOptions.MinConns
		}
		if providedOptions.ConnectionTimeoutMs > 0 {
			options.ConnectionTimeoutMs = providedOptions.ConnectionTimeoutMs
		}
		if providedOptions.MaxConnLifetime > 0 {
			options.MaxConnLifetime = providedOptions.MaxConnLifetime
		}
		if providedOptions.MaxConnIdleTime > 0 {
			options.MaxConnIdleTime = providedOptions.MaxConnIdleTime
		}
		if providedOptions.HealthCheckPeriod > 0 {
			options.HealthCheckPeriod = providedOptions.HealthCheckPeriod
		}
		if providedOptions.SSLMode != "" {
			options.SSLMode = providedOptions.SSLMode
		}
		if providedOptions.SSLRootCert != "" {
			options.SSLRootCert = providedOptions.SSLRootCert
		}
		if providedOptions.SSLCert != "" {
			options.SSLCert = providedOptions.SSLCert
		}
		if providedOptions.SSLKey != "" {
			options.SSLKey = providedOptions.SSLKey
		}
	}

	log.Debugf("initializing database connection pool")
	log.Tracef("database pool configured (maxConns: %d, minConns: %d)", databaseConfig.MaxConns, databaseConfig.MinConns)
	databaseConfig.MaxConns = options.MaxConns
	databaseConfig.MinConns = options.MinConns
	databaseConfig.MaxConnLifetime = options.MaxConnLifetime
	databaseConfig.MaxConnIdleTime = options.MaxConnIdleTime
	databaseConfig.HealthCheckPeriod = options.HealthCheckPeriod

	initDatabaseTLS(databaseConfig, options)

	pool, _ := pgxpool.NewWithConfig(ctx, databaseConfig) // Infallible with valid parsed config

	// Verify connection
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, time.Duration(options.ConnectionTimeoutMs)*time.Millisecond)
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

func initDatabaseTLS(databaseConfig *pgxpool.Config, options DatabasePoolOptions) {
	if options.SSLMode == "disable" {
		databaseConfig.ConnConfig.TLSConfig = nil
		return
	}
	if options.SSLMode != "" || options.SSLRootCert != "" || (options.SSLCert != "" && options.SSLKey != "") {
		if databaseConfig.ConnConfig.TLSConfig == nil {
			databaseConfig.ConnConfig.TLSConfig = &tls.Config{}
		}
		if options.SSLRootCert != "" {
			caCertPool := x509.NewCertPool()
			caCert, _ := os.ReadFile(options.SSLRootCert)
			if len(caCert) == 0 {
				caCert = []byte(options.SSLRootCert)
			}
			caCertPool.AppendCertsFromPEM(caCert)
			databaseConfig.ConnConfig.TLSConfig.RootCAs = caCertPool
		}
		if options.SSLCert != "" && options.SSLKey != "" {
			certPEM, _ := os.ReadFile(options.SSLCert)
			if len(certPEM) == 0 {
				certPEM = []byte(options.SSLCert)
			}
			keyPEM, _ := os.ReadFile(options.SSLKey)
			if len(keyPEM) == 0 {
				keyPEM = []byte(options.SSLKey)
			}
			cert, _ := tls.X509KeyPair(certPEM, keyPEM)
			databaseConfig.ConnConfig.TLSConfig.Certificates = []tls.Certificate{cert}
		}
	}
}
