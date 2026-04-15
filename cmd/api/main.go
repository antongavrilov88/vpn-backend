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
	"vpn-backend/internal/domain"
	"vpn-backend/internal/infra/postgres"
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
			HealthChecker: application.DB,
			ResolveTelegramUserID: func(ctx context.Context, telegramUserID int64) (int64, error) {
				userRepository := postgres.NewUserRepository(application.DB)
				user, err := userRepository.GetByTelegramID(ctx, telegramUserID)
				if err != nil {
					return 0, err
				}

				return user.ID, nil
			},
			CreateDevice: func(ctx context.Context, userID int64, name string) (*apphttp.CreateDeviceResult, error) {
				if application.CreateDevice == nil {
					return nil, fmt.Errorf("create device is not configured")
				}

				result, err := application.CreateDevice.Execute(ctx, app.CreateDeviceInput{
					UserID: userID,
					Name:   name,
				})
				if err != nil {
					application.Logger.Error("create device failed", "user_id", userID, "name", name, "error", err)
					return nil, err
				}

				return &apphttp.CreateDeviceResult{
					Device:       toHTTPDevice(result.Device),
					ClientConfig: result.ClientConfig,
				}, nil
			},
			ListUserDevices: func(ctx context.Context, callerUserID int64) (*apphttp.ListUserDevicesResult, error) {
				if application.ListUserDevices == nil {
					return nil, fmt.Errorf("list user devices is not configured")
				}

				result, err := application.ListUserDevices.Execute(ctx, app.ListUserDevicesInput{
					CallerUserID: callerUserID,
					UserID:       callerUserID,
				})
				if err != nil {
					return nil, err
				}

				devices := make([]apphttp.Device, 0, len(result.Devices))
				for _, device := range result.Devices {
					devices = append(devices, apphttp.Device{
						ID:         device.ID,
						Name:       device.Name,
						AssignedIP: device.AssignedIP,
						Status:     string(device.Status),
						CreatedAt:  device.CreatedAt,
						RevokedAt:  device.RevokedAt,
					})
				}

				return &apphttp.ListUserDevicesResult{Devices: devices}, nil
			},
			ResendDeviceConfig: func(ctx context.Context, userID, deviceID int64) (*apphttp.ResendDeviceConfigResult, error) {
				if application.ResendDeviceConfig == nil {
					return nil, fmt.Errorf("resend device config is not configured")
				}

				result, err := application.ResendDeviceConfig.Execute(ctx, app.ResendDeviceConfigInput{
					UserID:   userID,
					DeviceID: deviceID,
				})
				if err != nil {
					return nil, err
				}

				return &apphttp.ResendDeviceConfigResult{
					Device:       toHTTPDevice(result.Device),
					ClientConfig: result.ClientConfig,
				}, nil
			},
			RevokeDevice: func(ctx context.Context, userID, deviceID int64) (*apphttp.RevokeDeviceResult, error) {
				if application.RevokeDevice == nil {
					return nil, fmt.Errorf("revoke device is not configured")
				}

				result, err := application.RevokeDevice.Execute(ctx, app.RevokeDeviceInput{
					UserID:   userID,
					DeviceID: deviceID,
				})
				if err != nil {
					return nil, err
				}

				return &apphttp.RevokeDeviceResult{
					Device: toHTTPDevice(result.Device),
				}, nil
			},
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

func toHTTPDevice(device *domain.Device) apphttp.Device {
	return apphttp.Device{
		ID:         device.ID,
		Name:       device.Name,
		AssignedIP: device.AssignedIP,
		Status:     string(device.Status),
		CreatedAt:  device.CreatedAt,
		RevokedAt:  device.RevokedAt,
	}
}
