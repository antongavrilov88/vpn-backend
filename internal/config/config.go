package config

import (
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
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

type BotProcessConfig struct {
	AppEnv     string
	Bot        TelegramBotConfig
	BackendAPI BackendAPIConfig
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
	URL               string
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
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

type TelegramBotConfig struct {
	Token       string
	PollTimeout time.Duration
}

type BackendAPIConfig struct {
	BaseURL string
	Timeout time.Duration
}

func Load() (Config, error) {
	dbURL := getEnv("DB_URL", "")
	if dbURL == "" {
		builtDBURL, err := buildPostgresURL()
		if err != nil {
			return Config{}, err
		}

		dbURL = builtDBURL
	}

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

	dbMaxConnLifetime, err := getDurationEnv("DB_MAX_CONN_LIFETIME", 0)
	if err != nil {
		return Config{}, err
	}

	dbMaxConnIdleTime, err := getDurationEnv("DB_MAX_CONN_IDLE_TIME", 0)
	if err != nil {
		return Config{}, err
	}

	dbHealthCheckPeriod, err := getDurationEnv("DB_HEALTH_CHECK_PERIOD", 0)
	if err != nil {
		return Config{}, err
	}

	proxyPort, err := getIntEnv("PROXY_SSH_PORT", 22)
	if err != nil {
		return Config{}, err
	}
	if proxyPort <= 0 || proxyPort > 65535 {
		return Config{}, fmt.Errorf("PROXY_SSH_PORT must be between 1 and 65535")
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

	dbMaxConns, err := getOptionalIntEnv("DB_MAX_CONNS")
	if err != nil {
		return Config{}, err
	}
	if dbMaxConns != nil && *dbMaxConns <= 0 {
		return Config{}, fmt.Errorf("DB_MAX_CONNS must be greater than 0")
	}

	dbMinConns, err := getOptionalIntEnv("DB_MIN_CONNS")
	if err != nil {
		return Config{}, err
	}
	if dbMinConns != nil && *dbMinConns < 0 {
		return Config{}, fmt.Errorf("DB_MIN_CONNS must be greater than or equal to 0")
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
			URL:               dbURL,
			MaxConnLifetime:   dbMaxConnLifetime,
			MaxConnIdleTime:   dbMaxConnIdleTime,
			HealthCheckPeriod: dbHealthCheckPeriod,
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
			Port:                     proxyPort,
			User:                     getEnv("PROXY_SSH_USER", ""),
			PrivateKeyPath:           getEnv("PROXY_SSH_PRIVATE_KEY_PATH", ""),
			KnownHostsPath:           getEnv("PROXY_SSH_KNOWN_HOSTS_PATH", ""),
			InsecureSkipHostKeyCheck: insecureSkipHostKeyCheck,
			AddPeerCommand:           getEnv("PROXY_ADD_PEER_COMMAND", ""),
			RemovePeerCommand:        getEnv("PROXY_REMOVE_PEER_COMMAND", ""),
			Timeout:                  proxyTimeout,
		},
	}

	if dbMaxConns != nil {
		cfg.DB.MaxConns = int32(*dbMaxConns)
	}

	if dbMinConns != nil {
		cfg.DB.MinConns = int32(*dbMinConns)
	}

	if cfg.DB.URL == "" {
		return Config{}, fmt.Errorf("DB_URL or POSTGRES_HOST/POSTGRES_PORT/POSTGRES_DB/POSTGRES_USER/POSTGRES_PASSWORD is required")
	}

	return cfg, nil
}

func LoadBot() (BotProcessConfig, error) {
	pollTimeout, err := getDurationEnv("TELEGRAM_POLL_TIMEOUT", 30*time.Second)
	if err != nil {
		return BotProcessConfig{}, err
	}

	backendTimeout, err := getDurationEnv("BACKEND_API_TIMEOUT", 5*time.Second)
	if err != nil {
		return BotProcessConfig{}, err
	}

	cfg := BotProcessConfig{
		AppEnv: getEnv("APP_ENV", "development"),
		Bot: TelegramBotConfig{
			Token:       getEnv("TELEGRAM_BOT_TOKEN", ""),
			PollTimeout: pollTimeout,
		},
		BackendAPI: BackendAPIConfig{
			BaseURL: getEnv("BACKEND_API_BASE_URL", ""),
			Timeout: backendTimeout,
		},
	}

	if cfg.Bot.Token == "" {
		return BotProcessConfig{}, fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}

	if cfg.BackendAPI.BaseURL == "" {
		return BotProcessConfig{}, fmt.Errorf("BACKEND_API_BASE_URL is required")
	}

	if err := validateAbsoluteURL(cfg.BackendAPI.BaseURL); err != nil {
		return BotProcessConfig{}, fmt.Errorf("invalid BACKEND_API_BASE_URL: %w", err)
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

func getIntEnv(key string, fallback int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	return parseIntEnv(key, value)
}

func getOptionalIntEnv(key string) (*int, error) {
	value := os.Getenv(key)
	if value == "" {
		return nil, nil
	}

	parsed, err := parseIntEnv(key, value)
	if err != nil {
		return nil, err
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

func buildPostgresURL() (string, error) {
	host := os.Getenv("POSTGRES_HOST")
	port := os.Getenv("POSTGRES_PORT")
	database := os.Getenv("POSTGRES_DB")
	user := os.Getenv("POSTGRES_USER")
	password := os.Getenv("POSTGRES_PASSWORD")

	if host == "" || port == "" || database == "" || user == "" || password == "" {
		return "", nil
	}

	sslMode := getEnv("POSTGRES_SSL_MODE", "disable")
	if err := validatePostgresSSLMode(sslMode); err != nil {
		return "", err
	}

	return (&url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Host:   fmt.Sprintf("%s:%s", host, port),
		Path:   database,
		RawQuery: url.Values{
			"sslmode": []string{sslMode},
		}.Encode(),
	}).String(), nil
}

func validateAbsoluteURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return err
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("scheme must be http or https")
	}

	if parsed.Host == "" {
		return fmt.Errorf("host is required")
	}

	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("host is required")
	}

	if port := parsed.Port(); port != "" {
		if _, err := net.LookupPort("tcp", port); err != nil {
			return fmt.Errorf("invalid port")
		}
	}

	return nil
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

func parseIntEnv(key, value string) (int, error) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("invalid integer for %s", key)
	}

	return parsed, nil
}

func validatePostgresSSLMode(value string) error {
	switch value {
	case "disable", "allow", "prefer", "require", "verify-ca", "verify-full":
		return nil
	default:
		return fmt.Errorf("invalid POSTGRES_SSL_MODE")
	}
}
