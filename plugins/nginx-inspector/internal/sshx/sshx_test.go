package sshx

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"
)

func TestParseCertificatePEMSkipsLeadingBlocks(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(7),
		Subject:      pkix.Name{CommonName: "chain.example.com"},
		DNSNames:     []string{"chain.example.com"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, publicKey, privateKey)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})

	var content bytes.Buffer
	if err := pem.Encode(&content, &pem.Block{Type: "PRIVATE KEY", Bytes: []byte("not really a key")}); err != nil {
		t.Fatalf("encode key block: %v", err)
	}
	content.Write(certPEM)

	cert, err := ParseCertificatePEM(content.Bytes())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cert.Subject.CommonName != "chain.example.com" {
		t.Fatalf("common name = %q", cert.Subject.CommonName)
	}
}

func TestParseCertificatePEMRejectsOtherBlocks(t *testing.T) {
	content := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("x")})
	if _, err := ParseCertificatePEM(content); err == nil {
		t.Fatalf("expected an error when no CERTIFICATE block exists")
	}
	if _, err := ParseCertificatePEM([]byte("not pem")); err == nil {
		t.Fatalf("expected an error for non-PEM content")
	}
}

func TestShellQuote(t *testing.T) {
	quoted := ShellQuote("/etc/nginx/nginx.conf")
	if quoted != "'/etc/nginx/nginx.conf'" {
		t.Fatalf("quoted = %q", quoted)
	}
	if !strings.HasPrefix(quoted, "'") || !strings.HasSuffix(quoted, "'") {
		t.Fatalf("quoted value is not single-quoted: %q", quoted)
	}
}

func TestRetryable(t *testing.T) {
	if !retryable(errString("read: connection reset by peer")) {
		t.Fatalf("connection reset should be retryable")
	}
	if retryable(errString("permission denied")) {
		t.Fatalf("permission denied must not be retryable")
	}
	if retryable(nil) {
		t.Fatalf("nil must not be retryable")
	}
}

type errString string

func (e errString) Error() string { return string(e) }
