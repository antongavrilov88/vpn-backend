package app

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"vpn-backend/internal/config"
	"vpn-backend/internal/infra/encryption"
	"vpn-backend/internal/infra/logger"
	"vpn-backend/internal/infra/postgres"
	"vpn-backend/internal/infra/proxy"
	"vpn-backend/internal/infra/wireguard"
)

type App struct {
	Config             config.Config
	Logger             *slog.Logger
	DB                 *pgxpool.Pool
	CreateDevice       *CreateDeviceUseCase
	ResendDeviceConfig *ResendDeviceConfigUseCase
	RevokeDevice       *RevokeDeviceUseCase
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	log := logger.New(cfg.AppEnv)

	db, err := pgxpool.New(ctx, cfg.DB.URL)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	if err := db.Ping(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	createDevice, err := newCreateDeviceUseCase(cfg, db)
	if err != nil {
		db.Close()
		return nil, err
	}
	resendDeviceConfig, err := newResendDeviceConfigUseCase(cfg, db)
	if err != nil {
		db.Close()
		return nil, err
	}
	revokeDevice, err := newRevokeDeviceUseCase(cfg, db)
	if err != nil {
		db.Close()
		return nil, err
	}

	return &App{
		Config:             cfg,
		Logger:             log,
		DB:                 db,
		CreateDevice:       createDevice,
		ResendDeviceConfig: resendDeviceConfig,
		RevokeDevice:       revokeDevice,
	}, nil
}

func newCreateDeviceUseCase(cfg config.Config, db *pgxpool.Pool) (*CreateDeviceUseCase, error) {
	requiredSettings := []requiredSetting{
		{name: "DEVICE_PRIVATE_KEY_CIPHER_KEY", present: len(cfg.Crypto.DevicePrivateKeyCipherKey) > 0},
		{name: "VPN_SERVER_PUBLIC_KEY", present: cfg.VPN.ServerPublicKey != ""},
		{name: "VPN_SERVER_ENDPOINT", present: cfg.VPN.Endpoint != ""},
		{name: "VPN_ALLOWED_IPS", present: len(cfg.VPN.AllowedIPs) > 0},
		{name: "PROXY_SSH_HOST", present: cfg.Proxy.Host != ""},
		{name: "PROXY_SSH_USER", present: cfg.Proxy.User != ""},
		{name: "PROXY_SSH_PRIVATE_KEY_PATH", present: cfg.Proxy.PrivateKeyPath != ""},
	}

	if !hasAny(requiredSettings) {
		return nil, nil
	}

	if missing := missingSettings(requiredSettings); len(missing) > 0 {
		return nil, fmt.Errorf("incomplete create device provisioning config: missing %s", strings.Join(missing, ", "))
	}

	userRepository := postgres.NewUserRepository(db)
	deviceRepository := postgres.NewDeviceRepository(db)
	subscriptionRepository := postgres.NewSubscriptionRepository(db)

	keyGenerator := wireguard.NewKeyGenerator()

	privateKeyCipher, err := encryption.NewPrivateKeyCipher(cfg.Crypto.DevicePrivateKeyCipherKey)
	if err != nil {
		return nil, fmt.Errorf("create private key cipher: %w", err)
	}

	ipAllocator := postgres.NewIPAllocator(db)

	clientConfigBuilder, err := wireguard.NewClientConfigBuilder(wireguard.ClientConfigBuilderConfig{
		ServerPublicKey:     cfg.VPN.ServerPublicKey,
		Endpoint:            cfg.VPN.Endpoint,
		AllowedIPs:          cfg.VPN.AllowedIPs,
		DNS:                 cfg.VPN.DNS,
		PersistentKeepalive: cfg.VPN.PersistentKeepalive,
	})
	if err != nil {
		return nil, fmt.Errorf("create client config builder: %w", err)
	}

	transport, err := proxy.NewTransport(proxy.Config{
		Host:                     cfg.Proxy.Host,
		Port:                     cfg.Proxy.Port,
		User:                     cfg.Proxy.User,
		PrivateKeyPath:           cfg.Proxy.PrivateKeyPath,
		KnownHostsPath:           cfg.Proxy.KnownHostsPath,
		InsecureSkipHostKeyCheck: cfg.Proxy.InsecureSkipHostKeyCheck,
		AddPeerCommand:           cfg.Proxy.AddPeerCommand,
		RemovePeerCommand:        cfg.Proxy.RemovePeerCommand,
		Timeout:                  cfg.Proxy.Timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("create vpn transport: %w", err)
	}

	return NewCreateDeviceUseCase(
		userRepository,
		deviceRepository,
		subscriptionRepository,
		transport,
		keyGenerator,
		privateKeyCipher,
		ipAllocator,
		clientConfigBuilder,
	), nil
}

func newResendDeviceConfigUseCase(cfg config.Config, db *pgxpool.Pool) (*ResendDeviceConfigUseCase, error) {
	requiredSettings := []requiredSetting{
		{name: "DEVICE_PRIVATE_KEY_CIPHER_KEY", present: len(cfg.Crypto.DevicePrivateKeyCipherKey) > 0},
		{name: "VPN_SERVER_PUBLIC_KEY", present: cfg.VPN.ServerPublicKey != ""},
		{name: "VPN_SERVER_ENDPOINT", present: cfg.VPN.Endpoint != ""},
		{name: "VPN_ALLOWED_IPS", present: len(cfg.VPN.AllowedIPs) > 0},
	}

	if !hasAny(requiredSettings) {
		return nil, nil
	}

	if missing := missingSettings(requiredSettings); len(missing) > 0 {
		return nil, fmt.Errorf("incomplete resend device config provisioning config: missing %s", strings.Join(missing, ", "))
	}

	userRepository := postgres.NewUserRepository(db)
	deviceRepository := postgres.NewDeviceRepository(db)

	privateKeyCipher, err := encryption.NewPrivateKeyCipher(cfg.Crypto.DevicePrivateKeyCipherKey)
	if err != nil {
		return nil, fmt.Errorf("create private key cipher: %w", err)
	}

	clientConfigBuilder, err := wireguard.NewClientConfigBuilder(wireguard.ClientConfigBuilderConfig{
		ServerPublicKey:     cfg.VPN.ServerPublicKey,
		Endpoint:            cfg.VPN.Endpoint,
		AllowedIPs:          cfg.VPN.AllowedIPs,
		DNS:                 cfg.VPN.DNS,
		PersistentKeepalive: cfg.VPN.PersistentKeepalive,
	})
	if err != nil {
		return nil, fmt.Errorf("create client config builder: %w", err)
	}

	return NewResendDeviceConfigUseCase(
		userRepository,
		deviceRepository,
		privateKeyCipher,
		clientConfigBuilder,
	), nil
}

func newRevokeDeviceUseCase(cfg config.Config, db *pgxpool.Pool) (*RevokeDeviceUseCase, error) {
	requiredSettings := []requiredSetting{
		{name: "PROXY_SSH_HOST", present: cfg.Proxy.Host != ""},
		{name: "PROXY_SSH_USER", present: cfg.Proxy.User != ""},
		{name: "PROXY_SSH_PRIVATE_KEY_PATH", present: cfg.Proxy.PrivateKeyPath != ""},
		{name: "PROXY_REMOVE_PEER_COMMAND", present: cfg.Proxy.RemovePeerCommand != ""},
		{name: "PROXY_SSH_KNOWN_HOSTS_PATH or PROXY_SSH_INSECURE_SKIP_HOST_KEY_CHECK", present: cfg.Proxy.KnownHostsPath != "" || cfg.Proxy.InsecureSkipHostKeyCheck},
	}

	if !hasAny(requiredSettings) {
		return nil, nil
	}

	if missing := missingSettings(requiredSettings); len(missing) > 0 {
		return nil, fmt.Errorf("incomplete revoke device transport config: missing %s", strings.Join(missing, ", "))
	}

	userRepository := postgres.NewUserRepository(db)
	deviceRepository := postgres.NewDeviceRepository(db)

	transport, err := proxy.NewTransport(proxy.Config{
		Host:                     cfg.Proxy.Host,
		Port:                     cfg.Proxy.Port,
		User:                     cfg.Proxy.User,
		PrivateKeyPath:           cfg.Proxy.PrivateKeyPath,
		KnownHostsPath:           cfg.Proxy.KnownHostsPath,
		InsecureSkipHostKeyCheck: cfg.Proxy.InsecureSkipHostKeyCheck,
		AddPeerCommand:           cfg.Proxy.AddPeerCommand,
		RemovePeerCommand:        cfg.Proxy.RemovePeerCommand,
		Timeout:                  cfg.Proxy.Timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("create vpn transport: %w", err)
	}

	return NewRevokeDeviceUseCase(
		userRepository,
		deviceRepository,
		transport,
	), nil
}

type requiredSetting struct {
	name    string
	present bool
}

func hasAny(settings []requiredSetting) bool {
	for _, setting := range settings {
		if setting.present {
			return true
		}
	}

	return false
}

func missingSettings(settings []requiredSetting) []string {
	missing := make([]string, 0)
	for _, setting := range settings {
		if !setting.present {
			missing = append(missing, setting.name)
		}
	}

	return missing
}

func (a *App) Close(_ context.Context) error {
	if a.DB != nil {
		a.DB.Close()
	}

	return nil
}
