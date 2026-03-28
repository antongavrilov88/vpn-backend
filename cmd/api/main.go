package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"vpn-backend/internal/app"
	"vpn-backend/internal/config"
	apphttp "vpn-backend/internal/transport/http"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	application, err := app.New(ctx, cfg)
	if err != nil {
		log.Fatalf("initialize app: %v", err)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
		defer cancel()

		if closeErr := application.Close(closeCtx); closeErr != nil {
			application.Logger.Error("close application", "error", closeErr)
		}
	}()

	server := &http.Server{
		Addr:         cfg.HTTP.Addr,
		Handler:      apphttp.NewRouter(application),
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
		IdleTimeout:  cfg.HTTP.IdleTimeout,
	}

	if err := application.Run(ctx, server); err != nil {
		application.Logger.Error("application stopped with error", "error", err)
		log.Fatalf("run application: %v", err)
	}
}
