package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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
