package telegram

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestClientSetCommands(t *testing.T) {
	client, err := NewClient("test-token", time.Second)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	client.baseURL = "https://api.telegram.org/bottest-token"
	client.httpClient.Transport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/bottest-token/setMyCommands" {
			t.Fatalf("path = %s, want %s", r.URL.Path, "/bottest-token/setMyCommands")
		}

		if got := r.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
			t.Fatalf("content-type = %q, want %q", got, "application/x-www-form-urlencoded")
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}

		values, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("parse query: %v", err)
		}

		if got := values.Get("commands"); got != `[{"command":"start","description":"Check access and begin"},{"command":"help","description":"Show help"},{"command":"promo","description":"Enter invite code"}]` {
			t.Fatalf("commands payload = %q", got)
		}

		return jsonResponse(`{"ok":true,"result":true}`), nil
	})

	err = client.SetCommands(context.Background(), []BotCommand{
		{Command: "start", Description: "Check access and begin"},
		{Command: "help", Description: "Show help"},
		{Command: "promo", Description: "Enter invite code"},
	})
	if err != nil {
		t.Fatalf("SetCommands() error = %v", err)
	}
}

func TestClientSetCommandsWithScope(t *testing.T) {
	client, err := NewClient("test-token", time.Second)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	client.baseURL = "https://api.telegram.org/bottest-token"
	client.httpClient.Transport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}

		values, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("parse query: %v", err)
		}

		if got := values.Get("scope"); got != `{"type":"all_private_chats"}` {
			t.Fatalf("scope payload = %q", got)
		}

		return jsonResponse(`{"ok":true,"result":true}`), nil
	})

	err = client.SetCommandsWithScope(context.Background(), []BotCommand{
		{Command: "promo", Description: "Enter invite code"},
	}, &BotCommandScope{Type: BotCommandScopeAllPrivateChats})
	if err != nil {
		t.Fatalf("SetCommandsWithScope() error = %v", err)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: io.NopCloser(strings.NewReader(body)),
	}
}
