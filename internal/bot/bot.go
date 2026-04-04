package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"vpn-backend/internal/infra/backendapi"
	"vpn-backend/internal/infra/telegram"
)

type Bot struct {
	logger      *slog.Logger
	telegram    telegramClient
	backend     backendClient
	pollTimeout time.Duration
}

const maxDeviceNameLength = 128

type telegramClient interface {
	GetUpdates(ctx context.Context, offset int64, timeout time.Duration) ([]telegram.Update, error)
	SendMessage(ctx context.Context, chatID int64, text string) error
}

type backendClient interface {
	backendapi.HealthChecker
	ListDevices(ctx context.Context, telegramUserID int64) (*backendapi.ListDevicesResult, error)
	CreateDevice(ctx context.Context, telegramUserID int64, name string) (*backendapi.CreateDeviceResult, error)
	ResendDeviceConfig(ctx context.Context, telegramUserID, deviceID int64) (*backendapi.ResendDeviceConfigResult, error)
	RevokeDevice(ctx context.Context, telegramUserID, deviceID int64) (*backendapi.RevokeDeviceResult, error)
}

func New(logger *slog.Logger, telegramClient telegramClient, backendClient backendClient, pollTimeout time.Duration) *Bot {
	return &Bot{
		logger:      logger,
		telegram:    telegramClient,
		backend:     backendClient,
		pollTimeout: pollTimeout,
	}
}

func (b *Bot) Run(ctx context.Context) error {
	if err := b.checkBackend(ctx); err != nil {
		b.logger.Warn("backend api is unavailable at startup", "error", err)
	}

	b.logger.Info("telegram bot started")

	var offset int64
	for {
		updates, err := b.telegram.GetUpdates(ctx, offset, b.pollTimeout)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
				return nil
			}

			b.logger.Error("poll telegram updates", "error", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(2 * time.Second):
			}
			continue
		}

		for _, update := range updates {
			if update.Message == nil || update.Message.Text == "" {
				offset = update.UpdateID + 1
				continue
			}

			if err := b.handleMessage(ctx, update.Message); err != nil {
				b.logger.Error("handle telegram message", "error", err)
				continue
			}

			offset = update.UpdateID + 1
		}
	}
}

func (b *Bot) handleMessage(ctx context.Context, message *telegram.Message) error {
	fields := strings.Fields(message.Text)
	if len(fields) == 0 {
		return nil
	}

	switch fields[0] {
	case "/start":
		return b.telegram.SendMessage(ctx, message.Chat.ID, b.startMessage(ctx))
	case "/help":
		return b.telegram.SendMessage(ctx, message.Chat.ID, helpMessage())
	case "/devices":
		return b.handleDevices(ctx, message)
	case "/newdevice":
		return b.handleNewDevice(ctx, message, strings.TrimSpace(strings.TrimPrefix(message.Text, fields[0])))
	case "/config":
		return b.handleConfig(ctx, message, strings.TrimSpace(strings.TrimPrefix(message.Text, fields[0])))
	case "/revoke":
		return b.handleRevoke(ctx, message, strings.TrimSpace(strings.TrimPrefix(message.Text, fields[0])))
	default:
		return nil
	}
}

func (b *Bot) startMessage(ctx context.Context) string {
	if err := b.checkBackend(ctx); err != nil {
		b.logger.Warn("backend api is unavailable for /start", "error", err)
		return "VPN bot is connected, but backend API is temporarily unavailable.\n\nUse /help to see available commands."
	}

	return "VPN bot is connected and backend API is reachable.\n\nUse /help to see available commands."
}

func helpMessage() string {
	return "Available commands:\n/start - show welcome message\n/help - show this help\n/devices - show your devices\n/newdevice <device_name> - create a new device\n/config <device_id> - resend config for a device\n/revoke <device_id> - revoke a device"
}

func (b *Bot) checkBackend(ctx context.Context) error {
	healthCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := b.backend.Health(healthCtx); err != nil {
		return fmt.Errorf("check backend health: %w", err)
	}

	return nil
}

func (b *Bot) handleDevices(ctx context.Context, message *telegram.Message) error {
	if message.From == nil {
		return b.telegram.SendMessage(ctx, message.Chat.ID, "Cannot identify Telegram user for this command.")
	}

	result, err := b.backend.ListDevices(ctx, message.From.ID)
	if err != nil {
		b.logger.Error("list devices via backend", "telegram_user_id", message.From.ID, "error", err)
		if errors.Is(err, backendapi.ErrNotFound) {
			return b.telegram.SendMessage(ctx, message.Chat.ID, "You are not linked to a VPN user yet.")
		}
		return b.telegram.SendMessage(ctx, message.Chat.ID, "Failed to load devices right now. Please try again later.")
	}

	return b.telegram.SendMessage(ctx, message.Chat.ID, formatDevicesMessage(result))
}

