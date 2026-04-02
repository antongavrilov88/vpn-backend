package wireguard

import (
	"context"
	"fmt"
	"net/netip"
	"strings"

	"vpn-backend/internal/domain"
)

type ClientConfigBuilderConfig struct {
	ServerPublicKey     string
	Endpoint            string
	AllowedIPs          []string
	DNS                 []string
	PersistentKeepalive *int
}

type ClientConfigBuilder struct {
	serverPublicKey     string
	endpoint            string
	allowedIPs          []string
	dns                 []string
	persistentKeepalive *int
}

var _ domain.ClientConfigBuilder = (*ClientConfigBuilder)(nil)

func NewClientConfigBuilder(cfg ClientConfigBuilderConfig) (*ClientConfigBuilder, error) {
	if cfg.ServerPublicKey == "" {
		return nil, fmt.Errorf("server public key is required")
	}

	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("endpoint is required")
	}

	if len(cfg.AllowedIPs) == 0 {
		return nil, fmt.Errorf("allowed IPs are required")
	}

	return &ClientConfigBuilder{
		serverPublicKey:     cfg.ServerPublicKey,
		endpoint:            cfg.Endpoint,
		allowedIPs:          append([]string(nil), cfg.AllowedIPs...),
		dns:                 append([]string(nil), cfg.DNS...),
		persistentKeepalive: cfg.PersistentKeepalive,
	}, nil
}

func (b *ClientConfigBuilder) Build(_ context.Context, input domain.BuildClientConfigInput) (string, error) {
	if input.ClientPrivateKey == "" {
		return "", fmt.Errorf("client private key is required")
	}

	if input.ClientAddress == "" {
		return "", fmt.Errorf("client address is required")
	}

	clientAddress, err := normalizeClientAddress(input.ClientAddress)
	if err != nil {
		return "", err
	}

	var builder strings.Builder

	if input.DeviceName != "" {
		builder.WriteString("# Device: ")
		builder.WriteString(sanitizeComment(input.DeviceName))
		builder.WriteString("\n")
	}

	builder.WriteString("[Interface]\n")
	builder.WriteString("PrivateKey = ")
	builder.WriteString(input.ClientPrivateKey)
	builder.WriteString("\n")
	builder.WriteString("Address = ")
	builder.WriteString(clientAddress)
	builder.WriteString("\n")

	if len(b.dns) > 0 {
		builder.WriteString("DNS = ")
		builder.WriteString(strings.Join(b.dns, ", "))
		builder.WriteString("\n")
	}

	builder.WriteString("\n[Peer]\n")
	builder.WriteString("PublicKey = ")
	builder.WriteString(b.serverPublicKey)
	builder.WriteString("\n")
	builder.WriteString("Endpoint = ")
	builder.WriteString(b.endpoint)
	builder.WriteString("\n")
	builder.WriteString("AllowedIPs = ")
	builder.WriteString(strings.Join(b.allowedIPs, ", "))
	builder.WriteString("\n")

	if b.persistentKeepalive != nil {
		builder.WriteString("PersistentKeepalive = ")
		builder.WriteString(fmt.Sprintf("%d", *b.persistentKeepalive))
		builder.WriteString("\n")
	}

	return builder.String(), nil
}

func normalizeClientAddress(address string) (string, error) {
	if strings.Contains(address, "/") {
		prefix, err := netip.ParsePrefix(address)
		if err != nil {
			return "", fmt.Errorf("invalid client address: %w", err)
		}

		return prefix.String(), nil
	}

	addr, err := netip.ParseAddr(address)
	if err != nil {
		return "", fmt.Errorf("invalid client address: %w", err)
	}

	return netip.PrefixFrom(addr, 32).String(), nil
}

func sanitizeComment(value string) string {
	return strings.NewReplacer("\r", " ", "\n", " ").Replace(value)
}
