// Package app provides server initialization and TLS/mTLS configuration.
package app

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"AshitomW/mini-lambda/internal/config"
)

// SetupTLSConfig builds a secure TLS 1.3 / mTLS configuration based on system settings.
func SetupTLSConfig(cfg config.Config) (*tls.Config, error) {
	if !cfg.TLSEnabled && !cfg.MTLSEnabled {
		return nil, nil
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load server TLS certificate keypair: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	if cfg.MTLSEnabled {
		if cfg.MTLSClientCAFile == "" {
			return nil, fmt.Errorf("mTLS is enabled but MTLS_CLIENT_CA_FILE is empty")
		}

		caPEM, err := os.ReadFile(cfg.MTLSClientCAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read mTLS client CA file: %w", err)
		}

		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("failed to parse client CA bundle from %s", cfg.MTLSClientCAFile)
		}

		tlsConfig.ClientCAs = caPool
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return tlsConfig, nil
}
