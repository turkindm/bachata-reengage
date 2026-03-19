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

		// Real API returns an array of clients, each with embedded messages.
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"success": true,
				"result": [
					{
						"clientId": "client-abc",
						"messages": [
							{
								"id": 10,
								"dialogId": 99,
								"whoSend": "client",
								"text": "hello",
								"dateTimeUTC": "2026-03-15 12:00:00"
							}
						],
						"operators": []
					}
				]
			}`)),
		}, nil
	}), 0)

	msgs, err := client.ListRecentClientMessages(context.Background(), time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatalf("ListRecentClientMessages() error = %v", err)
	}

	if len(msgs) != 1 || msgs[0].DialogID != 99 {
		t.Fatalf("msgs = %#v", msgs)
	}

	want := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	if !msgs[0].DateTimeUTC.Equal(want) {
		t.Fatalf("DateTimeUTC = %v, want %v", msgs[0].DateTimeUTC, want)
	}
}

func TestListRecentClientMessagesMultipleClients(t *testing.T) {
	client := NewClient("https://example.com", "secret", roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"success": true,
				"result": [
					{
						"clientId": "c1",
						"messages": [
							{"id": 1, "dialogId": 10, "whoSend": "client", "text": "hi", "dateTimeUTC": "2026-03-10 10:00:00"},
							{"id": 2, "dialogId": 10, "whoSend": "client", "text": "there", "dateTimeUTC": "2026-03-11 10:00:00"}
						],
						"operators": []
					},
					{
						"clientId": "c2",
						"messages": [
							{"id": 3, "dialogId": 20, "whoSend": "client", "text": "hey", "dateTimeUTC": "2026-03-12 10:00:00"}
						],
						"operators": []
					}
				]
			}`)),
		}, nil
	}), 0)

	msgs, err := client.ListRecentClientMessages(context.Background(), time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatalf("ListRecentClientMessages() error = %v", err)
	}

	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
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
							"phone": "79001234567"
						}
					},
					"messages": [
						{
							"id": 1,
							"dialogId": 77,
							"whoSend": "client",
							"text": "hi",
							"dateTimeUTC": "2026-03-15 12:00:00"
						},
						{
							"id": 2,
							"dialogId": 77,
							"whoSend": "operator",
							"text": "hello",
							"dateTimeUTC": "2026-03-15 13:00:00"
						}
					]
				}
			}`)),
		}, nil
	}), 0)

	dialog, err := client.GetDialog(context.Background(), 77)
	if err != nil {
		t.Fatalf("GetDialog() error = %v", err)
	}

	if dialog.ID != 77 {
		t.Fatalf("dialog.ID = %d", dialog.ID)
	}
	if dialog.ClientID != "client-1" {
		t.Fatalf("dialog.ClientID = %q", dialog.ClientID)
	}
	if dialog.Phone != "79001234567" {
		t.Fatalf("dialog.Phone = %q", dialog.Phone)
	}
	if len(dialog.Messages) != 2 {
		t.Fatalf("len(dialog.Messages) = %d", len(dialog.Messages))
	}
}

func TestGetDialogNoPhone(t *testing.T) {
	client := NewClient("https://example.com", "secret", roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"success": true,
				"result": {
					"dialog": {
						"dialogId": 50,
						"clientId": "c-no-phone",
						"client": {"clientId": "c-no-phone"}
					},
					"messages": []
				}
			}`)),
		}, nil
	}), 0)

	dialog, err := client.GetDialog(context.Background(), 50)
	if err != nil {
		t.Fatalf("GetDialog() error = %v", err)
	}

	if dialog.Phone != "" {
		t.Fatalf("expected empty phone, got %q", dialog.Phone)
	}
}
