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

const bachataTimeLayout = "2006-01-02 15:04:05"

type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

type Client struct {
	baseURL string
	token   string
	doer    Doer
}

type Message struct {
	ID          int       `json:"id"`
	DialogID    int64     `json:"dialogId"`
	WhoSend     string    `json:"whoSend"`
	Text        string    `json:"text"`
	DateTimeUTC time.Time `json:"dateTimeUTC"`
}

type Dialog struct {
	ID       int64      `json:"dialogId"`
	ClientID string     `json:"clientId"`
	Client   ChatClient `json:"client"`
	Messages []Message  `json:"messages"`
}

type ChatClient struct {
	ClientID string `json:"clientId"`
	Phone    string `json:"phone"`
	Name     string `json:"name"`
}

type MessagePage struct {
	Items []Message
	Count int
	Limit int
}

func NewClient(baseURL, token string, doer Doer) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		doer:    doer,
	}
}

func (c *Client) ListRecentClientMessages(ctx context.Context, start, stop time.Time, page, limit int) (MessagePage, error) {
	reqBody := map[string]any{
		"page":   page,
		"limit":  limit,
		"sender": "client",
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
		Result struct {
			Count int          `json:"count"`
			Limit int          `json:"limit"`
			Items []rawMessage `json:"items"`
		} `json:"result"`
	}

	if err := c.post(ctx, "/chat/message/getList", reqBody, &resp); err != nil {
		return MessagePage{}, err
	}

	items, err := normalizeMessages(resp.Result.Items)
	if err != nil {
		return MessagePage{}, err
	}

	return MessagePage{
		Items: items,
		Count: resp.Result.Count,
		Limit: resp.Result.Limit,
	}, nil
}

func (c *Client) GetDialog(ctx context.Context, dialogID int64) (Dialog, error) {
	reqBody := map[string]any{"dialogId": dialogID}

	var resp struct {
		Success bool `json:"success"`
		Error   struct {
			Code  int    `json:"code"`
			Descr string `json:"descr"`
		} `json:"error"`
		Result struct {
			Dialog   rawDialog    `json:"dialog"`
			Messages []rawMessage `json:"messages"`
		} `json:"result"`
	}

	if err := c.post(ctx, "/chat/message/getDialog", reqBody, &resp); err != nil {
		return Dialog{}, err
	}

	messages, err := normalizeMessages(resp.Result.Messages)
	if err != nil {
		return Dialog{}, err
	}

	return Dialog{
		ID:       resp.Result.Dialog.DialogID,
		ClientID: resp.Result.Dialog.ClientID,
		Client: ChatClient{
			ClientID: resp.Result.Dialog.Client.ClientID,
			Phone:    strings.TrimSpace(resp.Result.Dialog.Client.Phone),
			Name:     resp.Result.Dialog.Client.Name,
		},
		Messages: messages,
	}, nil
}

func (c *Client) post(ctx context.Context, path string, payload any, dst any) error {
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

type rawMessage struct {
	ID          int    `json:"id"`
	DialogID    int64  `json:"dialogId"`
	WhoSend     string `json:"whoSend"`
	Text        string `json:"text"`
	DateTimeUTC string `json:"dateTimeUTC"`
}

type rawDialog struct {
	DialogID int64         `json:"dialogId"`
	ClientID string        `json:"clientId"`
	Client   rawChatClient `json:"client"`
}

type rawChatClient struct {
	ClientID string `json:"clientId"`
	Phone    string `json:"phone"`
	Name     string `json:"name"`
}

func normalizeMessages(raw []rawMessage) ([]Message, error) {
	items := make([]Message, 0, len(raw))
	for _, msg := range raw {
		at, err := time.Parse(bachataTimeLayout, msg.DateTimeUTC)
		if err != nil {
			return nil, fmt.Errorf("parse message time: %w", err)
		}

		items = append(items, Message{
			ID:          msg.ID,
			DialogID:    msg.DialogID,
			WhoSend:     msg.WhoSend,
			Text:        msg.Text,
			DateTimeUTC: at.UTC(),
		})
	}

	return items, nil
}
