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
	createDeviceCalls          int
	createDeviceTelegramUserID int64
	createDeviceName           string
	createDeviceResult         *backendapi.CreateDeviceResult
	createDeviceErr            error
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
