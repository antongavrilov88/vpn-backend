package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message"`
}

type Message struct {
	MessageID int64  `json:"message_id"`
	Text      string `json:"text"`
	Chat      Chat   `json:"chat"`
	From      *User  `json:"from"`
}

type Chat struct {
	ID int64 `json:"id"`
}

type User struct {
	ID int64 `json:"id"`
}

type updatesResponse struct {
	OK          bool     `json:"ok"`
	Description string   `json:"description"`
	Result      []Update `json:"result"`
}

type sendMessageResponse struct {
	OK          bool    `json:"ok"`
	Description string  `json:"description"`
	Result      Message `json:"result"`
}

type sendDocumentResponse struct {
	OK          bool            `json:"ok"`
	Description string          `json:"description"`
	Result      json.RawMessage `json:"result"`
}

func NewClient(token string, timeout time.Duration) (*Client, error) {
	if token == "" {
		return nil, fmt.Errorf("telegram bot token is required")
	}

	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	return &Client{
		baseURL: "https://api.telegram.org/bot" + token,
		httpClient: &http.Client{
			Timeout: timeout + 5*time.Second,
		},
	}, nil
}

func (c *Client) GetUpdates(ctx context.Context, offset int64, timeout time.Duration) ([]Update, error) {
	query := url.Values{}
	if offset > 0 {
		query.Set("offset", strconv.FormatInt(offset, 10))
	}

	timeoutSeconds := int(timeout / time.Second)
	if timeoutSeconds <= 0 {
		timeoutSeconds = 30
	}
	query.Set("timeout", strconv.Itoa(timeoutSeconds))

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/getUpdates?"+query.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("build getUpdates request: %w", err)
	}

	var response updatesResponse
	if err := c.do(request, &response); err != nil {
		return nil, fmt.Errorf("get updates: %w", err)
	}

	return response.Result, nil
}

func (c *Client) SendMessage(ctx context.Context, chatID int64, text string) error {
	values := url.Values{}
	values.Set("chat_id", strconv.FormatInt(chatID, 10))
	values.Set("text", text)

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/sendMessage", strings.NewReader(values.Encode()))
	if err != nil {
		return fmt.Errorf("build sendMessage request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var response sendMessageResponse
	if err := c.do(request, &response); err != nil {
		return fmt.Errorf("send message: %w", err)
	}

	return nil
}

func (c *Client) SendDocument(ctx context.Context, chatID int64, fileName string, document []byte, caption string) error {
	if len(document) == 0 {
		return fmt.Errorf("document is required")
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("chat_id", strconv.FormatInt(chatID, 10)); err != nil {
		return fmt.Errorf("write chat_id field: %w", err)
	}

	if caption != "" {
		if err := writer.WriteField("caption", caption); err != nil {
			return fmt.Errorf("write caption field: %w", err)
		}
	}

	part, err := writer.CreateFormFile("document", fileName)
	if err != nil {
		return fmt.Errorf("create document form file: %w", err)
	}

	if _, err := part.Write(document); err != nil {
		return fmt.Errorf("write document body: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("close multipart writer: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/sendDocument", &body)
	if err != nil {
		return fmt.Errorf("build sendDocument request: %w", err)
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())

	var response sendDocumentResponse
	if err := c.do(request, &response); err != nil {
		return fmt.Errorf("send document: %w", err)
	}

	return nil
}

func (c *Client) do(request *http.Request, target interface{}) error {
	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return fmt.Errorf("decode telegram response: %w", err)
	}

	switch body := target.(type) {
	case *updatesResponse:
		if !body.OK {
			return fmt.Errorf("%s", body.Description)
		}
	case *sendMessageResponse:
		if !body.OK {
			return fmt.Errorf("%s", body.Description)
		}
	case *sendDocumentResponse:
		if !body.OK {
			return fmt.Errorf("%s", body.Description)
		}
	default:
		return fmt.Errorf("unsupported telegram response type")
	}

	return nil
}
