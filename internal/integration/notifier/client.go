package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"eventbooker/internal/config"
	"eventbooker/internal/logging"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	logger     *logging.EventBookerLogger
	enabled    bool
}

type createNotificationRequest struct {
	Message   string                      `json:"message"`
	SendAt    string                      `json:"send_at"`
	Channel   string                      `json:"channel"`
	Recipient createNotificationRecipient `json:"recipient"`
}

type createNotificationRecipient struct {
	Email  string `json:"email,omitempty"`
	UserID string `json:"user_id,omitempty"`
}

func NewClient(cfg config.NotifierConfig, logger *logging.EventBookerLogger) *Client {
	return &Client{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		httpClient: &http.Client{
			Timeout: cfg.Timeout(),
		},
		logger:  logger,
		enabled: cfg.Enabled,
	}
}

func (c *Client) ScheduleEmail(ctx context.Context, email, message string, sendAt time.Time) error {
	return c.createNotification(ctx, createNotificationRequest{
		Message: message,
		SendAt:  sendAt.UTC().Format(time.RFC3339),
		Channel: "email",
		Recipient: createNotificationRecipient{
			Email: email,
		},
	})
}

func (c *Client) ScheduleTelegram(ctx context.Context, userID int64, message string, sendAt time.Time) error {
	return c.createNotification(ctx, createNotificationRequest{
		Message: message,
		SendAt:  sendAt.UTC().Format(time.RFC3339),
		Channel: "telegram",
		Recipient: createNotificationRecipient{
			UserID: fmt.Sprintf("%d", userID),
		},
	})
}

func (c *Client) createNotification(ctx context.Context, payload createNotificationRequest) error {
	if !c.enabled {
		return nil
	}

	if c.baseURL == "" {
		return fmt.Errorf("notifier base url is empty")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal notifier request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/notify", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build notifier request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send notifier request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if readErr != nil {
			return fmt.Errorf("notifier returned status %d", resp.StatusCode)
		}
		return fmt.Errorf("notifier returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	return nil
}
