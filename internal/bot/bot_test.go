package bot

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"vpn-backend/internal/infra/backendapi"
	"vpn-backend/internal/infra/telegram"
)

func TestHandleMessageNewDeviceSuccess(t *testing.T) {
	telegramClient := &stubTelegramClient{}
	backendClient := &stubBackendClient{
		createDeviceResult: &backendapi.CreateDeviceResult{
			Device: backendapi.Device{
				ID:         100,
				Name:       "dad-laptop",
				AssignedIP: "10.67.0.2",
				Status:     "active",
				CreatedAt:  time.Date(2026, 4, 2, 10, 11, 12, 0, time.UTC),
			},
			ClientConfig: "[Interface]\nPrivateKey = private-key\n",
		},
	}

	b := New(slog.New(slog.NewTextHandler(io.Discard, nil)), telegramClient, backendClient, time.Second)

	err := b.handleMessage(context.Background(), &telegram.Message{
		Text: "/newdevice dad-laptop",
		Chat: telegram.Chat{ID: 99},
		From: &telegram.User{ID: 777},
	})
	if err != nil {
		t.Fatalf("handleMessage() error = %v", err)
	}

	if backendClient.createDeviceTelegramUserID != 777 {
		t.Fatalf("telegram user id = %d, want %d", backendClient.createDeviceTelegramUserID, 777)
	}

	if backendClient.createDeviceName != "dad-laptop" {
		t.Fatalf("device name = %q, want %q", backendClient.createDeviceName, "dad-laptop")
	}

	if len(telegramClient.sentMessages) != 1 {
		t.Fatalf("sent messages = %d, want %d", len(telegramClient.sentMessages), 1)
	}

	message := telegramClient.sentMessages[0]
	if message.chatID != 99 {
		t.Fatalf("chat id = %d, want %d", message.chatID, 99)
	}

	want := "Device created successfully.\nName: dad-laptop\nIP: 10.67.0.2\nStatus: active\nCreated: 2026-04-02\n\nClient config:\n[Interface]\nPrivateKey = private-key\n"
	if message.text != want {
		t.Fatalf("message = %q, want %q", message.text, want)
	}
}

func TestHandleMessageNewDeviceRequiresName(t *testing.T) {
	telegramClient := &stubTelegramClient{}
	backendClient := &stubBackendClient{}

	b := New(slog.New(slog.NewTextHandler(io.Discard, nil)), telegramClient, backendClient, time.Second)

	err := b.handleMessage(context.Background(), &telegram.Message{
		Text: "/newdevice",
		Chat: telegram.Chat{ID: 99},
		From: &telegram.User{ID: 777},
	})
	if err != nil {
		t.Fatalf("handleMessage() error = %v", err)
	}

	if backendClient.createDeviceCalls != 0 {
		t.Fatalf("create device calls = %d, want %d", backendClient.createDeviceCalls, 0)
	}

	if len(telegramClient.sentMessages) != 1 {
		t.Fatalf("sent messages = %d, want %d", len(telegramClient.sentMessages), 1)
	}

	if got := telegramClient.sentMessages[0].text; got != "Usage: /newdevice <device_name>" {
		t.Fatalf("message = %q, want usage", got)
	}
}

func TestHandleMessageNewDeviceMapsBackendErrors(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantMessage string
	}{
		{
			name:        "validation error",
			err:         &backendapi.APIError{StatusCode: 400, Message: "name is required"},
			wantMessage: "Cannot create device: name is required.",
		},
		{
			name:        "forbidden",
			err:         &backendapi.APIError{StatusCode: 403, Message: "forbidden"},
			wantMessage: "You are not allowed to create a device right now.",
		},
		{
			name:        "conflict",
			err:         &backendapi.APIError{StatusCode: 409, Message: "ip pool exhausted"},
			wantMessage: "Cannot create device: ip pool exhausted.",
		},
		{
			name:        "not found",
			err:         &backendapi.APIError{StatusCode: 404, Message: "not found"},
			wantMessage: "Failed to create the device right now. Please try again later.",
		},
		{
			name:        "unknown",
			err:         errors.New("timeout"),
			wantMessage: "Failed to create the device right now. Please try again later.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			telegramClient := &stubTelegramClient{}
			backendClient := &stubBackendClient{createDeviceErr: test.err}
			b := New(slog.New(slog.NewTextHandler(io.Discard, nil)), telegramClient, backendClient, time.Second)

			err := b.handleMessage(context.Background(), &telegram.Message{
				Text: "/newdevice iphone",
				Chat: telegram.Chat{ID: 99},
				From: &telegram.User{ID: 777},
			})
			if err != nil {
				t.Fatalf("handleMessage() error = %v", err)
			}

			if len(telegramClient.sentMessages) != 1 {
				t.Fatalf("sent messages = %d, want %d", len(telegramClient.sentMessages), 1)
			}

			if got := telegramClient.sentMessages[0].text; got != test.wantMessage {
				t.Fatalf("message = %q, want %q", got, test.wantMessage)
			}
		})
	}
}

