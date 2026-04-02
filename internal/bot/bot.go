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
	backend     backendapi.HealthChecker
	pollTimeout time.Duration
}

func New(logger *slog.Logger, telegramClient *telegram.Client, backendClient backendapi.HealthChecker, pollTimeout time.Duration) *Bot {
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
	return "Available commands:\n/start - show welcome message\n/help - show this help"
}

func (b *Bot) checkBackend(ctx context.Context) error {
	healthCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := b.backend.Health(healthCtx); err != nil {
		return fmt.Errorf("check backend health: %w", err)
	}

	return nil
}
