package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"vpn-backend/internal/domain"
)

type HealthChecker interface {
	Ping(ctx context.Context) error
}

type ResolveTelegramUserIDFunc func(ctx context.Context, telegramUserID int64) (int64, error)
type EnsureTelegramUserFunc func(ctx context.Context, telegramUserID int64, username string) (*EnsureTelegramUserResult, error)
type GetAccessStatusFunc func(ctx context.Context, userID int64) (*AccessStatusResult, error)
type ApplyInviteCodeFunc func(ctx context.Context, userID int64, code string) (*AccessStatusResult, error)
type CreateDeviceFunc func(ctx context.Context, userID int64, name string) (*CreateDeviceResult, error)
type ListUserDevicesFunc func(ctx context.Context, callerUserID int64) (*ListUserDevicesResult, error)
type ResendDeviceConfigFunc func(ctx context.Context, userID, deviceID int64) (*ResendDeviceConfigResult, error)
type RevokeDeviceFunc func(ctx context.Context, userID, deviceID int64) (*RevokeDeviceResult, error)

type Dependencies struct {
	HealthChecker         HealthChecker
	ResolveTelegramUserID ResolveTelegramUserIDFunc
	EnsureTelegramUser    EnsureTelegramUserFunc
	GetAccessStatus       GetAccessStatusFunc
	ApplyInviteCode       ApplyInviteCodeFunc
	CreateDevice          CreateDeviceFunc
	ListUserDevices       ListUserDevicesFunc
	ResendDeviceConfig    ResendDeviceConfigFunc
	RevokeDevice          RevokeDeviceFunc
	RequestTimeout        time.Duration
	ReadinessTimeout      time.Duration
	ReadinessRoutePath    string
}

type Device struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	AssignedIP string     `json:"assigned_ip"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

type User struct {
	ID         int64     `json:"id"`
	TelegramID *int64    `json:"telegram_id,omitempty"`
	Username   string    `json:"username"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type CreateDeviceResult struct {
	Device       Device `json:"device"`
	ClientConfig string `json:"client_config"`
}

type EnsureTelegramUserResult struct {
	User User `json:"user"`
}