func TestHandleMessageNewDeviceRejectsControlCharacters(t *testing.T) {
	telegramClient := &stubTelegramClient{}
	backendClient := &stubBackendClient{}

	b := New(slog.New(slog.NewTextHandler(io.Discard, nil)), telegramClient, backendClient, time.Second)

	err := b.handleMessage(context.Background(), &telegram.Message{
		Text: "/newdevice dad\nlaptop",
		Chat: telegram.Chat{ID: 99},
		From: &telegram.User{ID: 777},
	})
	if err != nil {
		t.Fatalf("handleMessage() error = %v", err)
	}

	if backendClient.createDeviceCalls != 0 {
		t.Fatalf("create device calls = %d, want %d", backendClient.createDeviceCalls, 0)
	}

	if len(telegramClient.sentMessages) != 1 {
		t.Fatalf("sent messages = %d, want %d", len(telegramClient.sentMessages), 1)
	}

	if got := telegramClient.sentMessages[0].text; got != "Device name must be a single line of plain text." {
		t.Fatalf("message = %q, want validation error", got)
	}
}

func TestHandleMessageNewDeviceRejectsLongName(t *testing.T) {
	telegramClient := &stubTelegramClient{}
	backendClient := &stubBackendClient{}

	b := New(slog.New(slog.NewTextHandler(io.Discard, nil)), telegramClient, backendClient, time.Second)

	err := b.handleMessage(context.Background(), &telegram.Message{
		Text: "/newdevice " + strings.Repeat("a", maxDeviceNameLength+1),
		Chat: telegram.Chat{ID: 99},
		From: &telegram.User{ID: 777},
	})
	if err != nil {
		t.Fatalf("handleMessage() error = %v", err)
	}

	if backendClient.createDeviceCalls != 0 {
		t.Fatalf("create device calls = %d, want %d", backendClient.createDeviceCalls, 0)
	}

	if len(telegramClient.sentMessages) != 1 {
		t.Fatalf("sent messages = %d, want %d", len(telegramClient.sentMessages), 1)
	}

	want := "Device name is too long. Use 128 characters or fewer."
	if got := telegramClient.sentMessages[0].text; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

func TestHandleMessageConfigSuccess(t *testing.T) {
	telegramClient := &stubTelegramClient{}
	backendClient := &stubBackendClient{
		resendDeviceConfigResult: &backendapi.ResendDeviceConfigResult{
			Device: backendapi.Device{
				ID:         100,
				Name:       "dad-laptop",
				AssignedIP: "10.67.0.2",
				Status:     "active",
			},
			ClientConfig: "[Interface]\nPrivateKey = private-key\n",
		},
	}

	b := New(slog.New(slog.NewTextHandler(io.Discard, nil)), telegramClient, backendClient, time.Second)

	err := b.handleMessage(context.Background(), &telegram.Message{
		Text: "/config 100",
		Chat: telegram.Chat{ID: 99},
		From: &telegram.User{ID: 777},
	})
	if err != nil {
		t.Fatalf("handleMessage() error = %v", err)
	}

	if backendClient.resendDeviceConfigTelegramUserID != 777 {
		t.Fatalf("telegram user id = %d, want %d", backendClient.resendDeviceConfigTelegramUserID, 777)
	}

	if backendClient.resendDeviceConfigDeviceID != 100 {
		t.Fatalf("device id = %d, want %d", backendClient.resendDeviceConfigDeviceID, 100)
	}

	if len(telegramClient.sentMessages) != 1 {
		t.Fatalf("sent messages = %d, want %d", len(telegramClient.sentMessages), 1)
	}

	want := "Device config rebuilt successfully.\nName: dad-laptop\nIP: 10.67.0.2\n\nClient config:\n[Interface]\nPrivateKey = private-key\n"
	if got := telegramClient.sentMessages[0].text; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

func TestHandleMessageConfigRequiresDeviceID(t *testing.T) {
	telegramClient := &stubTelegramClient{}
	backendClient := &stubBackendClient{}

	b := New(slog.New(slog.NewTextHandler(io.Discard, nil)), telegramClient, backendClient, time.Second)

	err := b.handleMessage(context.Background(), &telegram.Message{
		Text: "/config",
		Chat: telegram.Chat{ID: 99},
		From: &telegram.User{ID: 777},
	})
	if err != nil {
		t.Fatalf("handleMessage() error = %v", err)
	}

	if backendClient.resendDeviceConfigCalls != 0 {
		t.Fatalf("resend config calls = %d, want %d", backendClient.resendDeviceConfigCalls, 0)
	}

	if len(telegramClient.sentMessages) != 1 {
		t.Fatalf("sent messages = %d, want %d", len(telegramClient.sentMessages), 1)
	}

	if got := telegramClient.sentMessages[0].text; got != "Usage: /config <device_id>" {
		t.Fatalf("message = %q, want usage", got)
	}
}

func TestHandleMessageConfigRejectsInvalidDeviceID(t *testing.T) {
	telegramClient := &stubTelegramClient{}
	backendClient := &stubBackendClient{}

	b := New(slog.New(slog.NewTextHandler(io.Discard, nil)), telegramClient, backendClient, time.Second)

	err := b.handleMessage(context.Background(), &telegram.Message{
		Text: "/config abc",
		Chat: telegram.Chat{ID: 99},
		From: &telegram.User{ID: 777},
	})
	if err != nil {
		t.Fatalf("handleMessage() error = %v", err)
	}

	if backendClient.resendDeviceConfigCalls != 0 {
		t.Fatalf("resend config calls = %d, want %d", backendClient.resendDeviceConfigCalls, 0)
	}

	if len(telegramClient.sentMessages) != 1 {
		t.Fatalf("sent messages = %d, want %d", len(telegramClient.sentMessages), 1)
	}

	if got := telegramClient.sentMessages[0].text; got != "Device ID must be a positive number." {
		t.Fatalf("message = %q, want invalid id error", got)
	}
}

func TestHandleMessageConfigMapsBackendErrors(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantMessage string
	}{
		{
			name:        "not found",
			err:         &backendapi.APIError{StatusCode: 404, Message: "not found"},
			wantMessage: "Failed to resend the device config right now. Please try again later.",
		},
		{
			name:        "forbidden",
			err:         &backendapi.APIError{StatusCode: 403, Message: "forbidden"},
			wantMessage: "You are not allowed to access that device config.",
		},
		{
			name:        "revoked",
			err:         &backendapi.APIError{StatusCode: 409, Message: "device is revoked"},
			wantMessage: "Cannot resend config: device is revoked.",
		},
		{
			name:        "other backend error",
			err:         &backendapi.APIError{StatusCode: 500, Message: "internal error"},
			wantMessage: "Failed to resend the device config right now. Please try again later.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			telegramClient := &stubTelegramClient{}
			backendClient := &stubBackendClient{resendDeviceConfigErr: test.err}
			b := New(slog.New(slog.NewTextHandler(io.Discard, nil)), telegramClient, backendClient, time.Second)

			err := b.handleMessage(context.Background(), &telegram.Message{
				Text: "/config 100",
				Chat: telegram.Chat{ID: 99},
				From: &telegram.User{ID: 777},
			})
			if err != nil {
				t.Fatalf("handleMessage() error = %v", err)
			}

			if len(telegramClient.sentMessages) != 1 {
				t.Fatalf("sent messages = %d, want %d", len(telegramClient.sentMessages), 1)
			}

			if got := telegramClient.sentMessages[0].text; got != test.wantMessage {
				t.Fatalf("message = %q, want %q", got, test.wantMessage)
			}
		})
	}
}

