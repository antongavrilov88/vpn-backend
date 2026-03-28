package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"vpn-backend/internal/app"
	"vpn-backend/internal/config"
	apphttp "vpn-backend/internal/transport/http"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "api failed: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	application, err := app.New(ctx, cfg)
	if err != nil {
		return fmt.Errorf("initialize app: %w", err)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
		defer cancel()

		if closeErr := application.Close(closeCtx); closeErr != nil {
			application.Logger.Error("close application", "error", closeErr)
		}
	}()

	server := &http.Server{
		Addr: cfg.HTTP.Addr,
		Handler: apphttp.NewRouter(apphttp.Dependencies{
			HealthChecker:      application.DB,
			RequestTimeout:     cfg.HTTP.RequestTimeout,
			ReadinessTimeout:   cfg.HTTP.ReadinessTimeout,
			ReadinessRoutePath: "/ready",
		}),
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
		IdleTimeout:  cfg.HTTP.IdleTimeout,
	}

	serverErr := make(chan error, 1)

	go func() {
		application.Logger.Info("http server started", "addr", server.Addr)

		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}

		serverErr <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
		defer cancel()

		application.Logger.Info("shutdown signal received")

		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown http server: %w", err)
		}

		if err := <-serverErr; err != nil {
			return fmt.Errorf("wait for http server stop: %w", err)
		}

		return nil
	case err := <-serverErr:
		if err != nil {
			application.Logger.Error("http server stopped with error", "error", err)
			return fmt.Errorf("serve http: %w", err)
		}

		return nil
	}
}
