package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"vpn-backend/internal/domain"
)

type HealthChecker interface {
	Ping(ctx context.Context) error
}

type CreateDeviceFunc func(ctx context.Context, userID int64, name string) (*CreateDeviceResult, error)
type ListUserDevicesFunc func(ctx context.Context, callerUserID int64) (*ListUserDevicesResult, error)
type ResendDeviceConfigFunc func(ctx context.Context, userID, deviceID int64) (*ResendDeviceConfigResult, error)
type RevokeDeviceFunc func(ctx context.Context, userID, deviceID int64) (*RevokeDeviceResult, error)

type Dependencies struct {
	HealthChecker      HealthChecker
	CreateDevice       CreateDeviceFunc
	ListUserDevices    ListUserDevicesFunc
	ResendDeviceConfig ResendDeviceConfigFunc
	RevokeDevice       RevokeDeviceFunc
	RequestTimeout     time.Duration
	ReadinessTimeout   time.Duration
	ReadinessRoutePath string
}

type Device struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	AssignedIP string     `json:"assigned_ip"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

type CreateDeviceResult struct {
	Device       Device `json:"device"`
	ClientConfig string `json:"client_config"`
}

type ListUserDevicesResult struct {
	Devices []Device `json:"devices"`
}

type ResendDeviceConfigResult struct {
	Device       Device `json:"device"`
	ClientConfig string `json:"client_config"`
}

type RevokeDeviceResult struct {
	Device Device `json:"device"`
}

type healthResponse struct {
	Status string `json:"status"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type createDeviceRequest struct {
	Name string `json:"name"`
}

func NewRouter(deps Dependencies) http.Handler {
	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Timeout(defaultDuration(deps.RequestTimeout, 30*time.Second)))

	router.Get("/live", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
	})

	readinessHandler := func(w http.ResponseWriter, r *http.Request) {
		if deps.HealthChecker == nil {
			writeJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "degraded"})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), defaultDuration(deps.ReadinessTimeout, 2*time.Second))
		defer cancel()

		if err := deps.HealthChecker.Ping(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "degraded"})
			return
		}

		writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
	}

	router.Get("/health", readinessHandler)

	if path := deps.ReadinessRoutePath; path != "" && path != "/health" {
		router.Get(path, readinessHandler)
	}

	router.Route("/devices", func(r chi.Router) {
		r.Post("/", createDeviceHandler(deps))
		r.Get("/", listUserDevicesHandler(deps))
		r.Get("/{id}/config", resendDeviceConfigHandler(deps))
		r.Post("/{id}/revoke", revokeDeviceHandler(deps))
	})

	return router
}

func createDeviceHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.CreateDevice == nil {
			writeError(w, http.StatusServiceUnavailable, "create device is not configured")
			return
		}

		userID, ok := userIDFromRequest(w, r)
		if !ok {
			return
		}

		var request createDeviceRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}

		if request.Name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}

		result, err := deps.CreateDevice(r.Context(), userID, request.Name)
		if err != nil {
			writeUseCaseError(w, err)
			return
		}

		writeJSON(w, http.StatusCreated, result)
	}
}

func listUserDevicesHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.ListUserDevices == nil {
			writeError(w, http.StatusServiceUnavailable, "list user devices is not configured")
			return
		}

		userID, ok := userIDFromRequest(w, r)
		if !ok {
			return
		}

		result, err := deps.ListUserDevices(r.Context(), userID)
		if err != nil {
			writeUseCaseError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}

func resendDeviceConfigHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.ResendDeviceConfig == nil {
			writeError(w, http.StatusServiceUnavailable, "resend device config is not configured")
			return
		}

		userID, ok := userIDFromRequest(w, r)
		if !ok {
			return
		}

		deviceID, ok := deviceIDFromRoute(w, r)
		if !ok {
			return
		}

		result, err := deps.ResendDeviceConfig(r.Context(), userID, deviceID)
		if err != nil {
			writeUseCaseError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}

func revokeDeviceHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.RevokeDevice == nil {
			writeError(w, http.StatusServiceUnavailable, "revoke device is not configured")
			return
		}

		userID, ok := userIDFromRequest(w, r)
		if !ok {
			return
		}

		deviceID, ok := deviceIDFromRoute(w, r)
		if !ok {
			return
		}

		result, err := deps.RevokeDevice(r.Context(), userID, deviceID)
		if err != nil {
			writeUseCaseError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}

func userIDFromRequest(w http.ResponseWriter, r *http.Request) (int64, bool) {
	value := r.Header.Get("X-User-ID")
	if value == "" {
		writeError(w, http.StatusBadRequest, "X-User-ID header is required")
		return 0, false
	}

	userID, err := strconv.ParseInt(value, 10, 64)
	if err != nil || userID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid X-User-ID header")
		return 0, false
	}

	return userID, true
}

func deviceIDFromRoute(w http.ResponseWriter, r *http.Request) (int64, bool) {
	deviceID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || deviceID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid device id")
		return 0, false
	}

	return deviceID, true
}

func writeUseCaseError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, domain.ErrUserBlocked), errors.Is(err, domain.ErrUserDeleted), errors.Is(err, domain.ErrSubscriptionMiss):
		writeError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, domain.ErrDeviceExists), errors.Is(err, domain.ErrConflict), errors.Is(err, domain.ErrDeviceRevoked), errors.Is(err, domain.ErrIPPoolExhausted):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, errorResponse{Error: message})
}

func writeJSON(w http.ResponseWriter, statusCode int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if _, err := w.Write(append(response, '\n')); err != nil && !errors.Is(err, context.Canceled) {
		return
	}
}

func defaultDuration(value, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}

	return value
}