func TestHandleMessageRevokeSuccess(t *testing.T) {
	revokedAt := time.Date(2026, 4, 4, 10, 11, 12, 0, time.UTC)
	telegramClient := &stubTelegramClient{}
	backendClient := &stubBackendClient{
		revokeDeviceResult: &backendapi.RevokeDeviceResult{
			Device: backendapi.Device{
				ID:         100,
				Name:       "dad-laptop",
				AssignedIP: "10.67.0.2",
				Status:     "revoked",
				RevokedAt:  &revokedAt,
			},
		},
	}

	b := New(slog.New(slog.NewTextHandler(io.Discard, nil)), telegramClient, backendClient, time.Second)

	err := b.handleMessage(context.Background(), &telegram.Message{
		Text: "/revoke 100",
		Chat: telegram.Chat{ID: 99},
		From: &telegram.User{ID: 777},
	})
	if err != nil {
		t.Fatalf("handleMessage() error = %v", err)
	}

	if backendClient.revokeDeviceTelegramUserID != 777 {
		t.Fatalf("telegram user id = %d, want %d", backendClient.revokeDeviceTelegramUserID, 777)
	}

	if backendClient.revokeDeviceDeviceID != 100 {
		t.Fatalf("device id = %d, want %d", backendClient.revokeDeviceDeviceID, 100)
	}

	if len(telegramClient.sentMessages) != 1 {
		t.Fatalf("sent messages = %d, want %d", len(telegramClient.sentMessages), 1)
	}

	want := "Device revoked successfully.\nName: dad-laptop\nIP: 10.67.0.2\nStatus: revoked\nRevoked: 2026-04-04"
	if got := telegramClient.sentMessages[0].text; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

func TestHandleMessageRevokeRequiresDeviceID(t *testing.T) {
	telegramClient := &stubTelegramClient{}
	backendClient := &stubBackendClient{}

	b := New(slog.New(slog.NewTextHandler(io.Discard, nil)), telegramClient, backendClient, time.Second)

	err := b.handleMessage(context.Background(), &telegram.Message{
		Text: "/revoke",
		Chat: telegram.Chat{ID: 99},
		From: &telegram.User{ID: 777},
	})
	if err != nil {
		t.Fatalf("handleMessage() error = %v", err)
	}

	if backendClient.revokeDeviceCalls != 0 {
		t.Fatalf("revoke device calls = %d, want %d", backendClient.revokeDeviceCalls, 0)
	}

	if len(telegramClient.sentMessages) != 1 {
		t.Fatalf("sent messages = %d, want %d", len(telegramClient.sentMessages), 1)
	}

	if got := telegramClient.sentMessages[0].text; got != "Usage: /revoke <device_id>" {
		t.Fatalf("message = %q, want usage", got)
	}
}

func TestHandleMessageRevokeRejectsInvalidDeviceID(t *testing.T) {
	telegramClient := &stubTelegramClient{}
	backendClient := &stubBackendClient{}

	b := New(slog.New(slog.NewTextHandler(io.Discard, nil)), telegramClient, backendClient, time.Second)

	err := b.handleMessage(context.Background(), &telegram.Message{
		Text: "/revoke abc",
		Chat: telegram.Chat{ID: 99},
		From: &telegram.User{ID: 777},
	})
	if err != nil {
		t.Fatalf("handleMessage() error = %v", err)
	}

	if backendClient.revokeDeviceCalls != 0 {
		t.Fatalf("revoke device calls = %d, want %d", backendClient.revokeDeviceCalls, 0)
	}

	if len(telegramClient.sentMessages) != 1 {
		t.Fatalf("sent messages = %d, want %d", len(telegramClient.sentMessages), 1)
	}

	if got := telegramClient.sentMessages[0].text; got != "Device ID must be a positive number." {
		t.Fatalf("message = %q, want invalid id error", got)
	}
}

func TestHandleMessageRevokeMapsBackendErrors(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantMessage string
	}{
		{
			name:        "forbidden",
			err:         &backendapi.APIError{StatusCode: 403, Message: "forbidden"},
			wantMessage: "You are not allowed to revoke that device.",
		},
		{
			name:        "not found",
			err:         &backendapi.APIError{StatusCode: 404, Message: "not found"},
			wantMessage: "Device not found or unavailable.",
		},
		{
			name:        "already revoked",
			err:         &backendapi.APIError{StatusCode: 409, Message: "device is revoked"},
			wantMessage: "Cannot revoke device: device is revoked.",
		},
		{
			name:        "other backend error",
			err:         &backendapi.APIError{StatusCode: 500, Message: "internal error"},
			wantMessage: "Failed to revoke the device right now. Please try again later.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			telegramClient := &stubTelegramClient{}
			backendClient := &stubBackendClient{revokeDeviceErr: test.err}
			b := New(slog.New(slog.NewTextHandler(io.Discard, nil)), telegramClient, backendClient, time.Second)

			err := b.handleMessage(context.Background(), &telegram.Message{
				Text: "/revoke 100",
				Chat: telegram.Chat{ID: 99},
				From: &telegram.User{ID: 777},
			})
			if err != nil {
				t.Fatalf("handleMessage() error = %v", err)
			}

			if len(telegramClient.sentMessages) != 1 {
				t.Fatalf("sent messages = %d, want %d", len(telegramClient.sentMessages), 1)
			}

			if got := telegramClient.sentMessages[0].text; got != test.wantMessage {
				t.Fatalf("message = %q, want %q", got, test.wantMessage)
			}
		})
	}
}