func (b *Bot) handleNewDevice(ctx context.Context, message *telegram.Message, rawName string) error {
	if message.From == nil {
		return b.telegram.SendMessage(ctx, message.Chat.ID, "Cannot identify Telegram user for this command.")
	}

	name, validationMessage := validateDeviceName(rawName)
	if validationMessage != "" {
		return b.telegram.SendMessage(ctx, message.Chat.ID, validationMessage)
	}

	result, err := b.backend.CreateDevice(ctx, message.From.ID, name)
	if err != nil {
		b.logger.Error("create device via backend", "telegram_user_id", message.From.ID, "device_name", name, "error", err)
		return b.telegram.SendMessage(ctx, message.Chat.ID, formatCreateDeviceError(err))
	}

	return b.telegram.SendMessage(ctx, message.Chat.ID, formatCreateDeviceMessage(result))
}

func (b *Bot) handleConfig(ctx context.Context, message *telegram.Message, rawDeviceID string) error {
	if message.From == nil {
		return b.telegram.SendMessage(ctx, message.Chat.ID, "Cannot identify Telegram user for this command.")
	}

	deviceID, validationMessage := parseDeviceID(rawDeviceID, "/config <device_id>")
	if validationMessage != "" {
		return b.telegram.SendMessage(ctx, message.Chat.ID, validationMessage)
	}

	result, err := b.backend.ResendDeviceConfig(ctx, message.From.ID, deviceID)
	if err != nil {
		b.logger.Error("resend device config via backend", "telegram_user_id", message.From.ID, "device_id", deviceID, "error", err)
		return b.telegram.SendMessage(ctx, message.Chat.ID, formatResendDeviceConfigError(err))
	}

	return b.telegram.SendMessage(ctx, message.Chat.ID, formatResendDeviceConfigMessage(result))
}

func (b *Bot) handleRevoke(ctx context.Context, message *telegram.Message, rawDeviceID string) error {
	if message.From == nil {
		return b.telegram.SendMessage(ctx, message.Chat.ID, "Cannot identify Telegram user for this command.")
	}

	deviceID, validationMessage := parseDeviceID(rawDeviceID, "/revoke <device_id>")
	if validationMessage != "" {
		return b.telegram.SendMessage(ctx, message.Chat.ID, validationMessage)
	}

	result, err := b.backend.RevokeDevice(ctx, message.From.ID, deviceID)
	if err != nil {
		b.logger.Error("revoke device via backend", "telegram_user_id", message.From.ID, "device_id", deviceID, "error", err)
		return b.telegram.SendMessage(ctx, message.Chat.ID, formatRevokeDeviceError(err))
	}

	return b.telegram.SendMessage(ctx, message.Chat.ID, formatRevokeDeviceMessage(result))
}

func formatDevicesMessage(result *backendapi.ListDevicesResult) string {
	if result == nil || len(result.Devices) == 0 {
		return "You have no devices yet."
	}

	lines := []string{"Your devices:"}
	for i, device := range result.Devices {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, device.Name))
		lines = append(lines, fmt.Sprintf("Status: %s", device.Status))
		lines = append(lines, fmt.Sprintf("IP: %s", device.AssignedIP))
		lines = append(lines, fmt.Sprintf("Created: %s", device.CreatedAt.Format("2006-01-02")))
		if device.RevokedAt != nil {
			lines = append(lines, fmt.Sprintf("Revoked: %s", device.RevokedAt.Format("2006-01-02")))
		}
	}

	return strings.Join(lines, "\n")
}

func formatCreateDeviceMessage(result *backendapi.CreateDeviceResult) string {
	if result == nil {
		return "Device created."
	}

	lines := []string{"Device created successfully."}
	lines = append(lines, fmt.Sprintf("Name: %s", result.Device.Name))

	if result.Device.AssignedIP != "" {
		lines = append(lines, fmt.Sprintf("IP: %s", result.Device.AssignedIP))
	}

	if result.Device.Status != "" {
		lines = append(lines, fmt.Sprintf("Status: %s", result.Device.Status))
	}

	if !result.Device.CreatedAt.IsZero() {
		lines = append(lines, fmt.Sprintf("Created: %s", result.Device.CreatedAt.Format("2006-01-02")))
	}

	if strings.TrimSpace(result.ClientConfig) != "" {
		lines = append(lines, "", "Client config:", result.ClientConfig)
	}

	return strings.Join(lines, "\n")
}

