package proxy

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"vpn-backend/internal/domain"
)

type Transport struct {
	target            string
	user              string
	signer            ssh.Signer
	hostKeyCallback   ssh.HostKeyCallback
	addPeerCommand    string
	removePeerCommand string
	timeout           time.Duration
	sshConfigPath     string
	sshBinaryPath     string
}

var _ domain.VPNTransport = (*Transport)(nil)

type Config struct {
	Host                     string
	Port                     int
	User                     string
	SSHConfigPath            string
	PrivateKeyPath           string
	KnownHostsPath           string
	InsecureSkipHostKeyCheck bool
	AddPeerCommand           string
	RemovePeerCommand        string
	Timeout                  time.Duration
}

var (
	lookPath           = exec.LookPath
	execCommandContext = exec.CommandContext
)

func NewTransport(cfg Config) (*Transport, error) {
	if cfg.Host == "" {
		return nil, fmt.Errorf("proxy ssh host is required")
	}

	if cfg.AddPeerCommand == "" {
		return nil, fmt.Errorf("proxy add peer command is required")
	}

	if cfg.RemovePeerCommand == "" {
		return nil, fmt.Errorf("proxy remove peer command is required")
	}

	port := cfg.Port
	if port == 0 {
		port = 22
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	if cfg.SSHConfigPath != "" {
		sshBinaryPath, err := lookPath("ssh")
		if err != nil {
			return nil, fmt.Errorf("locate ssh client: %w", err)
		}

		return &Transport{
			target:            openSSHTarget(cfg.Host, cfg.User),
			addPeerCommand:    cfg.AddPeerCommand,
			removePeerCommand: cfg.RemovePeerCommand,
			timeout:           timeout,
			sshConfigPath:     cfg.SSHConfigPath,
			sshBinaryPath:     sshBinaryPath,
		}, nil
	}

	if cfg.User == "" {
		return nil, fmt.Errorf("proxy ssh user is required")
	}

	if cfg.PrivateKeyPath == "" {
		return nil, fmt.Errorf("proxy ssh private key path is required")
	}

	signer, err := loadSigner(cfg.PrivateKeyPath)
	if err != nil {
		return nil, err
	}

	hostKeyCallback, err := newHostKeyCallback(cfg.KnownHostsPath, cfg.InsecureSkipHostKeyCheck)
	if err != nil {
		return nil, err
	}

	return &Transport{
		target:            net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", port)),
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

	command := shellCommand(t.addPeerCommand, publicKey, peerRoute(assignedIP))
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
	if t.sshConfigPath != "" {
		return runSystemSSHCommand(ctx, t.timeout, t.sshBinaryPath, buildOpenSSHArgs(t.sshConfigPath, t.target, t.timeout, command))
	}

	client, err := ssh.Dial("tcp", t.target, &ssh.ClientConfig{
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

	return runRemoteCommand(ctx, t.timeout, func() ([]byte, error) {
		return session.CombinedOutput(command)
	}, func() {
		_ = session.Close()
		_ = client.Close()
	})
}

func runSystemSSHCommand(ctx context.Context, timeout time.Duration, sshBinaryPath string, args []string) error {
	commandCtx := ctx
	cancel := func() {}
	if timeout > 0 {
		commandCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	cmd := execCommandContext(commandCtx, sshBinaryPath, args...)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	if errors.Is(commandCtx.Err(), context.DeadlineExceeded) && timeout > 0 {
		return fmt.Errorf("proxy ssh command timed out after %s: %w", timeout, commandCtx.Err())
	}

	message := strings.TrimSpace(string(output))
	if message == "" {
		message = err.Error()
	}

	return fmt.Errorf("%w: %s", err, message)
}

func buildOpenSSHArgs(configPath, target string, timeout time.Duration, command string) []string {
	args := []string{
		"-F", configPath,
		"-o", "BatchMode=yes",
		"-o", "IdentitiesOnly=yes",
	}

	if timeout > 0 {
		args = append(args, "-o", fmt.Sprintf("ConnectTimeout=%d", sshConnectTimeoutSeconds(timeout)))
	}

	args = append(args, target, command)
	return args
}

func openSSHTarget(host, user string) string {
	if user == "" {
		return host
	}

	return user + "@" + host
}

func sshConnectTimeoutSeconds(timeout time.Duration) int {
	seconds := int(math.Ceil(timeout.Seconds()))
	if seconds < 1 {
		return 1
	}

	return seconds
}

func runRemoteCommand(ctx context.Context, timeout time.Duration, execute func() ([]byte, error), interrupt func()) error {
	commandCtx := ctx
	cancel := func() {}
	if timeout > 0 {
		commandCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	result := make(chan error, 1)
	go func() {
		output, runErr := execute()
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
	case err := <-result:
		return err
	case <-commandCtx.Done():
		interrupt()

		select {
		case err := <-result:
			if err == nil {
				return nil
			}
		default:
		}

		if errors.Is(commandCtx.Err(), context.DeadlineExceeded) && timeout > 0 {
			return fmt.Errorf("proxy ssh command timed out after %s: %w", timeout, commandCtx.Err())
		}

		return commandCtx.Err()
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

func peerRoute(assignedIP string) string {
	addr := netip.MustParseAddr(assignedIP)
	return netip.PrefixFrom(addr, 32).String()
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