type stubTelegramClient struct {
	sentMessages []sentMessage
}

type sentMessage struct {
	chatID int64
	text   string
}

func (s *stubTelegramClient) GetUpdates(context.Context, int64, time.Duration) ([]telegram.Update, error) {
	return nil, nil
}

func (s *stubTelegramClient) SendMessage(_ context.Context, chatID int64, text string) error {
	s.sentMessages = append(s.sentMessages, sentMessage{
		chatID: chatID,
		text:   text,
	})
	return nil
}

type stubBackendClient struct {
	createDeviceCalls                int
	createDeviceTelegramUserID       int64
	createDeviceName                 string
	createDeviceResult               *backendapi.CreateDeviceResult
	createDeviceErr                  error
	resendDeviceConfigCalls          int
	resendDeviceConfigTelegramUserID int64
	resendDeviceConfigDeviceID       int64
	resendDeviceConfigResult         *backendapi.ResendDeviceConfigResult
	resendDeviceConfigErr            error
	revokeDeviceCalls                int
	revokeDeviceTelegramUserID       int64
	revokeDeviceDeviceID             int64
	revokeDeviceResult               *backendapi.RevokeDeviceResult
	revokeDeviceErr                  error
}

func (s *stubBackendClient) Health(context.Context) error {
	return nil
}

func (s *stubBackendClient) ListDevices(context.Context, int64) (*backendapi.ListDevicesResult, error) {
	return nil, nil
}

func (s *stubBackendClient) CreateDevice(_ context.Context, telegramUserID int64, name string) (*backendapi.CreateDeviceResult, error) {
	s.createDeviceCalls++
	s.createDeviceTelegramUserID = telegramUserID
	s.createDeviceName = name
	return s.createDeviceResult, s.createDeviceErr
}

func (s *stubBackendClient) ResendDeviceConfig(_ context.Context, telegramUserID, deviceID int64) (*backendapi.ResendDeviceConfigResult, error) {
	s.resendDeviceConfigCalls++
	s.resendDeviceConfigTelegramUserID = telegramUserID
	s.resendDeviceConfigDeviceID = deviceID
	return s.resendDeviceConfigResult, s.resendDeviceConfigErr
}

func (s *stubBackendClient) RevokeDevice(_ context.Context, telegramUserID, deviceID int64) (*backendapi.RevokeDeviceResult, error) {
	s.revokeDeviceCalls++
	s.revokeDeviceTelegramUserID = telegramUserID
	s.revokeDeviceDeviceID = deviceID
	return s.revokeDeviceResult, s.revokeDeviceErr
}
