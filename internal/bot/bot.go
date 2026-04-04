package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"vpn-backend/internal/infra/backendapi"
	"vpn-backend/internal/infra/telegram"
)

type Bot struct {
	logger      *slog.Logger
	telegram    *telegram.Client
	backend     backendClient
	pollTimeout time.Duration
}

type backendClient interface {
	backendapi.HealthChecker
	ListDevices(ctx context.Context, telegramUserID int64) (*backendapi.ListDevicesResult, error)
}

func New(logger *slog.Logger, telegramClient *telegram.Client, backendClient backendClient, pollTimeout time.Duration) *Bot {
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
	return "Available commands:\n/start - show welcome message\n/help - show this help\n/devices - show your devices"
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
