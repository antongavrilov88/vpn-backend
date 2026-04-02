package proxy

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"vpn-backend/internal/domain"
)

type Transport struct {
	address           string
	user              string
	signer            ssh.Signer
	hostKeyCallback   ssh.HostKeyCallback
	addPeerCommand    string
	removePeerCommand string
	timeout           time.Duration
}

var _ domain.VPNTransport = (*Transport)(nil)

type Config struct {
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

func NewTransport(cfg Config) (*Transport, error) {
	if cfg.Host == "" {
		return nil, fmt.Errorf("proxy ssh host is required")
	}

	if cfg.User == "" {
		return nil, fmt.Errorf("proxy ssh user is required")
	}

	if cfg.PrivateKeyPath == "" {
		return nil, fmt.Errorf("proxy ssh private key path is required")
	}

	if cfg.AddPeerCommand == "" {
		return nil, fmt.Errorf("proxy add peer command is required")
	}

	if cfg.RemovePeerCommand == "" {
		return nil, fmt.Errorf("proxy remove peer command is required")
	}

	signer, err := loadSigner(cfg.PrivateKeyPath)
	if err != nil {
		return nil, err
	}

	hostKeyCallback, err := newHostKeyCallback(cfg.KnownHostsPath, cfg.InsecureSkipHostKeyCheck)
	if err != nil {
		return nil, err
	}

	port := cfg.Port
	if port == 0 {
		port = 22
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	return &Transport{
		address:           net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", port)),
		user:              cfg.User,
		signer:            signer,
		hostKeyCallback:   hostKeyCallback,
		addPeerCommand:    cfg.AddPeerCommand,
		removePeerCommand: cfg.RemovePeerCommand,
		timeout:           timeout,
	}, nil
}

func (t *Transport) CreatePeer(ctx context.Context, input domain.CreatePeerInput) (*domain.Peer, error) {
	publicKey, assignedIP, err := validatePeerInput(input.PublicKey, input.AssignedIP)
	if err != nil {
		return nil, err
	}

	command := shellCommand(t.addPeerCommand, publicKey, assignedIP)
	if err := t.run(ctx, command); err != nil {
		return nil, fmt.Errorf("create peer: %w", err)
	}

	return &domain.Peer{
		PublicKey:  publicKey,
		AssignedIP: assignedIP,
	}, nil
}

func (t *Transport) RemovePeer(ctx context.Context, input domain.RemovePeerInput) error {
	publicKey, _, err := validatePeerInput(input.PublicKey, input.AssignedIP)
	if err != nil {
		return err
	}

	command := shellCommand(t.removePeerCommand, publicKey)
	if err := t.run(ctx, command); err != nil {
		return fmt.Errorf("remove peer: %w", err)
	}

	return nil
}

func (t *Transport) GetPeerStatus(context.Context, domain.GetPeerStatusInput) (*domain.PeerStatus, error) {
	return nil, fmt.Errorf("get peer status is not implemented")
}

func (t *Transport) Reconcile(context.Context, domain.ReconcileInput) (*domain.ReconcileResult, error) {
	return nil, fmt.Errorf("reconcile is not implemented")
}

func (t *Transport) run(ctx context.Context, command string) error {
	client, err := ssh.Dial("tcp", t.address, &ssh.ClientConfig{
		User:            t.user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(t.signer)},
		HostKeyCallback: t.hostKeyCallback,
		Timeout:         t.timeout,
	})
	if err != nil {
		return fmt.Errorf("dial proxy ssh: %w", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("create ssh session: %w", err)
	}
	defer session.Close()

	result := make(chan error, 1)
	go func() {
		output, runErr := session.CombinedOutput(command)
		if runErr != nil {
			message := strings.TrimSpace(string(output))
			if message == "" {
				message = runErr.Error()
			}

			result <- fmt.Errorf("%w: %s", runErr, message)
			return
		}

		result <- nil
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-result:
		return err
	}
}

func loadSigner(privateKeyPath string) (ssh.Signer, error) {
	privateKey, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("read proxy ssh private key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("parse proxy ssh private key: %w", err)
	}

	return signer, nil
}

func newHostKeyCallback(knownHostsPath string, insecureSkipHostKeyCheck bool) (ssh.HostKeyCallback, error) {
	if knownHostsPath == "" && !insecureSkipHostKeyCheck {
		return nil, fmt.Errorf("proxy ssh known_hosts path is required unless insecure host key check is explicitly enabled")
	}

	if knownHostsPath == "" {
		return ssh.InsecureIgnoreHostKey(), nil
	}

	callback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("load known_hosts: %w", err)
	}

	return callback, nil
}

func validatePeerInput(publicKey, assignedIP string) (string, string, error) {
	decodedKey, err := base64.StdEncoding.DecodeString(publicKey)
	if err != nil {
		return "", "", fmt.Errorf("invalid public key: %w", err)
	}

	if len(decodedKey) != 32 {
		return "", "", fmt.Errorf("invalid public key length")
	}

	addr, err := netip.ParseAddr(assignedIP)
	if err != nil || !addr.Is4() {
		return "", "", fmt.Errorf("invalid assigned ip")
	}

	return publicKey, addr.String(), nil
}

func shellCommand(command string, args ...string) string {
	parts := []string{command}
	for _, arg := range args {
		parts = append(parts, shellQuote(arg))
	}

	return strings.Join(parts, " ")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}

	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
