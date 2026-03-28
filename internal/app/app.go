package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"vpn-backend/internal/config"
	"vpn-backend/internal/infra/logger"
)

type App struct {
	Config config.Config
	Logger *slog.Logger
	DB     *pgxpool.Pool
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	log := logger.New(cfg.AppEnv)

	db, err := pgxpool.New(ctx, cfg.DB.URL)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	if err := db.Ping(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &App{
		Config: cfg,
		Logger: log,
		DB:     db,
	}, nil
}

func (a *App) Close(_ context.Context) error {
	if a.DB != nil {
		a.DB.Close()
	}

	return nil
}
