package backendapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestClientListDevicesEnsuresTelegramSessionFirst(t *testing.T) {
	var requests []string

	client, err := NewClient("http://backend.local", time.Second)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	client.httpClient.Transport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		requests = append(requests, r.Method+" "+r.URL.Path)

		if got := r.Header.Get("X-Telegram-ID"); got != "777" {
			t.Fatalf("X-Telegram-ID = %q, want %q", got, "777")
		}

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/telegram/session":
			return jsonResponse(http.StatusOK, EnsureTelegramSessionResult{
				User: User{ID: 42, Status: "active"},
			}), nil
		case r.Method == http.MethodGet && r.URL.Path == "/devices":
			return jsonResponse(http.StatusOK, ListDevicesResult{
				Devices: []Device{{ID: 100, Name: "iphone", AssignedIP: "10.68.0.2", Status: "active"}},
			}), nil
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
			return nil, nil
		}
	})

	result, err := client.ListDevices(context.Background(), 777)
	if err != nil {
		t.Fatalf("ListDevices() error = %v", err)
	}

	if result == nil || len(result.Devices) != 1 {
		t.Fatalf("ListDevices() result = %#v, want one device", result)
	}

	wantRequests := []string{
		http.MethodPost + " /telegram/session",
		http.MethodGet + " /devices",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("requests = %#v, want %#v", requests, wantRequests)
	}
}

func TestClientListDevicesReturnsBootstrapError(t *testing.T) {
	var requests []string

	client, err := NewClient("http://backend.local", time.Second)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	client.httpClient.Transport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		requests = append(requests, r.Method+" "+r.URL.Path)

		if r.Method == http.MethodPost && r.URL.Path == "/telegram/session" {
			return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "internal error"}), nil
		}

		t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		return nil, nil
	})

	result, err := client.ListDevices(context.Background(), 777)
	if err == nil {
		t.Fatal("ListDevices() error = nil, want error")
	}

	if result != nil {
		t.Fatalf("ListDevices() result = %#v, want nil", result)
	}

	if len(requests) != 1 || requests[0] != http.MethodPost+" /telegram/session" {
		t.Fatalf("requests = %#v, want only bootstrap call", requests)
	}
}

func TestClientGetAccessStatus(t *testing.T) {
	client, err := NewClient("http://backend.local", time.Second)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	client.httpClient.Transport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet || r.URL.Path != "/access" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}

		if got := r.Header.Get("X-Telegram-ID"); got != "777" {
			t.Fatalf("X-Telegram-ID = %q, want %q", got, "777")
		}

		return jsonResponse(http.StatusOK, AccessStatusResult{
			AccessActive:    true,
			IsLifetime:      true,
			CanCreateDevice: true,
		}), nil
	})

	result, err := client.GetAccessStatus(context.Background(), 777)
	if err != nil {
		t.Fatalf("GetAccessStatus() error = %v", err)
	}

	if result == nil || !result.AccessActive || !result.IsLifetime || !result.CanCreateDevice {
		t.Fatalf("GetAccessStatus() result = %#v, want active lifetime access", result)
	}
}

func TestClientApplyInviteCode(t *testing.T) {
	client, err := NewClient("http://backend.local", time.Second)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	client.httpClient.Transport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost || r.URL.Path != "/access/apply-invite-code" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}

		if got := r.Header.Get("X-Telegram-ID"); got != "777" {
			t.Fatalf("X-Telegram-ID = %q, want %q", got, "777")
		}

		var payload struct {
			Code string `json:"code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if payload.Code != "BETA-ANTON" {
			t.Fatalf("code = %q, want %q", payload.Code, "BETA-ANTON")
		}

		return jsonResponse(http.StatusOK, AccessStatusResult{
			AccessActive:    true,
			IsLifetime:      false,
			CanCreateDevice: true,
		}), nil
	})

	result, err := client.ApplyInviteCode(context.Background(), 777, "BETA-ANTON")
	if err != nil {
		t.Fatalf("ApplyInviteCode() error = %v", err)
	}

	if result == nil || !result.AccessActive || !result.CanCreateDevice {
		t.Fatalf("ApplyInviteCode() result = %#v, want active access", result)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func jsonResponse(statusCode int, payload interface{}) *http.Response {
	body, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}

	return &http.Response{
		StatusCode: statusCode,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(string(body))),
	}
}
