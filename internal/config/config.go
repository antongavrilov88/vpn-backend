package config

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

type Config struct {
	AppEnv string
	HTTP   HTTPConfig
	DB     DBConfig
	Crypto CryptoConfig
	VPN    VPNConfig
	Proxy  ProxyConfig
}

type HTTPConfig struct {
	Addr             string
	ReadTimeout      time.Duration
	WriteTimeout     time.Duration
	IdleTimeout      time.Duration
	RequestTimeout   time.Duration
	ReadinessTimeout time.Duration
	ShutdownTimeout  time.Duration
}

type DBConfig struct {
	URL string
}

type CryptoConfig struct {
	DevicePrivateKeyCipherKey []byte
}

type VPNConfig struct {
	ServerPublicKey     string
	Endpoint            string
	AllowedIPs          []string
	DNS                 []string
	PersistentKeepalive *int
}

type ProxyConfig struct {
	Host                     string
	Port                     int
	User                     string
	PrivateKeyPath           string
	KnownHostsPath           string
	InsecureSkipHostKeyCheck bool
	AddPeerCommand           string
	RemovePeerCommand        string
	Timeout                  time.Duration
}

func Load() (Config, error) {
	readTimeout, err := getDurationEnv("HTTP_READ_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}

	writeTimeout, err := getDurationEnv("HTTP_WRITE_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}

	idleTimeout, err := getDurationEnv("HTTP_IDLE_TIMEOUT", 30*time.Second)
	if err != nil {
		return Config{}, err
	}

	requestTimeout, err := getDurationEnv("HTTP_REQUEST_TIMEOUT", 30*time.Second)
	if err != nil {
		return Config{}, err
	}

	readinessTimeout, err := getDurationEnv("HTTP_READINESS_TIMEOUT", 2*time.Second)
	if err != nil {
		return Config{}, err
	}

	shutdownTimeout, err := getDurationEnv("HTTP_SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}

	proxyTimeout, err := getDurationEnv("PROXY_SSH_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}

	insecureSkipHostKeyCheck, err := getBoolEnv("PROXY_SSH_INSECURE_SKIP_HOST_KEY_CHECK", false)
	if err != nil {
		return Config{}, err
	}

	cipherKey, err := getCipherKeyEnv("DEVICE_PRIVATE_KEY_CIPHER_KEY")
	if err != nil {
		return Config{}, err
	}

	persistentKeepalive, err := getOptionalIntEnv("VPN_PERSISTENT_KEEPALIVE")
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		AppEnv: getEnv("APP_ENV", "development"),
		HTTP: HTTPConfig{
			Addr:             getEnv("HTTP_ADDR", ":8080"),
			ReadTimeout:      readTimeout,
			WriteTimeout:     writeTimeout,
			IdleTimeout:      idleTimeout,
			RequestTimeout:   requestTimeout,
			ReadinessTimeout: readinessTimeout,
			ShutdownTimeout:  shutdownTimeout,
		},
		DB: DBConfig{
			URL: getEnv("DB_URL", buildPostgresURL()),
		},
		Crypto: CryptoConfig{
			DevicePrivateKeyCipherKey: cipherKey,
		},
		VPN: VPNConfig{
			ServerPublicKey:     getEnv("VPN_SERVER_PUBLIC_KEY", ""),
			Endpoint:            getEnv("VPN_SERVER_ENDPOINT", ""),
			AllowedIPs:          getListEnv("VPN_ALLOWED_IPS"),
			DNS:                 getListEnv("VPN_DNS"),
			PersistentKeepalive: persistentKeepalive,
		},
		Proxy: ProxyConfig{
			Host:                     getEnv("PROXY_SSH_HOST", ""),
			Port:                     getIntEnv("PROXY_SSH_PORT", 22),
			User:                     getEnv("PROXY_SSH_USER", ""),
			PrivateKeyPath:           getEnv("PROXY_SSH_PRIVATE_KEY_PATH", ""),
			KnownHostsPath:           getEnv("PROXY_SSH_KNOWN_HOSTS_PATH", ""),
			InsecureSkipHostKeyCheck: insecureSkipHostKeyCheck,
			AddPeerCommand:           getEnv("PROXY_ADD_PEER_COMMAND", "sudo /usr/local/bin/vpn-peer-add"),
			RemovePeerCommand:        getEnv("PROXY_REMOVE_PEER_COMMAND", "sudo /usr/local/bin/vpn-peer-remove"),
			Timeout:                  proxyTimeout,
		},
	}

	if cfg.DB.URL == "" {
		return Config{}, fmt.Errorf("DB_URL or POSTGRES_HOST/POSTGRES_PORT/POSTGRES_DB/POSTGRES_USER/POSTGRES_PASSWORD is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}

func getDurationEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid duration for %s: %w", key, err)
	}

	return duration, nil
}

func getIntEnv(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	var parsed int
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil {
		return fallback
	}

	return parsed
}

func getOptionalIntEnv(key string) (*int, error) {
	value := os.Getenv(key)
	if value == "" {
		return nil, nil
	}

	var parsed int
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil {
		return nil, fmt.Errorf("invalid integer for %s", key)
	}

	return &parsed, nil
}

func getBoolEnv(key string, fallback bool) (bool, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	switch strings.ToLower(value) {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean for %s", key)
	}
}

func getListEnv(key string) []string {
	value := os.Getenv(key)
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}

		values = append(values, trimmed)
	}

	return values
}

func buildPostgresURL() string {
	host := os.Getenv("POSTGRES_HOST")
	port := os.Getenv("POSTGRES_PORT")
	database := os.Getenv("POSTGRES_DB")
	user := os.Getenv("POSTGRES_USER")
	password := os.Getenv("POSTGRES_PASSWORD")

	if host == "" || port == "" || database == "" || user == "" || password == "" {
		return ""
	}

	return (&url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Host:   fmt.Sprintf("%s:%s", host, port),
		Path:   database,
		RawQuery: url.Values{
			"sslmode": []string{"disable"},
		}.Encode(),
	}).String()
}

func getCipherKeyEnv(key string) ([]byte, error) {
	value := os.Getenv(key)
	if value == "" {
		return nil, nil
	}

	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("invalid base64 for %s: %w", key, err)
	}

	if len(decoded) != 32 {
		return nil, fmt.Errorf("%s must decode to 32 bytes", key)
	}

	return decoded, nil
}
