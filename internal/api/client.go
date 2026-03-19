package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// bachataTimeLayout is the datetime format used by the Bachata API ("YYYY-MM-DD HH:MM:SS").
const bachataTimeLayout = "2006-01-02 15:04:05"

type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

type Client struct {
	baseURL       string
	token         string
	operatorLogin string
	doer          Doer
	tick          <-chan time.Time // rate limiter: one token per tick
}

// Message is a single chat message returned by the API.
type Message struct {
	ID          int
	DialogID    int64
	WhoSend     string
	Text        string
	DateTimeUTC time.Time
}

// Dialog is a full dialog with all its messages.
type Dialog struct {
	ID       int64
	ClientID string
	Phone    string
	Messages []Message
}

// NewClient creates a new Bachata API client limited to ratePerMin requests per minute.
// Pass ratePerMin=0 to disable rate limiting.
func NewClient(baseURL, token, operatorLogin string, doer Doer, ratePerMin int) *Client {
	var tick <-chan time.Time
	if ratePerMin > 0 {
		interval := time.Minute / time.Duration(ratePerMin)
		tick = time.NewTicker(interval).C
	}
	return &Client{
		baseURL:       strings.TrimRight(baseURL, "/"),
		token:         token,
		operatorLogin: operatorLogin,
		doer:          doer,
		tick:          tick,
	}
}

// ListDialogs returns all dialogs that had any activity within [start, stop).
// Each dialog includes all its messages (from both client and operator) and the
// client's phone number. A single API call to chat/message/getList is made.
func (c *Client) ListDialogs(ctx context.Context, start, stop time.Time) ([]Dialog, error) {
	reqBody := map[string]any{
		"dateRange": map[string]any{
			"start":   start.UTC().Format(bachataTimeLayout),
			"stop":    stop.UTC().Format(bachataTimeLayout),
			"isLocal": false,
		},
	}

	var resp struct {
		Success bool `json:"success"`
		Error   struct {
			Code  int    `json:"code"`
			Descr string `json:"descr"`
		} `json:"error"`
		Result []rawClientWithMessages `json:"result"`
	}

	if err := c.post(ctx, "/chat/message/getList", reqBody, &resp); err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("api error %d: %s", resp.Error.Code, resp.Error.Descr)
	}

	// Group messages by dialogId; a single client can have multiple dialogs.
	byDialog := make(map[int64]*Dialog)
	for _, client := range resp.Result {
		for _, raw := range client.Messages {
			msg, err := parseMessage(raw)
			if err != nil {
				return nil, err
			}
			d, ok := byDialog[msg.DialogID]
			if !ok {
				d = &Dialog{
					ID:       msg.DialogID,
					ClientID: client.ClientID,
					Phone:    strings.TrimSpace(client.Phone),
				}
				byDialog[msg.DialogID] = d
			}
			d.Messages = append(d.Messages, msg)
		}
	}

	dialogs := make([]Dialog, 0, len(byDialog))
	for _, d := range byDialog {
		dialogs = append(dialogs, *d)
	}
	return dialogs, nil
}

// SendMessage sends a text message to a client on behalf of the configured operator.
func (c *Client) SendMessage(ctx context.Context, clientID, text string) error {
	reqBody := map[string]any{
		"client":   map[string]any{"id": clientID},
		"operator": map[string]any{"login": c.operatorLogin, "virtual": true},
		"message":  map[string]any{"text": text},
	}

	var resp struct {
		Success bool `json:"success"`
		Error   struct {
			Code  int    `json:"code"`
			Descr string `json:"descr"`
		} `json:"error"`
	}

	if err := c.post(ctx, "/chat/message/sendToClient", reqBody, &resp); err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("api error %d: %s", resp.Error.Code, resp.Error.Descr)
	}

	return nil
}

func (c *Client) wait(ctx context.Context) error {
	if c.tick == nil {
		return nil
	}
	select {
	case <-c.tick:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Client) post(ctx context.Context, path string, payload any, dst any) error {
	if err := c.wait(ctx); err != nil {
		return fmt.Errorf("rate limiter: %w", err)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Token", c.token)

	resp, err := c.doer.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}

// rawMessage matches the message JSON from the Bachata API.
type rawMessage struct {
	ID          int    `json:"id"`
	DialogID    int64  `json:"dialogId"`
	WhoSend     string `json:"whoSend"`
	Text        string `json:"text"`
	DateTimeUTC string `json:"dateTimeUTC"`
}

// rawClientWithMessages is an element in the getList result array.
type rawClientWithMessages struct {
	ClientID string       `json:"clientId"`
	Phone    string       `json:"phone"`
	Messages []rawMessage `json:"messages"`
}

func parseMessage(raw rawMessage) (Message, error) {
	at, err := time.Parse(bachataTimeLayout, raw.DateTimeUTC)
	if err != nil {
		return Message{}, fmt.Errorf("parse message time %q: %w", raw.DateTimeUTC, err)
	}

	return Message{
		ID:          raw.ID,
		DialogID:    raw.DialogID,
		WhoSend:     raw.WhoSend,
		Text:        raw.Text,
		DateTimeUTC: at.UTC(),
	}, nil
}
