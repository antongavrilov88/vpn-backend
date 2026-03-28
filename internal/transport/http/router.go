package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type HealthChecker interface {
	Ping(ctx context.Context) error
}

type Dependencies struct {
	HealthChecker      HealthChecker
	RequestTimeout     time.Duration
	ReadinessTimeout   time.Duration
	ReadinessRoutePath string
}

type healthResponse struct {
	Status string `json:"status"`
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

	return router
}

func writeJSON(w http.ResponseWriter, statusCode int, payload healthResponse) {
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
