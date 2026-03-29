package security

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// TLSConfig holds TLS configuration.
type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
	AutoCert bool   `yaml:"auto_cert"` // auto-generate self-signed cert
}

// EnsureCert checks if cert/key exist, or generates self-signed ones.
func EnsureCert(cfg TLSConfig, dataDir string) (certFile, keyFile string, err error) {
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		// Use provided files
		if _, err := os.Stat(cfg.CertFile); err != nil {
			return "", "", fmt.Errorf("cert file not found: %w", err)
		}
		if _, err := os.Stat(cfg.KeyFile); err != nil {
			return "", "", fmt.Errorf("key file not found: %w", err)
		}
		return cfg.CertFile, cfg.KeyFile, nil
	}

	// Auto-generate self-signed certificate
	certDir := filepath.Join(dataDir, "certs")
	if err := os.MkdirAll(certDir, 0700); err != nil {
		return "", "", fmt.Errorf("create cert dir: %w", err)
	}

	certFile = filepath.Join(certDir, "server.crt")
	keyFile = filepath.Join(certDir, "server.key")

	// Check if already generated
	if _, err := os.Stat(certFile); err == nil {
		if _, err := os.Stat(keyFile); err == nil {
			return certFile, keyFile, nil
		}
	}

	// Generate self-signed cert
	if err := generateSelfSignedCert(certFile, keyFile); err != nil {
		return "", "", fmt.Errorf("generate cert: %w", err)
	}

	return certFile, keyFile, nil
}

func generateSelfSignedCert(certFile, keyFile string) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}

	serialNumber, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"CloudGuard Monitor"},
			CommonName:   "CloudGuard Self-Signed",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.IPv6loopback},
		DNSNames:              []string{"localhost"},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return err
	}

	// Write cert
	certOut, err := os.OpenFile(certFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	certOut.Close()

	// Write key
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	keyOut, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
	keyOut.Close()

	return nil
}

// LoadTLSConfig loads a TLS config from cert and key files.
func LoadTLSConfig(certFile, keyFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}
