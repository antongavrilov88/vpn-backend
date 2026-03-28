package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

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

func (a *App) Run(ctx context.Context, server *http.Server) error {
	serverErr := make(chan error, 1)

	go func() {
		a.Logger.Info("http server started", "addr", server.Addr)

		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}

		serverErr <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), a.Config.HTTP.ShutdownTimeout)
		defer cancel()

		a.Logger.Info("shutdown signal received")

		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown http server: %w", err)
		}

		if err := <-serverErr; err != nil {
			return fmt.Errorf("wait for http server stop: %w", err)
		}

		return nil
	case err := <-serverErr:
		if err != nil {
			return fmt.Errorf("serve http: %w", err)
		}

		return nil
	}
}

func (a *App) Close(_ context.Context) error {
	if a.DB != nil {
		a.DB.Close()
	}

	return nil
}