type AccessStatusResult struct {
	AccessActive    bool       `json:"access_active"`
	IsLifetime      bool       `json:"is_lifetime"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	CanCreateDevice bool       `json:"can_create_device"`
	DenialReason    string     `json:"denial_reason,omitempty"`
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

type ensureTelegramUserRequest struct {
	Username string `json:"username"`
}

type applyInviteCodeRequest struct {
	Code string `json:"code"`
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

	router.Route("/access", func(r chi.Router) {
		r.Get("/", accessStatusHandler(deps))
		r.Post("/apply-invite-code", applyInviteCodeHandler(deps))
	})

	router.Route("/telegram", func(r chi.Router) {
		r.Post("/session", ensureTelegramUserHandler(deps))
	})

	return router
}

func accessStatusHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.GetAccessStatus == nil {
			writeError(w, http.StatusServiceUnavailable, "access status is not configured")
			return
		}

		userID, ok := userIDFromRequest(w, r, deps.ResolveTelegramUserID, deps.EnsureTelegramUser)
		if !ok {
			return
		}

		result, err := deps.GetAccessStatus(r.Context(), userID)
		if err != nil {
			writeUseCaseError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}

func applyInviteCodeHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.ApplyInviteCode == nil {
			writeError(w, http.StatusServiceUnavailable, "apply invite code is not configured")
			return
		}

		userID, ok := userIDFromRequest(w, r, deps.ResolveTelegramUserID, deps.EnsureTelegramUser)
		if !ok {
			return
		}

		var request applyInviteCodeRequest
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

		request.Code = strings.TrimSpace(request.Code)
		if request.Code == "" {
			writeError(w, http.StatusBadRequest, "code is required")
			return
		}

		result, err := deps.ApplyInviteCode(r.Context(), userID, request.Code)
		if err != nil {
			writeUseCaseError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}

func ensureTelegramUserHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.EnsureTelegramUser == nil {
			writeError(w, http.StatusServiceUnavailable, "telegram session bootstrap is not configured")
			return
		}

		telegramUserID, ok := telegramUserIDFromHeader(w, r)
		if !ok {
			return
		}

		request, ok := decodeEnsureTelegramUserRequest(w, r)
		if !ok {
			return
		}

		result, err := deps.EnsureTelegramUser(r.Context(), telegramUserID, request.Username)
		if err != nil {
			writeUseCaseError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}

func createDeviceHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.CreateDevice == nil {
			writeError(w, http.StatusServiceUnavailable, "create device is not configured")
			return
		}

		userID, ok := userIDFromRequest(w, r, deps.ResolveTelegramUserID, deps.EnsureTelegramUser)
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

		userID, ok := userIDFromRequest(w, r, deps.ResolveTelegramUserID, deps.EnsureTelegramUser)
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

		userID, ok := userIDFromRequest(w, r, deps.ResolveTelegramUserID, deps.EnsureTelegramUser)
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

		userID, ok := userIDFromRequest(w, r, deps.ResolveTelegramUserID, deps.EnsureTelegramUser)
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

func userIDFromRequest(
	w http.ResponseWriter,
	r *http.Request,
	resolveTelegramUserID ResolveTelegramUserIDFunc,
	ensureTelegramUser EnsureTelegramUserFunc,
) (int64, bool) {
	value := r.Header.Get("X-User-ID")
	if value != "" {
		userID, err := strconv.ParseInt(value, 10, 64)
		if err != nil || userID <= 0 {
			writeError(w, http.StatusBadRequest, "invalid X-User-ID header")
			return 0, false
		}

		return userID, true
	}

	if resolveTelegramUserID == nil {
		writeError(w, http.StatusServiceUnavailable, "telegram identity is not configured")
		return 0, false
	}

	telegramUserID, ok := telegramUserIDFromHeader(w, r)
	if !ok {
		return 0, false
	}

	if ensureTelegramUser != nil {
		if _, err := ensureTelegramUser(r.Context(), telegramUserID, ""); err != nil {
			writeUseCaseError(w, err)
			return 0, false
		}
	}

	userID, err := resolveTelegramUserID(r.Context(), telegramUserID)
	if err != nil {
		writeUseCaseError(w, err)
		return 0, false
	}

	return userID, true
}

func telegramUserIDFromHeader(w http.ResponseWriter, r *http.Request) (int64, bool) {
	telegramValue := r.Header.Get("X-Telegram-ID")
	if telegramValue == "" {
		writeError(w, http.StatusBadRequest, "X-User-ID or X-Telegram-ID header is required")
		return 0, false
	}

	telegramUserID, err := strconv.ParseInt(telegramValue, 10, 64)
	if err != nil || telegramUserID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid X-Telegram-ID header")
		return 0, false
	}

	return telegramUserID, true
}

func decodeEnsureTelegramUserRequest(w http.ResponseWriter, r *http.Request) (ensureTelegramUserRequest, bool) {
	if r.Body == nil {
		return ensureTelegramUserRequest{}, true
	}

	var request ensureTelegramUserRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		if errors.Is(err, io.EOF) {
			return ensureTelegramUserRequest{}, true
		}

		writeError(w, http.StatusBadRequest, "invalid json body")
		return ensureTelegramUserRequest{}, false
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return ensureTelegramUserRequest{}, false
	}

	return request, true
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
	case errors.Is(err, domain.ErrDeviceExists), errors.Is(err, domain.ErrConflict), errors.Is(err, domain.ErrDeviceRevoked), errors.Is(err, domain.ErrIPPoolExhausted), errors.Is(err, domain.ErrPromoCodeInactive), errors.Is(err, domain.ErrPromoCodeUsed), errors.Is(err, domain.ErrPromoCodeLimit), errors.Is(err, domain.ErrPromoCodeAccess):
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
