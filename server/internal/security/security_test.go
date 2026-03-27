package security

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateToken(t *testing.T) {
	token, err := GenerateToken(32)
	if err != nil {
		t.Fatal(err)
	}
	if len(token) != 64 { // 32 bytes = 64 hex chars
		t.Errorf("token length = %d, want 64", len(token))
	}

	// Should be unique
	token2, _ := GenerateToken(32)
	if token == token2 {
		t.Error("two generated tokens should be different")
	}
}

func TestEnsureCert_AutoGenerate(t *testing.T) {
	dir := t.TempDir()

	cfg := TLSConfig{Enabled: true, AutoCert: true}
	certFile, keyFile, err := EnsureCert(cfg, dir)
	if err != nil {
		t.Fatal(err)
	}

	// Files should exist
	if _, err := os.Stat(certFile); err != nil {
		t.Errorf("cert file missing: %v", err)
	}
	if _, err := os.Stat(keyFile); err != nil {
		t.Errorf("key file missing: %v", err)
	}

	// Second call should reuse existing files
	certFile2, keyFile2, err := EnsureCert(cfg, dir)
	if err != nil {
		t.Fatal(err)
	}
	if certFile2 != certFile || keyFile2 != keyFile {
		t.Error("expected same paths on second call")
	}
}

func TestEnsureCert_ProvidedFiles(t *testing.T) {
	dir := t.TempDir()

	// Create dummy files
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	os.WriteFile(certPath, []byte("cert"), 0644)
	os.WriteFile(keyPath, []byte("key"), 0644)

	cfg := TLSConfig{CertFile: certPath, KeyFile: keyPath}
	gotCert, gotKey, err := EnsureCert(cfg, dir)
	if err != nil {
		t.Fatal(err)
	}
	if gotCert != certPath || gotKey != keyPath {
		t.Error("expected provided file paths")
	}
}

func TestEnsureCert_MissingProvided(t *testing.T) {
	cfg := TLSConfig{CertFile: "/nonexistent/cert.pem", KeyFile: "/nonexistent/key.pem"}
	_, _, err := EnsureCert(cfg, t.TempDir())
	if err == nil {
		t.Error("expected error for missing cert file")
	}
}

func TestLoadTLSConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := TLSConfig{Enabled: true, AutoCert: true}
	certFile, keyFile, err := EnsureCert(cfg, dir)
	if err != nil {
		t.Fatal(err)
	}

	tlsCfg, err := LoadTLSConfig(certFile, keyFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(tlsCfg.Certificates) != 1 {
		t.Errorf("expected 1 certificate, got %d", len(tlsCfg.Certificates))
	}
}
