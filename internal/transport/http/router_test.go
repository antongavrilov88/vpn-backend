package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"vpn-backend/internal/domain"
)

type stubHealthChecker struct {
	err error
}

func (s stubHealthChecker) Ping(context.Context) error {
	return s.err
}

func TestNewRouterLive(t *testing.T) {
	router := NewRouter(Dependencies{})

	request := httptest.NewRequest(http.MethodGet, "/live", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	if body := recorder.Body.String(); body != "{\"status\":\"ok\"}\n" {
		t.Fatalf("body = %q, want %q", body, "{\"status\":\"ok\"}\n")
	}
}

func TestNewRouterHealthReady(t *testing.T) {
	router := NewRouter(Dependencies{
		HealthChecker:      stubHealthChecker{},
		RequestTimeout:     5 * time.Second,
		ReadinessTimeout:   time.Second,
		ReadinessRoutePath: "/ready",
	})

	for _, path := range []string{"/health", "/ready"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d", path, recorder.Code, http.StatusOK)
		}
	}
}

func TestNewRouterHealthDegradedWhenPingFails(t *testing.T) {
	router := NewRouter(Dependencies{
		HealthChecker: stubHealthChecker{err: errors.New("db down")},
	})

	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}

	if body := recorder.Body.String(); body != "{\"status\":\"degraded\"}\n" {
		t.Fatalf("body = %q, want %q", body, "{\"status\":\"degraded\"}\n")
	}
}

type stubCreateDeviceUseCase struct {
	userID int64
	name   string
	result *CreateDeviceResult
	err    error
}

func (s *stubCreateDeviceUseCase) Execute(_ context.Context, userID int64, name string) (*CreateDeviceResult, error) {
	s.userID = userID
	s.name = name
	return s.result, s.err
}

type stubListUserDevicesUseCase struct {
	userID int64
	result *ListUserDevicesResult
	err    error
}

func (s *stubListUserDevicesUseCase) Execute(_ context.Context, userID int64) (*ListUserDevicesResult, error) {
	s.userID = userID
	return s.result, s.err
}

type stubResendDeviceConfigUseCase struct {
	userID   int64
	deviceID int64
	result   *ResendDeviceConfigResult
	err      error
}

func (s *stubResendDeviceConfigUseCase) Execute(_ context.Context, userID, deviceID int64) (*ResendDeviceConfigResult, error) {
	s.userID = userID
	s.deviceID = deviceID
	return s.result, s.err
}

type stubRevokeDeviceUseCase struct {
	userID   int64
	deviceID int64
	result   *RevokeDeviceResult
	err      error
}

func (s *stubRevokeDeviceUseCase) Execute(_ context.Context, userID, deviceID int64) (*RevokeDeviceResult, error) {
	s.userID = userID
	s.deviceID = deviceID
	return s.result, s.err
}

func TestNewRouterCreateDeviceHappyPath(t *testing.T) {
	useCase := &stubCreateDeviceUseCase{
		result: &CreateDeviceResult{
			Device: Device{
				ID:         100,
				Name:       "Dad Phone",
				AssignedIP: "10.67.0.2",
				Status:     "active",
				CreatedAt:  time.Date(2026, 4, 2, 10, 11, 12, 0, time.UTC),
			},
			ClientConfig: "[Interface]\nPrivateKey = key\n",
		},
	}

	router := NewRouter(Dependencies{
		CreateDevice:   useCase.Execute,
		RequestTimeout: 5 * time.Second,
	})

	request := httptest.NewRequest(http.MethodPost, "/devices", strings.NewReader(`{"name":"Dad Phone"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-User-ID", "42")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusCreated)
	}

	if useCase.userID != 42 || useCase.name != "Dad Phone" {
		t.Fatalf("input = user %d name %q, want user 42 name %q", useCase.userID, useCase.name, "Dad Phone")
	}

	body := recorder.Body.String()
	if !strings.Contains(body, `"client_config":"[Interface]\nPrivateKey = key\n"`) {
		t.Fatalf("body = %q, want client config", body)
	}

	if !strings.Contains(body, `"assigned_ip":"10.67.0.2"`) {
		t.Fatalf("body = %q, want assigned_ip", body)
	}

	if strings.Contains(body, "encrypted_private_key") || strings.Contains(body, "public_key") {
		t.Fatalf("body = %q, should not contain sensitive fields", body)
	}
}

func TestNewRouterRevokeDeviceMapsNotFound(t *testing.T) {
	useCase := &stubRevokeDeviceUseCase{
		err: domain.ErrNotFound,
	}

	router := NewRouter(Dependencies{
		RevokeDevice:   useCase.Execute,
		RequestTimeout: 5 * time.Second,
	})

	request := httptest.NewRequest(http.MethodPost, "/devices/100/revoke", nil)
	request.Header.Set("X-User-ID", "42")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}

	if useCase.userID != 42 || useCase.deviceID != 100 {
		t.Fatalf("input = user %d device %d, want user 42 device 100", useCase.userID, useCase.deviceID)
	}

	if body := recorder.Body.String(); body != "{\"error\":\"not found\"}\n" {
		t.Fatalf("body = %q, want %q", body, "{\"error\":\"not found\"}\n")
	}
}
