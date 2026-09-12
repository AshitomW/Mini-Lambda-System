// Package certutil provides dynamic X.509 Certificate Authority (CA), server, and client certificate generators for mTLS zero-trust communication.
package certutil

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"
)

// CertificatePair holds PEM-encoded certificate and private key bytes.
type CertificatePair struct {
	CertPEM []byte
	KeyPEM  []byte
}

// PKICluster holds generated CA, server, and client certificate pairs for testing and local mTLS environments.
type PKICluster struct {
	CA     CertificatePair
	Server CertificatePair
	Client CertificatePair
}

// GeneratePKICluster generates an in-memory Root CA, server certificate, and client certificate.
func GeneratePKICluster() (*PKICluster, error) {
	// 1. Generate Root CA
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate CA key: %w", err)
	}

	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "Mini-Lambda Root CA",
			Organization: []string{"Mini-Lambda Internal"},
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}

	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create CA certificate: %w", err)
	}

	caCertPEM := pemEncode("CERTIFICATE", caDER)
	caKeyPEM := pemEncode("RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(caKey))

	// 2. Generate Server Certificate
	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate server key: %w", err)
	}

	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName:   "localhost",
			Organization: []string{"Mini-Lambda Server"},
		},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.IPv6loopback},
		DNSNames:     []string{"localhost"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	serverDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caTemplate, &serverKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create server cert: %w", err)
	}

	serverCertPEM := pemEncode("CERTIFICATE", serverDER)
	serverKeyPEM := pemEncode("RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(serverKey))

	// 3. Generate Client Certificate for mTLS
	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate client key: %w", err)
	}

	clientTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject: pkix.Name{
			CommonName:   "trusted-client-app",
			Organization: []string{"Authorized-Invokers"},
		},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	clientDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, caTemplate, &clientKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create client cert: %w", err)
	}

	clientCertPEM := pemEncode("CERTIFICATE", clientDER)
	clientKeyPEM := pemEncode("RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(clientKey))

	return &PKICluster{
		CA: CertificatePair{
			CertPEM: caCertPEM,
			KeyPEM:  caKeyPEM,
		},
		Server: CertificatePair{
			CertPEM: serverCertPEM,
			KeyPEM:  serverKeyPEM,
		},
		Client: CertificatePair{
			CertPEM: clientCertPEM,
			KeyPEM:  clientKeyPEM,
		},
	}, nil
}

func pemEncode(blockType string, b []byte) []byte {
	var buf bytes.Buffer
	_ = pem.Encode(&buf, &pem.Block{Type: blockType, Bytes: b})
	return buf.Bytes()
}
