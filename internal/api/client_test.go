package api

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) Do(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestListRecentClientMessages(t *testing.T) {
	client := NewClient("https://example.com/", "secret", roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://example.com/chat/message/getList" {
			t.Fatalf("URL = %s", req.URL.String())
		}
		if req.Method != http.MethodPost {
			t.Fatalf("Method = %s", req.Method)
		}
		if got := req.Header.Get("X-Token"); got != "secret" {
			t.Fatalf("X-Token = %s", got)
		}

		body, _ := io.ReadAll(req.Body)
		if !strings.Contains(string(body), `"sender":"client"`) {
			t.Fatalf("body = %s", string(body))
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"success": true,
				"result": {
					"count": 1,
					"limit": 200,
					"items": [{
						"id": 10,
						"dialogId": 99,
						"whoSend": "client",
						"text": "hello",
						"dateTimeUTC": "2026-03-15 12:00:00"
					}]
				}
			}`)),
		}, nil
	}))

	page, err := client.ListRecentClientMessages(context.Background(), time.Now().Add(-time.Hour), time.Now(), 0, 200)
	if err != nil {
		t.Fatalf("ListRecentClientMessages() error = %v", err)
	}

	if len(page.Items) != 1 || page.Items[0].DialogID != 99 {
		t.Fatalf("Items = %#v", page.Items)
	}
}

func TestGetDialog(t *testing.T) {
	client := NewClient("https://example.com", "secret", roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"success": true,
				"result": {
					"dialog": {
						"dialogId": 77,
						"clientId": "client-1",
						"client": {
							"clientId": "client-1",
							"phone": "",
							"name": "Ivan"
						}
					},
					"messages": [{
						"id": 1,
						"dialogId": 77,
						"whoSend": "client",
						"text": "hi",
						"dateTimeUTC": "2026-03-15 12:00:00"
					}]
				}
			}`)),
		}, nil
	}))

	dialog, err := client.GetDialog(context.Background(), 77)
	if err != nil {
		t.Fatalf("GetDialog() error = %v", err)
	}

	if dialog.ID != 77 || dialog.Client.ClientID != "client-1" || len(dialog.Messages) != 1 {
		t.Fatalf("Dialog = %#v", dialog)
	}
}
