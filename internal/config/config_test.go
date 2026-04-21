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
	t.Setenv("DB_MAX_CONNS", "10")
	t.Setenv("DB_MIN_CONNS", "2")
	t.Setenv("DB_MAX_CONN_LIFETIME", "30m")
	t.Setenv("DB_MAX_CONN_IDLE_TIME", "5m")
	t.Setenv("DB_HEALTH_CHECK_PERIOD", "1m")
	t.Setenv("PROXY_SSH_TIMEOUT", "7s")
	t.Setenv("DB_URL", "postgres://test:test@localhost:5432/test?sslmode=disable")
	t.Setenv("DEVICE_PRIVATE_KEY_CIPHER_KEY", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	t.Setenv("VPN_SERVER_PUBLIC_KEY", "server-public-key")
	t.Setenv("VPN_SERVER_ENDPOINT", "vpn.example.com:51820")
	t.Setenv("VPN_SERVER_TUNNEL_ADDRESS", "10.68.0.1/24")
	t.Setenv("VPN_ALLOWED_IPS", "0.0.0.0/0, ::/0")
	t.Setenv("VPN_DNS", "1.1.1.1,8.8.8.8")
	t.Setenv("VPN_PERSISTENT_KEEPALIVE", "25")
	t.Setenv("PROXY_SSH_HOST", "proxy.internal")
	t.Setenv("PROXY_SSH_PORT", "2222")
	t.Setenv("PROXY_SSH_USER", "deploy")
	t.Setenv("PROXY_SSH_CONFIG_PATH", "/keys/ssh_config.app")
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

	if cfg.DB.MaxConns != 10 {
		t.Fatalf("DB.MaxConns = %d, want %d", cfg.DB.MaxConns, 10)
	}

	if cfg.DB.MinConns != 2 {
		t.Fatalf("DB.MinConns = %d, want %d", cfg.DB.MinConns, 2)
	}

	if cfg.DB.MaxConnLifetime != 30*time.Minute {
		t.Fatalf("DB.MaxConnLifetime = %v, want %v", cfg.DB.MaxConnLifetime, 30*time.Minute)
	}

	if cfg.DB.MaxConnIdleTime != 5*time.Minute {
		t.Fatalf("DB.MaxConnIdleTime = %v, want %v", cfg.DB.MaxConnIdleTime, 5*time.Minute)
	}

	if cfg.DB.HealthCheckPeriod != time.Minute {
		t.Fatalf("DB.HealthCheckPeriod = %v, want %v", cfg.DB.HealthCheckPeriod, time.Minute)
	}

	if len(cfg.Crypto.DevicePrivateKeyCipherKey) != 32 {
		t.Fatalf("Crypto.DevicePrivateKeyCipherKey length = %d, want %d", len(cfg.Crypto.DevicePrivateKeyCipherKey), 32)
	}

	if cfg.VPN.ServerPublicKey != "server-public-key" {
		t.Fatalf("VPN.ServerPublicKey = %q, want %q", cfg.VPN.ServerPublicKey, "server-public-key")
	}

	if cfg.VPN.Endpoint != "vpn.example.com:51820" {
		t.Fatalf("VPN.Endpoint = %q, want %q", cfg.VPN.Endpoint, "vpn.example.com:51820")
	}

	if cfg.VPN.ServerTunnelAddress != "10.68.0.1/24" {
		t.Fatalf("VPN.ServerTunnelAddress = %q, want %q", cfg.VPN.ServerTunnelAddress, "10.68.0.1/24")
	}

	if len(cfg.VPN.AllowedIPs) != 2 || cfg.VPN.AllowedIPs[0] != "0.0.0.0/0" || cfg.VPN.AllowedIPs[1] != "::/0" {
		t.Fatalf("VPN.AllowedIPs = %#v, want expected values", cfg.VPN.AllowedIPs)
	}

	if len(cfg.VPN.DNS) != 2 || cfg.VPN.DNS[0] != "1.1.1.1" || cfg.VPN.DNS[1] != "8.8.8.8" {
		t.Fatalf("VPN.DNS = %#v, want expected values", cfg.VPN.DNS)
	}

	if cfg.VPN.PersistentKeepalive == nil || *cfg.VPN.PersistentKeepalive != 25 {
		t.Fatalf("VPN.PersistentKeepalive = %#v, want 25", cfg.VPN.PersistentKeepalive)
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

	if cfg.Proxy.SSHConfigPath != "/keys/ssh_config.app" {
		t.Fatalf("Proxy.SSHConfigPath = %q, want %q", cfg.Proxy.SSHConfigPath, "/keys/ssh_config.app")
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
	t.Setenv("POSTGRES_SSL_MODE", "verify-full")
	t.Setenv("DEVICE_PRIVATE_KEY_CIPHER_KEY", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.DB.URL != "postgres://postgres:postgres@localhost:5433/vpn_mvp?sslmode=verify-full" {
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

func TestLoadRejectsInvalidProxyPort(t *testing.T) {
	t.Setenv("DB_URL", "postgres://test:test@localhost:5432/test?sslmode=disable")
	t.Setenv("PROXY_SSH_PORT", "22workers")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}

func TestLoadRejectsProxyPortOutsideValidRange(t *testing.T) {
	t.Setenv("DB_URL", "postgres://test:test@localhost:5432/test?sslmode=disable")
	t.Setenv("PROXY_SSH_PORT", "70000")

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

func TestLoadRejectsInvalidDBPoolSettings(t *testing.T) {
	t.Setenv("DB_URL", "postgres://test:test@localhost:5432/test?sslmode=disable")
	t.Setenv("DB_MAX_CONNS", "10workers")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}

func TestLoadRejectsZeroDBMaxConns(t *testing.T) {
	t.Setenv("DB_URL", "postgres://test:test@localhost:5432/test?sslmode=disable")
	t.Setenv("DB_MAX_CONNS", "0")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}

func TestLoadRejectsInvalidPostgresSSLMode(t *testing.T) {
	t.Setenv("POSTGRES_HOST", "localhost")
	t.Setenv("POSTGRES_PORT", "5433")
	t.Setenv("POSTGRES_DB", "vpn_mvp")
	t.Setenv("POSTGRES_USER", "postgres")
	t.Setenv("POSTGRES_PASSWORD", "postgres")
	t.Setenv("POSTGRES_SSL_MODE", "broken")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}

func TestLoadIgnoresDerivedPostgresSSLModeWhenDBURLIsSet(t *testing.T) {
	t.Setenv("DB_URL", "postgres://test:test@localhost:5432/test?sslmode=disable")
	t.Setenv("POSTGRES_HOST", "localhost")
	t.Setenv("POSTGRES_PORT", "5433")
	t.Setenv("POSTGRES_DB", "vpn_mvp")
	t.Setenv("POSTGRES_USER", "postgres")
	t.Setenv("POSTGRES_PASSWORD", "postgres")
	t.Setenv("POSTGRES_SSL_MODE", "broken")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.DB.URL != "postgres://test:test@localhost:5432/test?sslmode=disable" {
		t.Fatalf("DB.URL = %q, want expected url", cfg.DB.URL)
	}
}

func TestLoadRejectsInvalidPersistentKeepalive(t *testing.T) {
	t.Setenv("DB_URL", "postgres://test:test@localhost:5432/test?sslmode=disable")
	t.Setenv("VPN_PERSISTENT_KEEPALIVE", "not-an-int")

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

func TestLoadLeavesProxyCommandsEmptyWhenUnset(t *testing.T) {
	t.Setenv("DB_URL", "postgres://test:test@localhost:5432/test?sslmode=disable")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Proxy.AddPeerCommand != "" {
		t.Fatalf("Proxy.AddPeerCommand = %q, want empty", cfg.Proxy.AddPeerCommand)
	}

	if cfg.Proxy.RemovePeerCommand != "" {
		t.Fatalf("Proxy.RemovePeerCommand = %q, want empty", cfg.Proxy.RemovePeerCommand)
	}
}

func TestLoadLeavesProxySSHConfigPathEmptyWhenUnset(t *testing.T) {
	t.Setenv("DB_URL", "postgres://test:test@localhost:5432/test?sslmode=disable")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Proxy.SSHConfigPath != "" {
		t.Fatalf("Proxy.SSHConfigPath = %q, want empty", cfg.Proxy.SSHConfigPath)
	}
}

func TestLoadBot(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("TELEGRAM_BOT_TOKEN", "telegram-token")
	t.Setenv("TELEGRAM_POLL_TIMEOUT", "31s")
	t.Setenv("BACKEND_API_BASE_URL", "http://localhost:8080")
	t.Setenv("BACKEND_API_TIMEOUT", "6s")

	cfg, err := LoadBot()
	if err != nil {
		t.Fatalf("LoadBot() error = %v", err)
	}

	if cfg.AppEnv != "test" {
		t.Fatalf("AppEnv = %q, want %q", cfg.AppEnv, "test")
	}

	if cfg.Bot.Token != "telegram-token" {
		t.Fatalf("Bot.Token = %q, want %q", cfg.Bot.Token, "telegram-token")
	}

	if cfg.Bot.PollTimeout != 31*time.Second {
		t.Fatalf("Bot.PollTimeout = %v, want %v", cfg.Bot.PollTimeout, 31*time.Second)
	}

	if cfg.BackendAPI.BaseURL != "http://localhost:8080" {
		t.Fatalf("BackendAPI.BaseURL = %q, want %q", cfg.BackendAPI.BaseURL, "http://localhost:8080")
	}

	if cfg.BackendAPI.Timeout != 6*time.Second {
		t.Fatalf("BackendAPI.Timeout = %v, want %v", cfg.BackendAPI.Timeout, 6*time.Second)
	}
}

func TestLoadBotRequiresTelegramToken(t *testing.T) {
	t.Setenv("BACKEND_API_BASE_URL", "http://localhost:8080")

	_, err := LoadBot()
	if err == nil {
		t.Fatal("LoadBot() error = nil, want error")
	}
}

func TestLoadBotRequiresBackendAPIBaseURL(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "telegram-token")

	_, err := LoadBot()
	if err == nil {
		t.Fatal("LoadBot() error = nil, want error")
	}
}

func TestLoadBotRejectsInvalidBackendAPIBaseURL(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "telegram-token")
	t.Setenv("BACKEND_API_BASE_URL", "localhost:18080")

	_, err := LoadBot()
	if err == nil {
		t.Fatal("LoadBot() error = nil, want error")
	}
}
