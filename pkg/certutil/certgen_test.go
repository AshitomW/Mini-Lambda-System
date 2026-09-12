package certutil_test

import (
	"crypto/tls"
	"crypto/x509"
	"testing"

	"AshitomW/mini-lambda/pkg/certutil"
)

func TestGeneratePKICluster(t *testing.T) {
	pki, err := certutil.GeneratePKICluster()
	if err != nil {
		t.Fatalf("GeneratePKICluster failed: %v", err)
	}

	if len(pki.CA.CertPEM) == 0 || len(pki.CA.KeyPEM) == 0 {
		t.Fatalf("empty CA cert or key")
	}
	if len(pki.Server.CertPEM) == 0 || len(pki.Server.KeyPEM) == 0 {
		t.Fatalf("empty Server cert or key")
	}
	if len(pki.Client.CertPEM) == 0 || len(pki.Client.KeyPEM) == 0 {
		t.Fatalf("empty Client cert or key")
	}

	// Verify that server certificate parses and matches key
	serverCert, err := tls.X509KeyPair(pki.Server.CertPEM, pki.Server.KeyPEM)
	if err != nil {
		t.Fatalf("failed to parse server keypair: %v", err)
	}
	if len(serverCert.Certificate) == 0 {
		t.Fatalf("expected at least 1 certificate in server keypair")
	}

	// Verify CA cert pool validates client certificate
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(pki.CA.CertPEM) {
		t.Fatalf("failed to append CA cert to pool")
	}

	clientBlock, _ := x509.ParseCertificate(pki.Client.CertPEM) // will be parsed from raw DER or x509 certificate
	clientCert, err := tls.X509KeyPair(pki.Client.CertPEM, pki.Client.KeyPEM)
	if err != nil {
		t.Fatalf("failed to parse client keypair: %v", err)
	}

	parsedClient, err := x509.ParseCertificate(clientCert.Certificate[0])
	if err != nil {
		t.Fatalf("failed to parse client certificate: %v", err)
	}

	opts := x509.VerifyOptions{
		Roots:     caPool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	if _, err := parsedClient.Verify(opts); err != nil {
		t.Fatalf("client certificate failed CA verification: %v", err)
	}
	_ = clientBlock
}
