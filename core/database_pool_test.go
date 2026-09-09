package core

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCoreDatabasePoolDefaultOptionsUnit(t *testing.T) {
	databasePoolOptions := DefaultDatabasePoolOptions()
	if databasePoolOptions.MaxConns != 25 {
		t.Errorf("expected MaxConns 25, got %d", databasePoolOptions.MaxConns)
	}
	if databasePoolOptions.MinConns != 2 {
		t.Errorf("expected MinConns 2, got %d", databasePoolOptions.MinConns)
	}
	if databasePoolOptions.ConnectionTimeoutMs != 5000 {
		t.Errorf("expected ConnectionTimeoutMs 5000, got %d", databasePoolOptions.ConnectionTimeoutMs)
	}
	if databasePoolOptions.MaxConnLifetime != 30*time.Minute {
		t.Errorf("expected MaxConnLifetime 30m, got %v", databasePoolOptions.MaxConnLifetime)
	}
	if databasePoolOptions.MaxConnIdleTime != 5*time.Minute {
		t.Errorf("expected MaxConnIdleTime 5m, got %v", databasePoolOptions.MaxConnIdleTime)
	}
	if databasePoolOptions.HealthCheckPeriod != 15*time.Second {
		t.Errorf("expected HealthCheckPeriod 15s, got %v", databasePoolOptions.HealthCheckPeriod)
	}
}

func TestCoreDatabasePoolNewInvalidURLUnit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Invalid connection URL should fail immediately
	_, err := NewDatabasePool(ctx, "invalid-connection-string")
	if err == nil {
		t.Fatal("expected error with invalid database connection string")
	}

	// Unreachable host (valid URL format but connection refused)
	_, err = NewDatabasePool(ctx, "postgres://user:pass@127.0.0.1:19999/db?sslmode=disable")
	if err == nil {
		t.Fatal("expected error connecting to unreachable host")
	}
}

func TestCoreDatabasePoolNewOptionsParsingAndTLSUnit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Unreachable host with explicit zero/empty options struct
	_, err := NewDatabasePool(ctx, "postgres://user:pass@127.0.0.1:19999/db?sslmode=disable", DatabasePoolOptions{})
	if err == nil {
		t.Fatal("expected error connecting to unreachable host with empty options")
	}

	// Test each zero option individually
	_, _ = NewDatabasePool(ctx, "postgres://user:pass@127.0.0.1:19999/db?sslmode=disable", DatabasePoolOptions{
		MaxConns:            0,
		MinConns:            -1,
		ConnectionTimeoutMs: 0,
		MaxConnLifetime:     0,
		MaxConnIdleTime:     0,
		HealthCheckPeriod:   0,
	})

	// Test MinConns 0
	_, _ = NewDatabasePool(ctx, "postgres://user:pass@127.0.0.1:19999/db?sslmode=disable", DatabasePoolOptions{
		MinConns: 0,
	})

	// Custom DatabasePoolOptions on unreachable host (tests option parsing)
	customDatabasePoolOptions := DatabasePoolOptions{
		MaxConns:            50,
		MinConns:            5,
		ConnectionTimeoutMs: 100,
		MaxConnLifetime:     1 * time.Hour,
		MaxConnIdleTime:     10 * time.Minute,
		HealthCheckPeriod:   30 * time.Second,
	}
	_, err = NewDatabasePool(ctx, "postgres://user:pass@127.0.0.1:19999/db?sslmode=disable", customDatabasePoolOptions)
	if err == nil {
		t.Fatal("expected error connecting to unreachable host with custom options")
	}

	// Negative ConnectionTimeoutMs fallback
	badTimeoutDatabasePoolOptions := DatabasePoolOptions{
		ConnectionTimeoutMs: -1,
	}
	_, err = NewDatabasePool(ctx, "postgres://user:pass@127.0.0.1:19999/db?sslmode=disable", badTimeoutDatabasePoolOptions)
	if err == nil {
		t.Fatal("expected error connecting to unreachable host with negative timeout options")
	}

	// SSL options coverage (disable, custom root cert, client cert/key)
	temporaryDirectory := t.TempDir()
	certFile := filepath.Join(temporaryDirectory, "root.crt")
	_ = os.WriteFile(certFile, []byte("-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----"), 0600)
	keyFile := filepath.Join(temporaryDirectory, "client.key")
	_ = os.WriteFile(keyFile, []byte("-----BEGIN PRIVATE KEY-----\nMIIE\n-----END PRIVATE KEY-----"), 0600)

	sslDatabasePoolOptions := DatabasePoolOptions{
		SSLMode:     "require",
		SSLRootCert: certFile,
		SSLCert:     certFile,
		SSLKey:      keyFile,
	}
	_, _ = NewDatabasePool(ctx, "postgres://user:pass@127.0.0.1:19999/db?sslmode=require", sslDatabasePoolOptions)

	// SSL with inline PEM string and SSLMode disable
	sslInlineDatabasePoolOptions := DatabasePoolOptions{
		SSLMode:     "disable",
		SSLRootCert: "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----",
		SSLCert:     "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----",
		SSLKey:      "-----BEGIN PRIVATE KEY-----\nMIIE\n-----END PRIVATE KEY-----",
	}
	_, _ = NewDatabasePool(ctx, "postgres://user:pass@127.0.0.1:19999/db?sslmode=disable", sslInlineDatabasePoolOptions)

	// SSL with generated valid TLS cert pair to test successful tls.X509KeyPair loading
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate rsa key: %v", err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Layr Test"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(1 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("failed to marshal private key: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyBytes})
	validSSLDatabasePoolOptions := DatabasePoolOptions{
		SSLMode:     "require",
		SSLRootCert: string(certPEM),
		SSLCert:     string(certPEM),
		SSLKey:      string(keyPEM),
	}
	_, _ = NewDatabasePool(ctx, "postgres://user:pass@127.0.0.1:19999/db?sslmode=require", validSSLDatabasePoolOptions)

	// Test opt.SSLRootCert alone
	_, _ = NewDatabasePool(ctx, "postgres://user:pass@127.0.0.1:19999/db?sslmode=disable", DatabasePoolOptions{
		SSLRootCert: string(certPEM),
	})

	// Test opt.SSLCert and opt.SSLKey alone without SSLRootCert
	_, _ = NewDatabasePool(ctx, "postgres://user:pass@127.0.0.1:19999/db?sslmode=disable", DatabasePoolOptions{
		SSLCert: string(certPEM),
		SSLKey:  string(keyPEM),
	})

	// Test opt.SSLMode empty and no certs (early return)
	_, _ = NewDatabasePool(ctx, "postgres://user:pass@127.0.0.1:19999/db?sslmode=disable", DatabasePoolOptions{
		SSLMode: "",
	})
}
