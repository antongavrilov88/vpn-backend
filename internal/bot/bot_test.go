package bot

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
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

	if len(telegramClient.sentMessages) != 2 {
		t.Fatalf("sent messages = %d, want %d", len(telegramClient.sentMessages), 2)
	}

	if len(telegramClient.sentPhotos) != 1 {
		t.Fatalf("sent documents = %d, want %d", len(telegramClient.sentPhotos), 1)
	}

	if got := telegramClient.sentMessages[0].text; got != "Rick is calibrating the portal gun for dad-laptop.\n\nBuilding your WireGuard config..." {
		t.Fatalf("progress message = %q", got)
	}

	message := telegramClient.sentMessages[1]
	if message.chatID != 99 {
		t.Fatalf("chat id = %d, want %d", message.chatID, 99)
	}

	want := "Device created successfully.\nName: dad-laptop\nIP: 10.67.0.2\nStatus: active\nCreated: 2026-04-02\n\nClient config:\n[Interface]\nPrivateKey = private-key\n"
	if message.text != want {
		t.Fatalf("message = %q, want %q", message.text, want)
	}

	document := telegramClient.sentPhotos[0]
	if document.chatID != 99 {
		t.Fatalf("document chat id = %d, want %d", document.chatID, 99)
	}
	if document.fileName != "dad-laptop-wireguard-qr.png" {
		t.Fatalf("document file name = %q, want %q", document.fileName, "dad-laptop-wireguard-qr.png")
	}
	if document.caption != "WireGuard QR code" {
		t.Fatalf("document caption = %q, want %q", document.caption, "WireGuard QR code")
	}
	if len(document.document) == 0 {
		t.Fatal("document bytes are empty")
	}
}

func TestHandleMessageStartWhenBackendHealthy(t *testing.T) {
	telegramClient := &stubTelegramClient{}
	backendClient := &stubBackendClient{
		accessStatusResult: &backendapi.AccessStatusResult{
			AccessActive:    true,
			CanCreateDevice: true,
			IsLifetime:      true,
		},
	}

	b := New(slog.New(slog.NewTextHandler(io.Discard, nil)), telegramClient, backendClient, time.Second)

	err := b.handleMessage(context.Background(), &telegram.Message{
		Text: "/start",
		Chat: telegram.Chat{ID: 99},
		From: &telegram.User{ID: 777},
	})
	if err != nil {
		t.Fatalf("handleMessage() error = %v", err)
	}

	if backendClient.accessStatusCalls != 1 {
		t.Fatalf("access status calls = %d, want %d", backendClient.accessStatusCalls, 1)
	}

	if len(telegramClient.sentMessages) != 1 {
		t.Fatalf("sent messages = %d, want %d", len(telegramClient.sentMessages), 1)
	}

	want := "Access is active.\n\nClosed beta access is lifetime.\nUse the buttons below to manage devices or create a new one."
	if got := telegramClient.sentMessages[0].text; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}

	assertReplyKeyboardTexts(t, telegramClient.sentMessages[0].replyKB, [][]string{
		{menuDevicesLabel, menuCreateLabel},
		{menuInviteCodeLabel, menuHelpLabel},
	})
}

func TestHandleMessageStartWhenBackendUnavailable(t *testing.T) {
	telegramClient := &stubTelegramClient{}
	backendClient := &stubBackendClient{accessStatusErr: errors.New("timeout")}

	b := New(slog.New(slog.NewTextHandler(io.Discard, nil)), telegramClient, backendClient, time.Second)

	err := b.handleMessage(context.Background(), &telegram.Message{
		Text: "/start",
		Chat: telegram.Chat{ID: 99},
		From: &telegram.User{ID: 777},
	})
	if err != nil {
		t.Fatalf("handleMessage() error = %v", err)
	}

	if len(telegramClient.sentMessages) != 1 {
		t.Fatalf("sent messages = %d, want %d", len(telegramClient.sentMessages), 1)
	}

	want := "VPN bot is connected, but backend API is temporarily unavailable.\n\nPlease try again in a moment."
	if got := telegramClient.sentMessages[0].text; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}

	assertReplyKeyboardTexts(t, telegramClient.sentMessages[0].replyKB, [][]string{
		{menuHelpLabel},
	})
}

func TestHandleMessageStartWhenAccessInactive(t *testing.T) {
	telegramClient := &stubTelegramClient{}
	backendClient := &stubBackendClient{
		accessStatusResult: &backendapi.AccessStatusResult{
			AccessActive:    false,
			CanCreateDevice: false,
			DenialReason:    "invite_code_required",
		},
	}

	b := New(slog.New(slog.NewTextHandler(io.Discard, nil)), telegramClient, backendClient, time.Second)

	err := b.handleMessage(context.Background(), &telegram.Message{
		Text: "/start",
		Chat: telegram.Chat{ID: 99},
		From: &telegram.User{ID: 777},
	})
	if err != nil {
		t.Fatalf("handleMessage() error = %v", err)
	}

	want := "Closed beta access is not active yet.\n\nEnter your invite code to unlock VPN access."
	if got := telegramClient.sentMessages[0].text; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}

	assertReplyKeyboardTexts(t, telegramClient.sentMessages[0].replyKB, [][]string{
		{menuInviteCodeLabel, menuHelpLabel},
	})
}