func formatCreateDeviceError(err error) string {
	var apiErr *backendapi.APIError
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.StatusCode == 0:
			return "Failed to create the device right now. Please try again later."
		case apiErr.StatusCode == 400 && apiErr.Message != "":
			return fmt.Sprintf("Cannot create device: %s.", apiErr.Message)
		case apiErr.StatusCode == 403:
			return "You are not allowed to create a device right now."
		case apiErr.StatusCode == 409 && apiErr.Message != "":
			return fmt.Sprintf("Cannot create device: %s.", apiErr.Message)
		default:
			return "Failed to create the device right now. Please try again later."
		}
	}

	return "Failed to create the device right now. Please try again later."
}

func formatResendDeviceConfigMessage(result *backendapi.ResendDeviceConfigResult) string {
	if result == nil {
		return "Device config rebuilt."
	}

	lines := []string{"Device config rebuilt successfully."}
	if result.Device.Name != "" {
		lines = append(lines, fmt.Sprintf("Name: %s", result.Device.Name))
	}
	if result.Device.AssignedIP != "" {
		lines = append(lines, fmt.Sprintf("IP: %s", result.Device.AssignedIP))
	}
	if strings.TrimSpace(result.ClientConfig) != "" {
		lines = append(lines, "", "Client config:", result.ClientConfig)
	}

	return strings.Join(lines, "\n")
}

func formatResendDeviceConfigError(err error) string {
	if errors.Is(err, backendapi.ErrNotFound) {
		return "Device not found."
	}

	var apiErr *backendapi.APIError
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.StatusCode == 403:
			return "You are not allowed to access that device config."
		case apiErr.StatusCode == 409 && apiErr.Message != "":
			return fmt.Sprintf("Cannot resend config: %s.", apiErr.Message)
		default:
			return "Failed to resend the device config right now. Please try again later."
		}
	}

	return "Failed to resend the device config right now. Please try again later."
}

func formatRevokeDeviceMessage(result *backendapi.RevokeDeviceResult) string {
	if result == nil {
		return "Device revoked."
	}

	lines := []string{"Device revoked successfully."}
	if result.Device.Name != "" {
		lines = append(lines, fmt.Sprintf("Name: %s", result.Device.Name))
	}
	if result.Device.AssignedIP != "" {
		lines = append(lines, fmt.Sprintf("IP: %s", result.Device.AssignedIP))
	}
	if result.Device.Status != "" {
		lines = append(lines, fmt.Sprintf("Status: %s", result.Device.Status))
	}
	if result.Device.RevokedAt != nil {
		lines = append(lines, fmt.Sprintf("Revoked: %s", result.Device.RevokedAt.Format("2006-01-02")))
	}

	return strings.Join(lines, "\n")
}

func formatRevokeDeviceError(err error) string {
	var apiErr *backendapi.APIError
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.StatusCode == 403:
			return "You are not allowed to revoke that device."
		case apiErr.StatusCode == 404:
			return "Device not found or unavailable."
		case apiErr.StatusCode == 409 && apiErr.Message != "":
			return fmt.Sprintf("Cannot revoke device: %s.", apiErr.Message)
		default:
			return "Failed to revoke the device right now. Please try again later."
		}
	}

	return "Failed to revoke the device right now. Please try again later."
}

func validateDeviceName(rawName string) (string, string) {
	name := strings.TrimSpace(rawName)
	if name == "" {
		return "", "Usage: /newdevice <device_name>"
	}

	if utf8.RuneCountInString(name) > maxDeviceNameLength {
		return "", fmt.Sprintf("Device name is too long. Use %d characters or fewer.", maxDeviceNameLength)
	}

	for _, r := range name {
		if unicode.IsControl(r) {
			return "", "Device name must be a single line of plain text."
		}
	}

	return name, ""
}

func parseDeviceID(rawValue, usage string) (int64, string) {
	value := strings.TrimSpace(rawValue)
	if value == "" {
		return 0, "Usage: " + usage
	}

	deviceID, err := strconv.ParseInt(value, 10, 64)
	if err != nil || deviceID <= 0 {
		return 0, "Device ID must be a positive number."
	}

	return deviceID, ""
}
