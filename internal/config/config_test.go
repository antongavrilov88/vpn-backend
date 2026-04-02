package config

import (
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("HTTP_READ_TIMEOUT", "6s")
	t.Setenv("HTTP_WRITE_TIMEOUT", "11s")
	t.Setenv("HTTP_IDLE_TIMEOUT", "31s")
	t.Setenv("HTTP_REQUEST_TIMEOUT", "32s")
	t.Setenv("HTTP_READINESS_TIMEOUT", "3s")
	t.Setenv("HTTP_SHUTDOWN_TIMEOUT", "12s")
	t.Setenv("PROXY_SSH_TIMEOUT", "7s")
	t.Setenv("DB_URL", "postgres://test:test@localhost:5432/test?sslmode=disable")
	t.Setenv("DEVICE_PRIVATE_KEY_CIPHER_KEY", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	t.Setenv("PROXY_SSH_HOST", "proxy.internal")
	t.Setenv("PROXY_SSH_PORT", "2222")
	t.Setenv("PROXY_SSH_USER", "deploy")
	t.Setenv("PROXY_SSH_PRIVATE_KEY_PATH", "/keys/proxy")
	t.Setenv("PROXY_SSH_KNOWN_HOSTS_PATH", "/keys/known_hosts")
	t.Setenv("PROXY_SSH_INSECURE_SKIP_HOST_KEY_CHECK", "false")
	t.Setenv("PROXY_ADD_PEER_COMMAND", "sudo /usr/local/bin/vpn-peer-add")
	t.Setenv("PROXY_REMOVE_PEER_COMMAND", "sudo /usr/local/bin/vpn-peer-remove")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.AppEnv != "test" {
		t.Fatalf("AppEnv = %q, want %q", cfg.AppEnv, "test")
	}

	if cfg.HTTP.Addr != ":9090" {
		t.Fatalf("HTTP.Addr = %q, want %q", cfg.HTTP.Addr, ":9090")
	}

	if cfg.HTTP.ReadTimeout != 6*time.Second {
		t.Fatalf("ReadTimeout = %v, want %v", cfg.HTTP.ReadTimeout, 6*time.Second)
	}

	if cfg.HTTP.WriteTimeout != 11*time.Second {
		t.Fatalf("WriteTimeout = %v, want %v", cfg.HTTP.WriteTimeout, 11*time.Second)
	}

	if cfg.HTTP.IdleTimeout != 31*time.Second {
		t.Fatalf("IdleTimeout = %v, want %v", cfg.HTTP.IdleTimeout, 31*time.Second)
	}

	if cfg.HTTP.RequestTimeout != 32*time.Second {
		t.Fatalf("RequestTimeout = %v, want %v", cfg.HTTP.RequestTimeout, 32*time.Second)
	}

	if cfg.HTTP.ReadinessTimeout != 3*time.Second {
		t.Fatalf("ReadinessTimeout = %v, want %v", cfg.HTTP.ReadinessTimeout, 3*time.Second)
	}

	if cfg.HTTP.ShutdownTimeout != 12*time.Second {
		t.Fatalf("ShutdownTimeout = %v, want %v", cfg.HTTP.ShutdownTimeout, 12*time.Second)
	}

	if cfg.DB.URL != "postgres://test:test@localhost:5432/test?sslmode=disable" {
		t.Fatalf("DB.URL = %q, want expected url", cfg.DB.URL)
	}

	if len(cfg.Crypto.DevicePrivateKeyCipherKey) != 32 {
		t.Fatalf("Crypto.DevicePrivateKeyCipherKey length = %d, want %d", len(cfg.Crypto.DevicePrivateKeyCipherKey), 32)
	}

	if cfg.Proxy.Host != "proxy.internal" {
		t.Fatalf("Proxy.Host = %q, want %q", cfg.Proxy.Host, "proxy.internal")
	}

	if cfg.Proxy.Port != 2222 {
		t.Fatalf("Proxy.Port = %d, want %d", cfg.Proxy.Port, 2222)
	}

	if cfg.Proxy.User != "deploy" {
		t.Fatalf("Proxy.User = %q, want %q", cfg.Proxy.User, "deploy")
	}

	if cfg.Proxy.PrivateKeyPath != "/keys/proxy" {
		t.Fatalf("Proxy.PrivateKeyPath = %q, want %q", cfg.Proxy.PrivateKeyPath, "/keys/proxy")
	}

	if cfg.Proxy.KnownHostsPath != "/keys/known_hosts" {
		t.Fatalf("Proxy.KnownHostsPath = %q, want %q", cfg.Proxy.KnownHostsPath, "/keys/known_hosts")
	}

	if cfg.Proxy.InsecureSkipHostKeyCheck {
		t.Fatal("Proxy.InsecureSkipHostKeyCheck = true, want false")
	}

	if cfg.Proxy.AddPeerCommand != "sudo /usr/local/bin/vpn-peer-add" {
		t.Fatalf("Proxy.AddPeerCommand = %q, want expected value", cfg.Proxy.AddPeerCommand)
	}

	if cfg.Proxy.RemovePeerCommand != "sudo /usr/local/bin/vpn-peer-remove" {
		t.Fatalf("Proxy.RemovePeerCommand = %q, want expected value", cfg.Proxy.RemovePeerCommand)
	}

	if cfg.Proxy.Timeout != 7*time.Second {
		t.Fatalf("Proxy.Timeout = %v, want %v", cfg.Proxy.Timeout, 7*time.Second)
	}
}

func TestLoadBuildsDBURLFromPostgresEnv(t *testing.T) {
	t.Setenv("POSTGRES_HOST", "localhost")
	t.Setenv("POSTGRES_PORT", "5433")
	t.Setenv("POSTGRES_DB", "vpn_mvp")
	t.Setenv("POSTGRES_USER", "postgres")
	t.Setenv("POSTGRES_PASSWORD", "postgres")
	t.Setenv("DEVICE_PRIVATE_KEY_CIPHER_KEY", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.DB.URL != "postgres://postgres:postgres@localhost:5433/vpn_mvp?sslmode=disable" {
		t.Fatalf("DB.URL = %q, want derived url", cfg.DB.URL)
	}
}

func TestLoadRequiresDatabaseConfiguration(t *testing.T) {
	t.Setenv("DB_URL", "")
	t.Setenv("POSTGRES_HOST", "")
	t.Setenv("POSTGRES_PORT", "")
	t.Setenv("POSTGRES_DB", "")
	t.Setenv("POSTGRES_USER", "")
	t.Setenv("POSTGRES_PASSWORD", "")
	t.Setenv("DEVICE_PRIVATE_KEY_CIPHER_KEY", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}

func TestLoadRejectsInvalidDuration(t *testing.T) {
	t.Setenv("DB_URL", "postgres://test:test@localhost:5432/test?sslmode=disable")
	t.Setenv("DEVICE_PRIVATE_KEY_CIPHER_KEY", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	t.Setenv("HTTP_REQUEST_TIMEOUT", "not-a-duration")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}

func TestLoadRejectsInvalidProxyTimeout(t *testing.T) {
	t.Setenv("DB_URL", "postgres://test:test@localhost:5432/test?sslmode=disable")
	t.Setenv("PROXY_SSH_TIMEOUT", "not-a-duration")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}

func TestLoadRejectsInvalidProxyHostKeyCheckFlag(t *testing.T) {
	t.Setenv("DB_URL", "postgres://test:test@localhost:5432/test?sslmode=disable")
	t.Setenv("PROXY_SSH_INSECURE_SKIP_HOST_KEY_CHECK", "maybe")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}

func TestLoadAllowsMissingCipherKeyBeforeRuntimeWiring(t *testing.T) {
	t.Setenv("DB_URL", "postgres://test:test@localhost:5432/test?sslmode=disable")
	t.Setenv("DEVICE_PRIVATE_KEY_CIPHER_KEY", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.Crypto.DevicePrivateKeyCipherKey) != 0 {
		t.Fatalf("Crypto.DevicePrivateKeyCipherKey length = %d, want %d", len(cfg.Crypto.DevicePrivateKeyCipherKey), 0)
	}
}

func TestLoadRejectsInvalidCipherKey(t *testing.T) {
	t.Setenv("DB_URL", "postgres://test:test@localhost:5432/test?sslmode=disable")
	t.Setenv("DEVICE_PRIVATE_KEY_CIPHER_KEY", "not-base64")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}