func TestHandleMessageStartInactiveAcceptsNextPlainTextInviteCode(t *testing.T) {
	telegramClient := &stubTelegramClient{}
	backendClient := &stubBackendClient{
		accessStatusResult: &backendapi.AccessStatusResult{
			AccessActive:    false,
			CanCreateDevice: false,
			DenialReason:    "invite_code_required",
		},
		applyInviteCodeResult: &backendapi.AccessStatusResult{
			AccessActive:    true,
			CanCreateDevice: true,
			IsLifetime:      true,
		},
	}

	b := New(slog.New(slog.NewTextHandler(io.Discard, nil)), telegramClient, backendClient, time.Second)

	if err := b.handleMessage(context.Background(), &telegram.Message{
		Text: "/start",
		Chat: telegram.Chat{ID: 99},
		From: &telegram.User{ID: 777},
	}); err != nil {
		t.Fatalf("handleMessage(/start) error = %v", err)
	}

	if err := b.handleMessage(context.Background(), &telegram.Message{
		Text: "BETA-ANTON",
		Chat: telegram.Chat{ID: 99},
		From: &telegram.User{ID: 777},
	}); err != nil {
		t.Fatalf("handleMessage(invite code) error = %v", err)
	}

	if backendClient.applyInviteCodeCalls != 1 {
		t.Fatalf("apply invite code calls = %d, want %d", backendClient.applyInviteCodeCalls, 1)
	}

	if backendClient.applyInviteCodeCode != "BETA-ANTON" {
		t.Fatalf("invite code = %q, want %q", backendClient.applyInviteCodeCode, "BETA-ANTON")
	}

	if len(telegramClient.sentMessages) != 3 {
		t.Fatalf("sent messages = %d, want %d", len(telegramClient.sentMessages), 3)
	}

	if got := telegramClient.sentMessages[1].text; got != "Checking your invite code..." {
		t.Fatalf("progress message = %q, want invite progress message", got)
	}
}

func TestHandleMessageStartInactiveInvalidInviteCodeKeepsInviteKeyboard(t *testing.T) {
	telegramClient := &stubTelegramClient{}
	backendClient := &stubBackendClient{
		accessStatusResult: &backendapi.AccessStatusResult{
			AccessActive:    false,
			CanCreateDevice: false,
			DenialReason:    "invite_code_required",
		},
	}

	b := New(slog.New(slog.NewTextHandler(io.Discard, nil)), telegramClient, backendClient, time.Second)

	if err := b.handleMessage(context.Background(), &telegram.Message{
		Text: "/start",
		Chat: telegram.Chat{ID: 99},
		From: &telegram.User{ID: 777},
	}); err != nil {
		t.Fatalf("handleMessage(/start) error = %v", err)
	}

	if err := b.handleMessage(context.Background(), &telegram.Message{
		Text: strings.Repeat("a", maxInviteCodeLength+1),
		Chat: telegram.Chat{ID: 99},
		From: &telegram.User{ID: 777},
	}); err != nil {
		t.Fatalf("handleMessage(invite code) error = %v", err)
	}

	if backendClient.applyInviteCodeCalls != 0 {
		t.Fatalf("apply invite code calls = %d, want %d", backendClient.applyInviteCodeCalls, 0)
	}

	if len(telegramClient.sentMessages) != 2 {
		t.Fatalf("sent messages = %d, want %d", len(telegramClient.sentMessages), 2)
	}

	want := "Invite code is too long. Use 128 characters or fewer."
	if got := telegramClient.sentMessages[1].text; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}

	assertReplyKeyboardTexts(t, telegramClient.sentMessages[1].replyKB, [][]string{
		{menuInviteCodeLabel, menuHelpLabel},
	})
}

func TestHandleMessageHelpWhenAccessInactiveShowsInvitePrompt(t *testing.T) {
	telegramClient := &stubTelegramClient{}
	backendClient := &stubBackendClient{
		accessStatusResult: &backendapi.AccessStatusResult{
			AccessActive:    false,
			CanCreateDevice: false,
			DenialReason:    "invite_code_required",
		},
	}

	b := New(slog.New(slog.NewTextHandler(io.Discard, nil)), telegramClient, backendClient, time.Second)

	err := b.handleMessage(context.Background(), &telegram.Message{
		Text: "/help",
		Chat: telegram.Chat{ID: 99},
		From: &telegram.User{ID: 777},
	})
	if err != nil {
		t.Fatalf("handleMessage() error = %v", err)
	}

	if backendClient.accessStatusCalls != 1 {
		t.Fatalf("access status calls = %d, want %d", backendClient.accessStatusCalls, 1)
	}

	if len(telegramClient.sentMessages) != 1 {
		t.Fatalf("sent messages = %d, want %d", len(telegramClient.sentMessages), 1)
	}

	want := "Closed beta access is not active yet.\n\nEnter your invite code to unlock VPN access."
	if got := telegramClient.sentMessages[0].text; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}

	assertReplyKeyboardTexts(t, telegramClient.sentMessages[0].replyKB, [][]string{
		{menuInviteCodeLabel, menuHelpLabel},
	})
}

