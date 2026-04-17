package backendapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type HealthChecker interface {
	Health(ctx context.Context) error
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

type EnsureTelegramSessionResult struct {
	User User `json:"user"`
}

type AccessStatusResult struct {
	AccessActive    bool       `json:"access_active"`
	IsLifetime      bool       `json:"is_lifetime"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	CanCreateDevice bool       `json:"can_create_device"`
	DenialReason    string     `json:"denial_reason,omitempty"`
}

type ListDevicesResult struct {
	Devices []Device `json:"devices"`
}

type CreateDeviceResult struct {
	Device       Device `json:"device"`
	ClientConfig string `json:"client_config"`
}

type ResendDeviceConfigResult struct {
	Device       Device `json:"device"`
	ClientConfig string `json:"client_config"`
}

type RevokeDeviceResult struct {
	Device Device `json:"device"`
}

var ErrNotFound = errors.New("backend api not found")

type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}

	if e.Message != "" {
		return fmt.Sprintf("backend api status %d: %s", e.StatusCode, e.Message)
	}

	return fmt.Sprintf("backend api status %d", e.StatusCode)
}

func NewClient(baseURL string, timeout time.Duration) (*Client, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		return nil, fmt.Errorf("backend api base url is required")
	}

	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

func (c *Client) Health(ctx context.Context) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return fmt.Errorf("build health request: %w", err)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("call backend health: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("backend health returned status %d", response.StatusCode)
	}

	return nil
}

func (c *Client) ListDevices(ctx context.Context, telegramUserID int64) (*ListDevicesResult, error) {
	if _, err := c.EnsureTelegramSession(ctx, telegramUserID, ""); err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/devices", nil)
	if err != nil {
		return nil, fmt.Errorf("build list devices request: %w", err)
	}
	request.Header.Set("X-Telegram-ID", strconv.FormatInt(telegramUserID, 10))

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call backend devices: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, decodeAPIError(response.Body, response.StatusCode)
	}

	var result ListDevicesResult
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode backend devices response: %w", err)
	}

	return &result, nil
}

func (c *Client) GetAccessStatus(ctx context.Context, telegramUserID int64) (*AccessStatusResult, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/access", nil)
	if err != nil {
		return nil, fmt.Errorf("build access status request: %w", err)
	}
	request.Header.Set("X-Telegram-ID", strconv.FormatInt(telegramUserID, 10))

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call backend access status: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, decodeAPIError(response.Body, response.StatusCode)
	}

	var result AccessStatusResult
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode backend access status response: %w", err)
	}

	return &result, nil
}

func (c *Client) ApplyInviteCode(ctx context.Context, telegramUserID int64, code string) (*AccessStatusResult, error) {
	requestBody, err := json.Marshal(struct {
		Code string `json:"code"`
	}{
		Code: code,
	})
	if err != nil {
		return nil, fmt.Errorf("encode apply invite code request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/access/apply-invite-code", bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("build apply invite code request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Telegram-ID", strconv.FormatInt(telegramUserID, 10))

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call backend apply invite code: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, decodeAPIError(response.Body, response.StatusCode)
	}

	var result AccessStatusResult
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode backend apply invite code response: %w", err)
	}

	return &result, nil
}

func (c *Client) CreateDevice(ctx context.Context, telegramUserID int64, name string) (*CreateDeviceResult, error) {
	if _, err := c.EnsureTelegramSession(ctx, telegramUserID, ""); err != nil {
		return nil, err
	}

	requestBody, err := json.Marshal(struct {
		Name string `json:"name"`
	}{
		Name: name,
	})
	if err != nil {
		return nil, fmt.Errorf("encode create device request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/devices", bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("build create device request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Telegram-ID", strconv.FormatInt(telegramUserID, 10))

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call backend create device: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		return nil, decodeAPIError(response.Body, response.StatusCode)
	}

	var result CreateDeviceResult
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode backend create device response: %w", err)
	}

	return &result, nil
}

func (c *Client) ResendDeviceConfig(ctx context.Context, telegramUserID, deviceID int64) (*ResendDeviceConfigResult, error) {
	if _, err := c.EnsureTelegramSession(ctx, telegramUserID, ""); err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/devices/"+strconv.FormatInt(deviceID, 10)+"/config", nil)
	if err != nil {
		return nil, fmt.Errorf("build resend device config request: %w", err)
	}
	request.Header.Set("X-Telegram-ID", strconv.FormatInt(telegramUserID, 10))

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call backend resend device config: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}

	if response.StatusCode != http.StatusOK {
		return nil, decodeAPIError(response.Body, response.StatusCode)
	}

	var result ResendDeviceConfigResult
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode backend resend device config response: %w", err)
	}

	return &result, nil
}

func (c *Client) RevokeDevice(ctx context.Context, telegramUserID, deviceID int64) (*RevokeDeviceResult, error) {
	if _, err := c.EnsureTelegramSession(ctx, telegramUserID, ""); err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/devices/"+strconv.FormatInt(deviceID, 10)+"/revoke", nil)
	if err != nil {
		return nil, fmt.Errorf("build revoke device request: %w", err)
	}
	request.Header.Set("X-Telegram-ID", strconv.FormatInt(telegramUserID, 10))

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call backend revoke device: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, decodeAPIError(response.Body, response.StatusCode)
	}

	var result RevokeDeviceResult
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode backend revoke device response: %w", err)
	}

	return &result, nil
}

func (c *Client) EnsureTelegramSession(ctx context.Context, telegramUserID int64, username string) (*EnsureTelegramSessionResult, error) {
	requestBody, err := json.Marshal(struct {
		Username string `json:"username,omitempty"`
	}{
		Username: username,
	})
	if err != nil {
		return nil, fmt.Errorf("encode ensure telegram session request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/telegram/session", bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("build ensure telegram session request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Telegram-ID", strconv.FormatInt(telegramUserID, 10))

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call backend ensure telegram session: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, decodeAPIError(response.Body, response.StatusCode)
	}

	var result EnsureTelegramSessionResult
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode ensure telegram session response: %w", err)
	}

	return &result, nil
}

type errorResponse struct {
	Error string `json:"error"`
}

func decodeAPIError(body io.Reader, statusCode int) error {
	var payload errorResponse
	if err := json.NewDecoder(body).Decode(&payload); err == nil && payload.Error != "" {
		return &APIError{
			StatusCode: statusCode,
			Message:    payload.Error,
		}
	}

	return &APIError{StatusCode: statusCode}
}
