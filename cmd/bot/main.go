package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"vpn-backend/internal/bot"
	"vpn-backend/internal/config"
	"vpn-backend/internal/infra/backendapi"
	"vpn-backend/internal/infra/logger"
	"vpn-backend/internal/infra/telegram"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "bot failed: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.LoadBot()
	if err != nil {
		return fmt.Errorf("load bot config: %w", err)
	}

	log := logger.New(cfg.AppEnv)

	backendClient, err := backendapi.NewClient(cfg.BackendAPI.BaseURL, cfg.BackendAPI.Timeout)
	if err != nil {
		return fmt.Errorf("initialize backend api client: %w", err)
	}

	telegramClient, err := telegram.NewClient(cfg.Bot.Token, cfg.Bot.PollTimeout)
	if err != nil {
		return fmt.Errorf("initialize telegram client: %w", err)
	}

	b := bot.New(log, telegramClient, backendClient, cfg.Bot.PollTimeout)

	if err := b.Run(ctx); err != nil {
		return fmt.Errorf("run bot: %w", err)
	}

	return nil
}