func TestHandleMessagePromoSuccess(t *testing.T) {
	telegramClient := &stubTelegramClient{}
	backendClient := &stubBackendClient{
		applyInviteCodeResult: &backendapi.AccessStatusResult{
			AccessActive:    true,
			CanCreateDevice: true,
			IsLifetime:      true,
		},
	}

	b := New(slog.New(slog.NewTextHandler(io.Discard, nil)), telegramClient, backendClient, time.Second)

	err := b.handleMessage(context.Background(), &telegram.Message{
		Text: "/promo BETA-ANTON",
		Chat: telegram.Chat{ID: 99},
		From: &telegram.User{ID: 777},
	})
	if err != nil {
		t.Fatalf("handleMessage() error = %v", err)
	}

	if backendClient.applyInviteCodeCalls != 1 {
		t.Fatalf("apply invite code calls = %d, want %d", backendClient.applyInviteCodeCalls, 1)
	}

	if backendClient.applyInviteCodeTelegramUserID != 777 {
		t.Fatalf("telegram user id = %d, want %d", backendClient.applyInviteCodeTelegramUserID, 777)
	}

	if backendClient.applyInviteCodeCode != "BETA-ANTON" {
		t.Fatalf("invite code = %q, want %q", backendClient.applyInviteCodeCode, "BETA-ANTON")
	}

	if len(telegramClient.sentMessages) != 2 {
		t.Fatalf("sent messages = %d, want %d", len(telegramClient.sentMessages), 2)
	}

	if got := telegramClient.sentMessages[0].text; got != "Checking your invite code..." {
		t.Fatalf("progress message = %q", got)
	}

	want := "Invite code accepted.\n\nLifetime beta access is now active.\nYou can create your first device."
	if got := telegramClient.sentMessages[1].text; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

func TestHandleMessagePromoInvalidCode(t *testing.T) {
	telegramClient := &stubTelegramClient{}
	backendClient := &stubBackendClient{
		applyInviteCodeErr: &backendapi.APIError{StatusCode: 404, Message: "not found"},
	}

	b := New(slog.New(slog.NewTextHandler(io.Discard, nil)), telegramClient, backendClient, time.Second)

	err := b.handleMessage(context.Background(), &telegram.Message{
		Text: "/promo UNKNOWN",
		Chat: telegram.Chat{ID: 99},
		From: &telegram.User{ID: 777},
	})
	if err != nil {
		t.Fatalf("handleMessage() error = %v", err)
	}

	if len(telegramClient.sentMessages) != 2 {
		t.Fatalf("sent messages = %d, want %d", len(telegramClient.sentMessages), 2)
	}

	if got := telegramClient.sentMessages[1].text; got != "That invite code is not valid." {
		t.Fatalf("message = %q, want invalid code message", got)
	}

	assertReplyKeyboardTexts(t, telegramClient.sentMessages[1].replyKB, [][]string{
		{menuInviteCodeLabel, menuHelpLabel},
	})
}

func TestHandleMessageDevicesSuccess(t *testing.T) {
	telegramClient := &stubTelegramClient{}
	backendClient := &stubBackendClient{
		listDevicesResult: &backendapi.ListDevicesResult{
			Devices: []backendapi.Device{
				{
					ID:         100,
					Name:       "dad-laptop",
					AssignedIP: "10.68.0.2",
					Status:     "active",
					CreatedAt:  time.Date(2026, 4, 15, 10, 11, 12, 0, time.UTC),
				},
				{
					ID:         101,
					Name:       "mom-phone",
					AssignedIP: "10.68.0.3",
					Status:     "revoked",
					CreatedAt:  time.Date(2026, 4, 14, 10, 11, 12, 0, time.UTC),
					RevokedAt:  timePtr(time.Date(2026, 4, 16, 10, 11, 12, 0, time.UTC)),
				},
			},
		},
	}

	b := New(slog.New(slog.NewTextHandler(io.Discard, nil)), telegramClient, backendClient, time.Second)

	err := b.handleMessage(context.Background(), &telegram.Message{
		Text: "/devices",
		Chat: telegram.Chat{ID: 99},
		From: &telegram.User{ID: 777},
	})
	if err != nil {
		t.Fatalf("handleMessage() error = %v", err)
	}

	if backendClient.listDevicesCalls != 1 {
		t.Fatalf("list device calls = %d, want %d", backendClient.listDevicesCalls, 1)
	}

	if backendClient.listDevicesTelegramUserID != 777 {
		t.Fatalf("telegram user id = %d, want %d", backendClient.listDevicesTelegramUserID, 777)
	}

	if len(telegramClient.sentMessages) != 2 {
		t.Fatalf("sent messages = %d, want %d", len(telegramClient.sentMessages), 2)
	}

	wantSummary := "Your devices:\n\nTap a button under any active device to get the config, show the QR, or revoke it."
	if got := telegramClient.sentMessages[0].text; got != wantSummary {
		t.Fatalf("summary message = %q, want %q", got, wantSummary)
	}

	wantActive := "Name: dad-laptop\nStatus: Active\nIP: 10.68.0.2\nCreated: 2026-04-15"
	if got := telegramClient.sentMessages[1].text; got != wantActive {
		t.Fatalf("active device message = %q, want %q", got, wantActive)
	}

	keyboard := telegramClient.sentMessages[1].keyboard
	if keyboard == nil {
		t.Fatal("active device keyboard = nil, want inline actions")
	}

	if len(keyboard.InlineKeyboard) != 2 {
		t.Fatalf("keyboard rows = %d, want %d", len(keyboard.InlineKeyboard), 2)
	}

	if got := keyboard.InlineKeyboard[0][0].CallbackData; got != "device:config:100" {
		t.Fatalf("config callback data = %q, want %q", got, "device:config:100")
	}

	if got := keyboard.InlineKeyboard[0][1].CallbackData; got != "device:qr:100" {
		t.Fatalf("qr callback data = %q, want %q", got, "device:qr:100")
	}

	if got := keyboard.InlineKeyboard[1][0].CallbackData; got != "device:revoke:100" {
		t.Fatalf("revoke callback data = %q, want %q", got, "device:revoke:100")
	}

	if backendClient.listDevicesResult.Devices[1].Status != "revoked" {
		t.Fatal("test fixture no longer includes a revoked device")
	}
}

func TestHandleMessageDevicesEmptyList(t *testing.T) {
	telegramClient := &stubTelegramClient{}
	backendClient := &stubBackendClient{
		listDevicesResult: &backendapi.ListDevicesResult{},
	}

	b := New(slog.New(slog.NewTextHandler(io.Discard, nil)), telegramClient, backendClient, time.Second)

	err := b.handleMessage(context.Background(), &telegram.Message{
		Text: "/devices",
		Chat: telegram.Chat{ID: 99},
		From: &telegram.User{ID: 777},
	})
	if err != nil {
		t.Fatalf("handleMessage() error = %v", err)
	}

	if len(telegramClient.sentMessages) != 1 {
		t.Fatalf("sent messages = %d, want %d", len(telegramClient.sentMessages), 1)
	}

	want := "You have no devices yet.\n\nTap Create device below to add one."
	if got := telegramClient.sentMessages[0].text; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

func TestHandleMessageDevicesWhenOnlyRevokedDevicesRemain(t *testing.T) {
	telegramClient := &stubTelegramClient{}
	backendClient := &stubBackendClient{
		listDevicesResult: &backendapi.ListDevicesResult{
			Devices: []backendapi.Device{
				{
					ID:         101,
					Name:       "mom-phone",
					AssignedIP: "10.68.0.3",
					Status:     "revoked",
					CreatedAt:  time.Date(2026, 4, 14, 10, 11, 12, 0, time.UTC),
					RevokedAt:  timePtr(time.Date(2026, 4, 16, 10, 11, 12, 0, time.UTC)),
				},
			},
		},
	}

	b := New(slog.New(slog.NewTextHandler(io.Discard, nil)), telegramClient, backendClient, time.Second)

	err := b.handleMessage(context.Background(), &telegram.Message{
		Text: "/devices",
		Chat: telegram.Chat{ID: 99},
		From: &telegram.User{ID: 777},
	})
	if err != nil {
		t.Fatalf("handleMessage() error = %v", err)
	}

	if len(telegramClient.sentMessages) != 1 {
		t.Fatalf("sent messages = %d, want %d", len(telegramClient.sentMessages), 1)
	}

	want := "You have no active devices right now.\n\nTap Create device below to add a new one."
	if got := telegramClient.sentMessages[0].text; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

func TestHandleMessageDevicesWhenUserNotLinked(t *testing.T) {
	telegramClient := &stubTelegramClient{}
	backendClient := &stubBackendClient{listDevicesErr: backendapi.ErrNotFound}

	b := New(slog.New(slog.NewTextHandler(io.Discard, nil)), telegramClient, backendClient, time.Second)

	err := b.handleMessage(context.Background(), &telegram.Message{
		Text: "/devices",
		Chat: telegram.Chat{ID: 99},
		From: &telegram.User{ID: 777},
	})
	if err != nil {
		t.Fatalf("handleMessage() error = %v", err)
	}

	if len(telegramClient.sentMessages) != 1 {
		t.Fatalf("sent messages = %d, want %d", len(telegramClient.sentMessages), 1)
	}

	if got := telegramClient.sentMessages[0].text; got != "You are not linked to a VPN user yet." {
		t.Fatalf("message = %q, want not linked message", got)
	}
}

func TestHandleMessageDevicesWhenAccessInactiveDoesNotListDevices(t *testing.T) {
	telegramClient := &stubTelegramClient{}
	backendClient := &stubBackendClient{
		accessStatusResult: &backendapi.AccessStatusResult{
			AccessActive:    false,
			CanCreateDevice: false,
			DenialReason:    "invite_code_required",
		},
	}

	b := New(slog.New(slog.NewTextHandler(io.Discard, nil)), telegramClient, backendClient, time.Second)

	err := b.handleMessage(context.Background(), &telegram.Message{
		Text: "/devices",
		Chat: telegram.Chat{ID: 99},
		From: &telegram.User{ID: 777},
	})
	if err != nil {
		t.Fatalf("handleMessage() error = %v", err)
	}

	if backendClient.listDevicesCalls != 0 {
		t.Fatalf("list device calls = %d, want %d", backendClient.listDevicesCalls, 0)
	}

	if len(telegramClient.sentMessages) != 1 {
		t.Fatalf("sent messages = %d, want %d", len(telegramClient.sentMessages), 1)
	}

	want := "Closed beta access is not active yet.\n\nEnter your invite code to unlock VPN access."
	if got := telegramClient.sentMessages[0].text; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}

	assertReplyKeyboardTexts(t, telegramClient.sentMessages[0].replyKB, [][]string{
		{menuInviteCodeLabel, menuHelpLabel},
	})
}

func TestHandleMessageDevicesWhenAccessStatusUnavailableShowsHelpOnlyKeyboard(t *testing.T) {
	telegramClient := &stubTelegramClient{}
	backendClient := &stubBackendClient{accessStatusErr: errors.New("timeout")}

	b := New(slog.New(slog.NewTextHandler(io.Discard, nil)), telegramClient, backendClient, time.Second)

	err := b.handleMessage(context.Background(), &telegram.Message{
		Text: "/devices",
		Chat: telegram.Chat{ID: 99},
		From: &telegram.User{ID: 777},
	})
	if err != nil {
		t.Fatalf("handleMessage() error = %v", err)
	}

	if backendClient.listDevicesCalls != 0 {
		t.Fatalf("list device calls = %d, want %d", backendClient.listDevicesCalls, 0)
	}

	if len(telegramClient.sentMessages) != 1 {
		t.Fatalf("sent messages = %d, want %d", len(telegramClient.sentMessages), 1)
	}

	want := "VPN bot is connected, but backend API is temporarily unavailable.\n\nPlease try again in a moment."
	if got := telegramClient.sentMessages[0].text; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}

	assertReplyKeyboardTexts(t, telegramClient.sentMessages[0].replyKB, [][]string{
		{menuHelpLabel},
	})
}

func TestHandleCallbackQueryConfigSuccess(t *testing.T) {
	telegramClient := &stubTelegramClient{}
	backendClient := &stubBackendClient{
		resendDeviceConfigResult: &backendapi.ResendDeviceConfigResult{
			Device: backendapi.Device{
				ID:         100,
				Name:       "dad-laptop",
				AssignedIP: "10.68.0.2",
				Status:     "active",
			},
			ClientConfig: "[Interface]\nPrivateKey = private-key\n",
		},
	}

	b := New(slog.New(slog.NewTextHandler(io.Discard, nil)), telegramClient, backendClient, time.Second)

	err := b.handleCallbackQuery(context.Background(), &telegram.CallbackQuery{
		ID:   "cb-1",
		Data: "device:config:100",
		From: &telegram.User{ID: 777},
		Message: &telegram.Message{
			Chat: telegram.Chat{ID: 99},
		},
	})
	if err != nil {
		t.Fatalf("handleCallbackQuery() error = %v", err)
	}

	if backendClient.resendDeviceConfigTelegramUserID != 777 || backendClient.resendDeviceConfigDeviceID != 100 {
		t.Fatalf("backend resend input = (%d, %d), want (%d, %d)", backendClient.resendDeviceConfigTelegramUserID, backendClient.resendDeviceConfigDeviceID, 777, 100)
	}

	if len(telegramClient.callbackAnswers) != 1 {
		t.Fatalf("callback answers = %d, want %d", len(telegramClient.callbackAnswers), 1)
	}

	if got := telegramClient.callbackAnswers[0].text; got != "Morty, hold still. Rebuilding that config." {
		t.Fatalf("callback answer = %q", got)
	}

	if len(telegramClient.sentMessages) != 1 {
		t.Fatalf("sent messages = %d, want %d", len(telegramClient.sentMessages), 1)
	}

	if len(telegramClient.sentPhotos) != 0 {
		t.Fatalf("sent documents = %d, want %d", len(telegramClient.sentPhotos), 0)
	}
}

func TestHandleCallbackQueryShowQRSuccess(t *testing.T) {
	telegramClient := &stubTelegramClient{}
	backendClient := &stubBackendClient{
		resendDeviceConfigResult: &backendapi.ResendDeviceConfigResult{
			Device: backendapi.Device{
				ID:         100,
				Name:       "dad-laptop",
				AssignedIP: "10.68.0.2",
				Status:     "active",
			},
			ClientConfig: "[Interface]\nPrivateKey = private-key\n",
		},
	}

	b := New(slog.New(slog.NewTextHandler(io.Discard, nil)), telegramClient, backendClient, time.Second)

	err := b.handleCallbackQuery(context.Background(), &telegram.CallbackQuery{
		ID:   "cb-2",
		Data: "device:qr:100",
		From: &telegram.User{ID: 777},
		Message: &telegram.Message{
			Chat: telegram.Chat{ID: 99},
		},
	})
	if err != nil {
		t.Fatalf("handleCallbackQuery() error = %v", err)
	}

	if len(telegramClient.callbackAnswers) != 1 {
		t.Fatalf("callback answers = %d, want %d", len(telegramClient.callbackAnswers), 1)
	}

	if got := telegramClient.callbackAnswers[0].text; got != "Opening a tiny portal and turning it into a QR code." {
		t.Fatalf("callback answer = %q", got)
	}

	if len(telegramClient.sentMessages) != 0 {
		t.Fatalf("sent messages = %d, want %d", len(telegramClient.sentMessages), 0)
	}

	if len(telegramClient.sentPhotos) != 1 {
		t.Fatalf("sent documents = %d, want %d", len(telegramClient.sentPhotos), 1)
	}
}

func TestHandleCallbackQueryRevokeSuccess(t *testing.T) {
	revokedAt := time.Date(2026, 4, 16, 10, 11, 12, 0, time.UTC)
	telegramClient := &stubTelegramClient{}
	backendClient := &stubBackendClient{
		revokeDeviceResult: &backendapi.RevokeDeviceResult{
			Device: backendapi.Device{
				ID:         100,
				Name:       "dad-laptop",
				AssignedIP: "10.68.0.2",
				Status:     "revoked",
				RevokedAt:  &revokedAt,
			},
		},
	}

	b := New(slog.New(slog.NewTextHandler(io.Discard, nil)), telegramClient, backendClient, time.Second)

	err := b.handleCallbackQuery(context.Background(), &telegram.CallbackQuery{
		ID:   "cb-3",
		Data: "device:revoke:100",
		From: &telegram.User{ID: 777},
		Message: &telegram.Message{
			Chat: telegram.Chat{ID: 99},
		},
	})
	if err != nil {
		t.Fatalf("handleCallbackQuery() error = %v", err)
	}

	if len(telegramClient.callbackAnswers) != 1 {
		t.Fatalf("callback answers = %d, want %d", len(telegramClient.callbackAnswers), 1)
	}

	if got := telegramClient.callbackAnswers[0].text; got != "Rick is closing this portal and cleaning up the timeline." {
		t.Fatalf("callback answer = %q", got)
	}

	if len(telegramClient.sentMessages) != 1 {
		t.Fatalf("sent messages = %d, want %d", len(telegramClient.sentMessages), 1)
	}

	want := "Device revoked successfully.\nName: dad-laptop\nIP: 10.68.0.2\nStatus: revoked\nRevoked: 2026-04-16"
	if got := telegramClient.sentMessages[0].text; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

func TestHandleCallbackQueryInvalidAction(t *testing.T) {
	telegramClient := &stubTelegramClient{}
	backendClient := &stubBackendClient{}

	b := New(slog.New(slog.NewTextHandler(io.Discard, nil)), telegramClient, backendClient, time.Second)

	err := b.handleCallbackQuery(context.Background(), &telegram.CallbackQuery{
		ID:   "cb-4",
		Data: "nope",
		From: &telegram.User{ID: 777},
		Message: &telegram.Message{
			Chat: telegram.Chat{ID: 99},
		},
	})
	if err != nil {
		t.Fatalf("handleCallbackQuery() error = %v", err)
	}

	if len(telegramClient.callbackAnswers) != 1 {
		t.Fatalf("callback answers = %d, want %d", len(telegramClient.callbackAnswers), 1)
	}

	if got := telegramClient.callbackAnswers[0].text; got != "This action is no longer available." {
		t.Fatalf("callback answer = %q, want %q", got, "This action is no longer available.")
	}

	if backendClient.resendDeviceConfigCalls != 0 || backendClient.revokeDeviceCalls != 0 {
		t.Fatal("backend should not be called for invalid callback data")
	}
}

func TestHandleCallbackQueryConfigWhenAccessInactiveShowsInvitePrompt(t *testing.T) {
	telegramClient := &stubTelegramClient{}
	backendClient := &stubBackendClient{
		accessStatusResult: &backendapi.AccessStatusResult{
			AccessActive:    false,
			CanCreateDevice: false,
			DenialReason:    "invite_code_required",
		},
	}

	b := New(slog.New(slog.NewTextHandler(io.Discard, nil)), telegramClient, backendClient, time.Second)

	err := b.handleCallbackQuery(context.Background(), &telegram.CallbackQuery{
		ID:   "cb-inactive",
		Data: "device:config:100",
		From: &telegram.User{ID: 777},
		Message: &telegram.Message{
			Chat: telegram.Chat{ID: 99},
		},
	})
	if err != nil {
		t.Fatalf("handleCallbackQuery() error = %v", err)
	}

	if backendClient.resendDeviceConfigCalls != 0 {
		t.Fatalf("resend config calls = %d, want %d", backendClient.resendDeviceConfigCalls, 0)
	}

	if len(telegramClient.callbackAnswers) != 1 {
		t.Fatalf("callback answers = %d, want %d", len(telegramClient.callbackAnswers), 1)
	}

	if got := telegramClient.callbackAnswers[0].text; got != "Enter an invite code first." {
		t.Fatalf("callback answer = %q, want %q", got, "Enter an invite code first.")
	}

	if len(telegramClient.sentMessages) != 1 {
		t.Fatalf("sent messages = %d, want %d", len(telegramClient.sentMessages), 1)
	}

	want := "Closed beta access is not active yet.\n\nEnter your invite code to unlock VPN access."
	if got := telegramClient.sentMessages[0].text; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}

	assertReplyKeyboardTexts(t, telegramClient.sentMessages[0].replyKB, [][]string{
		{menuInviteCodeLabel, menuHelpLabel},
	})
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

	want := "Let's spin up a new device.\n\nHow should we call it?\nExamples: iphone, macbook, dad-phone"
	if got := telegramClient.sentMessages[0].text; got != want {
		t.Fatalf("message = %q, want prompt", got)
	}
}

func TestHandleMessageNewDeviceWhenAccessInactiveDoesNotPromptForName(t *testing.T) {
	telegramClient := &stubTelegramClient{}
	backendClient := &stubBackendClient{
		accessStatusResult: &backendapi.AccessStatusResult{
			AccessActive:    false,
			CanCreateDevice: false,
			DenialReason:    "invite_code_required",
		},
	}

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

	want := "Closed beta access is not active yet.\n\nEnter your invite code to unlock VPN access."
	if got := telegramClient.sentMessages[0].text; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}

	assertReplyKeyboardTexts(t, telegramClient.sentMessages[0].replyKB, [][]string{
		{menuInviteCodeLabel, menuHelpLabel},
	})
}

