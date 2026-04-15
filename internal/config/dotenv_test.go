package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotenvFromFilesUsesHighestPrecedenceFileWhenEnvIsEmpty(t *testing.T) {
	t.Setenv("PROXY_SSH_HOST", "")

	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	localEnvPath := filepath.Join(dir, ".env.local")

	if err := os.WriteFile(envPath, []byte("PROXY_SSH_HOST=from-env\n"), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}

	if err := os.WriteFile(localEnvPath, []byte("PROXY_SSH_HOST=from-local-env\n"), 0o600); err != nil {
		t.Fatalf("write local env: %v", err)
	}

	if err := loadDotenvFromFiles([]string{envPath, localEnvPath}); err != nil {
		t.Fatalf("loadDotenvFromFiles() error = %v", err)
	}

	if got := os.Getenv("PROXY_SSH_HOST"); got != "from-local-env" {
		t.Fatalf("PROXY_SSH_HOST = %q, want %q", got, "from-local-env")
	}
}

func TestLoadDotenvFromFilesKeepsNonEmptyEnvironmentValue(t *testing.T) {
	t.Setenv("PROXY_SSH_HOST", "from-shell")

	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	if err := os.WriteFile(envPath, []byte("PROXY_SSH_HOST=from-file\n"), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}

	if err := loadDotenvFromFiles([]string{envPath}); err != nil {
		t.Fatalf("loadDotenvFromFiles() error = %v", err)
	}

	if got := os.Getenv("PROXY_SSH_HOST"); got != "from-shell" {
		t.Fatalf("PROXY_SSH_HOST = %q, want %q", got, "from-shell")
	}
}
