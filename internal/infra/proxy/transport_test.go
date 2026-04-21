package proxy

import (
	"context"
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPeerRoute(t *testing.T) {
	if got := peerRoute("10.68.0.2"); got != "10.68.0.2/32" {
		t.Fatalf("peerRoute() = %q, want %q", got, "10.68.0.2/32")
	}
}

func TestRunRemoteCommandReturnsCommandTimeout(t *testing.T) {
	blocked := make(chan struct{})
	interrupted := make(chan struct{}, 1)

	err := runRemoteCommand(context.Background(), 20*time.Millisecond, func() ([]byte, error) {
		<-blocked
		return nil, nil
	}, func() {
		select {
		case interrupted <- struct{}{}:
		default:
		}
		close(blocked)
	})

	if err == nil {
		t.Fatal("runRemoteCommand() error = nil, want timeout")
	}

	if !strings.Contains(err.Error(), "proxy ssh command timed out") {
		t.Fatalf("runRemoteCommand() error = %q, want timeout message", err)
	}

	select {
	case <-interrupted:
	case <-time.After(time.Second):
		t.Fatal("interrupt was not called")
	}
}

func TestRunRemoteCommandFormatsRemoteErrorOutput(t *testing.T) {
	err := runRemoteCommand(context.Background(), time.Second, func() ([]byte, error) {
		return []byte("permission denied\n"), errors.New("exit status 1")
	}, func() {})

	if err == nil {
		t.Fatal("runRemoteCommand() error = nil, want wrapped command error")
	}

	if !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("runRemoteCommand() error = %q, want remote stderr", err)
	}
}

func TestNewTransportAllowsDedicatedSSHConfigWithoutInlineKeySettings(t *testing.T) {
	previousLookPath := lookPath
	lookPath = func(file string) (string, error) {
		if file != "ssh" {
			t.Fatalf("lookPath() file = %q, want %q", file, "ssh")
		}

		return "/usr/bin/ssh", nil
	}
	t.Cleanup(func() {
		lookPath = previousLookPath
	})

	transport, err := NewTransport(Config{
		Host:              "yc-vpnmgr",
		SSHConfigPath:     "/run/secrets/ssh_config.app",
		AddPeerCommand:    "sudo -n /usr/local/bin/vpn-peer-add",
		RemovePeerCommand: "sudo -n /usr/local/bin/vpn-peer-remove",
	})
	if err != nil {
		t.Fatalf("NewTransport() error = %v", err)
	}

	if transport.sshConfigPath != "/run/secrets/ssh_config.app" {
		t.Fatalf("sshConfigPath = %q, want %q", transport.sshConfigPath, "/run/secrets/ssh_config.app")
	}

	if transport.target != "yc-vpnmgr" {
		t.Fatalf("target = %q, want %q", transport.target, "yc-vpnmgr")
	}

	if transport.sshBinaryPath != "/usr/bin/ssh" {
		t.Fatalf("sshBinaryPath = %q, want %q", transport.sshBinaryPath, "/usr/bin/ssh")
	}
}

func TestBuildOpenSSHArgsUsesDedicatedConfigFile(t *testing.T) {
	args := buildOpenSSHArgs("/run/secrets/ssh_config.app", "vpnmgr@yc-vpnmgr", 1500*time.Millisecond, "echo ok")
	want := []string{
		"-F", "/run/secrets/ssh_config.app",
		"-o", "BatchMode=yes",
		"-o", "IdentitiesOnly=yes",
		"-o", "ConnectTimeout=2",
		"vpnmgr@yc-vpnmgr",
		"echo ok",
	}

	if !reflect.DeepEqual(args, want) {
		t.Fatalf("buildOpenSSHArgs() = %#v, want %#v", args, want)
	}
}

func TestRunSystemSSHCommandReturnsCommandTimeout(t *testing.T) {
	previousExecCommandContext := execCommandContext
	execCommandContext = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", "sleep 5")
	}
	t.Cleanup(func() {
		execCommandContext = previousExecCommandContext
	})

	err := runSystemSSHCommand(context.Background(), 20*time.Millisecond, "/usr/bin/ssh", []string{"-F", "/run/secrets/ssh_config.app", "yc-vpnmgr", "echo ok"})
	if err == nil {
		t.Fatal("runSystemSSHCommand() error = nil, want timeout")
	}

	if !strings.Contains(err.Error(), "proxy ssh command timed out") {
		t.Fatalf("runSystemSSHCommand() error = %q, want timeout message", err)
	}
}