func TestHandleMessageNewDeviceTwoStepFlow(t *testing.T) {
	telegramClient := &stubTelegramClient{}
	backendClient := &stubBackendClient{
		createDeviceResult: &backendapi.CreateDeviceResult{
			Device: backendapi.Device{
				ID:         100,
				Name:       "iphone",
				AssignedIP: "10.68.0.2",
				Status:     "active",
				CreatedAt:  time.Date(2026, 4, 15, 10, 11, 12, 0, time.UTC),
			},
			ClientConfig: "[Interface]\nPrivateKey = private-key\n",
		},
	}

	b := New(slog.New(slog.NewTextHandler(io.Discard, nil)), telegramClient, backendClient, time.Second)

	err := b.handleMessage(context.Background(), &telegram.Message{
		Text: "/newdevice",
		Chat: telegram.Chat{ID: 99},
		From: &telegram.User{ID: 777},
	})
	if err != nil {
		t.Fatalf("prompt handleMessage() error = %v", err)
	}

	err = b.handleMessage(context.Background(), &telegram.Message{
		Text: "iphone",
		Chat: telegram.Chat{ID: 99},
		From: &telegram.User{ID: 777},
	})
	if err != nil {
		t.Fatalf("name handleMessage() error = %v", err)
	}

	if backendClient.createDeviceCalls != 1 {
		t.Fatalf("create device calls = %d, want %d", backendClient.createDeviceCalls, 1)
	}

	if backendClient.createDeviceName != "iphone" {
		t.Fatalf("device name = %q, want %q", backendClient.createDeviceName, "iphone")
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
			wantMessage: "Access is not active yet.\n\nTap Enter invite code below or send /promo <invite_code>.",
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

			if len(telegramClient.sentMessages) != 2 {
				t.Fatalf("sent messages = %d, want %d", len(telegramClient.sentMessages), 2)
			}

			if got := telegramClient.sentMessages[1].text; got != test.wantMessage {
				t.Fatalf("message = %q, want %q", got, test.wantMessage)
			}

			if test.name == "forbidden" {
				assertReplyKeyboardTexts(t, telegramClient.sentMessages[1].replyKB, [][]string{
					{menuInviteCodeLabel, menuHelpLabel},
				})
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

	if len(telegramClient.sentPhotos) != 1 {
		t.Fatalf("sent documents = %d, want %d", len(telegramClient.sentPhotos), 1)
	}

	want := "Device config rebuilt successfully.\nName: dad-laptop\nIP: 10.67.0.2\n\nClient config:\n[Interface]\nPrivateKey = private-key\n"
	if got := telegramClient.sentMessages[0].text; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}

	document := telegramClient.sentPhotos[0]
	if document.fileName != "dad-laptop-wireguard-qr.png" {
		t.Fatalf("document file name = %q, want %q", document.fileName, "dad-laptop-wireguard-qr.png")
	}
	if len(document.document) == 0 {
		t.Fatal("document bytes are empty")
	}
}

func TestHandleMessageConfigFallsBackWhenQRDeliveryFails(t *testing.T) {
	telegramClient := &stubTelegramClient{sendPhotoErr: errors.New("telegram sendDocument failed")}
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

	if len(telegramClient.sentMessages) != 2 {
		t.Fatalf("sent messages = %d, want %d", len(telegramClient.sentMessages), 2)
	}

	if len(telegramClient.sentPhotos) != 0 {
		t.Fatalf("sent documents = %d, want %d", len(telegramClient.sentPhotos), 0)
	}

	want := "QR code is unavailable right now. Use the config text above."
	if got := telegramClient.sentMessages[1].text; got != want {
		t.Fatalf("fallback message = %q, want %q", got, want)
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

func TestSanitizeLoggedInputRedactsInviteCodeCommand(t *testing.T) {
	got := sanitizeLoggedInput("/promo BETA-ANTON", "/promo", false)
	want := "/promo <invite_code_redacted>"
	if got != want {
		t.Fatalf("sanitizeLoggedInput() = %q, want %q", got, want)
	}
}

func TestSanitizeLoggedInputRedactsPendingInviteCodeEntry(t *testing.T) {
	got := sanitizeLoggedInput("BETA-ANTON", "BETA-ANTON", true)
	want := "<invite_code_redacted>"
	if got != want {
		t.Fatalf("sanitizeLoggedInput() = %q, want %q", got, want)
	}
}

type stubTelegramClient struct {
	sentMessages     []sentMessage
	sentPhotos       []sentPhoto
	callbackAnswers  []callbackAnswer
	sendPhotoErr     error
	setCommands      []telegram.BotCommand
	setCommandsCalls int
	setCommandScopes []*telegram.BotCommandScope
}

type sentMessage struct {
	chatID   int64
	text     string
	keyboard *telegram.InlineKeyboardMarkup
	replyKB  *telegram.ReplyKeyboardMarkup
}

type sentPhoto struct {
	chatID   int64
	fileName string
	document []byte
	caption  string
}

type callbackAnswer struct {
	id   string
	text string
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

func (s *stubTelegramClient) SendMessageWithInlineKeyboard(_ context.Context, chatID int64, text string, keyboard telegram.InlineKeyboardMarkup) error {
	keyboardCopy := keyboard
	s.sentMessages = append(s.sentMessages, sentMessage{
		chatID:   chatID,
		text:     text,
		keyboard: &keyboardCopy,
	})
	return nil
}

func (s *stubTelegramClient) SendMessageWithReplyKeyboard(_ context.Context, chatID int64, text string, keyboard telegram.ReplyKeyboardMarkup) error {
	keyboardCopy := keyboard
	s.sentMessages = append(s.sentMessages, sentMessage{
		chatID:  chatID,
		text:    text,
		replyKB: &keyboardCopy,
	})
	return nil
}

func (s *stubTelegramClient) SendDocument(_ context.Context, chatID int64, fileName string, document []byte, caption string) error {
	if s.sendPhotoErr != nil {
		return s.sendPhotoErr
	}

	s.sentPhotos = append(s.sentPhotos, sentPhoto{
		chatID:   chatID,
		fileName: fileName,
		document: append([]byte(nil), document...),
		caption:  caption,
	})
	return nil
}

func (s *stubTelegramClient) AnswerCallbackQuery(_ context.Context, callbackQueryID, text string) error {
	s.callbackAnswers = append(s.callbackAnswers, callbackAnswer{
		id:   callbackQueryID,
		text: text,
	})
	return nil
}

func (s *stubTelegramClient) SetCommands(_ context.Context, commands []telegram.BotCommand) error {
	s.setCommandsCalls++
	s.setCommands = append([]telegram.BotCommand(nil), commands...)
	return nil
}

func (s *stubTelegramClient) SetCommandsWithScope(_ context.Context, commands []telegram.BotCommand, scope *telegram.BotCommandScope) error {
	s.setCommandsCalls++
	s.setCommands = append([]telegram.BotCommand(nil), commands...)
	if scope == nil {
		s.setCommandScopes = append(s.setCommandScopes, nil)
		return nil
	}

	scopeCopy := *scope
	s.setCommandScopes = append(s.setCommandScopes, &scopeCopy)
	return nil
}

type stubBackendClient struct {
	healthCalls                      int
	healthErr                        error
	accessStatusCalls                int
	accessStatusTelegramUserID       int64
	accessStatusResult               *backendapi.AccessStatusResult
	accessStatusErr                  error
	applyInviteCodeCalls             int
	applyInviteCodeTelegramUserID    int64
	applyInviteCodeCode              string
	applyInviteCodeResult            *backendapi.AccessStatusResult
	applyInviteCodeErr               error
	listDevicesCalls                 int
	listDevicesTelegramUserID        int64
	listDevicesResult                *backendapi.ListDevicesResult
	listDevicesErr                   error
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
	s.healthCalls++
	return s.healthErr
}

func (s *stubBackendClient) GetAccessStatus(_ context.Context, telegramUserID int64) (*backendapi.AccessStatusResult, error) {
	s.accessStatusCalls++
	s.accessStatusTelegramUserID = telegramUserID
	if s.accessStatusErr == nil && s.accessStatusResult == nil {
		return &backendapi.AccessStatusResult{
			AccessActive:    true,
			CanCreateDevice: true,
			IsLifetime:      true,
		}, nil
	}
	return s.accessStatusResult, s.accessStatusErr
}

func (s *stubBackendClient) ApplyInviteCode(_ context.Context, telegramUserID int64, code string) (*backendapi.AccessStatusResult, error) {
	s.applyInviteCodeCalls++
	s.applyInviteCodeTelegramUserID = telegramUserID
	s.applyInviteCodeCode = code
	return s.applyInviteCodeResult, s.applyInviteCodeErr
}

func (s *stubBackendClient) ListDevices(_ context.Context, telegramUserID int64) (*backendapi.ListDevicesResult, error) {
	s.listDevicesCalls++
	s.listDevicesTelegramUserID = telegramUserID
	return s.listDevicesResult, s.listDevicesErr
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

func timePtr(value time.Time) *time.Time {
	return &value
}

func TestClosedBetaBotCommands(t *testing.T) {
	got := closedBetaBotCommands()
	want := []telegram.BotCommand{
		{Command: "start", Description: "Check access and begin"},
		{Command: "help", Description: "Show help"},
		{Command: "promo", Description: "Enter invite code"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("closedBetaBotCommands() = %#v, want %#v", got, want)
	}
}

func TestSyncCommandsUsesClosedBetaCommandSet(t *testing.T) {
	telegramClient := &stubTelegramClient{}
	backendClient := &stubBackendClient{}
	b := New(slog.New(slog.NewTextHandler(io.Discard, nil)), telegramClient, backendClient, time.Second)

	if err := b.syncCommands(context.Background()); err != nil {
		t.Fatalf("syncCommands() error = %v", err)
	}

	if telegramClient.setCommandsCalls != 2 {
		t.Fatalf("set commands calls = %d, want %d", telegramClient.setCommandsCalls, 2)
	}

	if !reflect.DeepEqual(telegramClient.setCommands, closedBetaBotCommands()) {
		t.Fatalf("set commands = %#v, want %#v", telegramClient.setCommands, closedBetaBotCommands())
	}

	if len(telegramClient.setCommandScopes) != 1 {
		t.Fatalf("set command scopes = %#v, want one scoped sync", telegramClient.setCommandScopes)
	}

	if telegramClient.setCommandScopes[0] == nil || telegramClient.setCommandScopes[0].Type != telegram.BotCommandScopeAllPrivateChats {
		t.Fatalf("scoped command sync = %#v, want all_private_chats", telegramClient.setCommandScopes[0])
	}
}

func assertReplyKeyboardTexts(t *testing.T, keyboard *telegram.ReplyKeyboardMarkup, want [][]string) {
	t.Helper()

	if keyboard == nil {
		t.Fatal("reply keyboard = nil, want keyboard")
	}

	got := replyKeyboardTexts(keyboard)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("reply keyboard = %#v, want %#v", got, want)
	}
}

func replyKeyboardTexts(keyboard *telegram.ReplyKeyboardMarkup) [][]string {
	if keyboard == nil {
		return nil
	}

	rows := make([][]string, 0, len(keyboard.Keyboard))
	for _, row := range keyboard.Keyboard {
		texts := make([]string, 0, len(row))
		for _, button := range row {
			texts = append(texts, button.Text)
		}
		rows = append(rows, texts)
	}

	return rows
}
