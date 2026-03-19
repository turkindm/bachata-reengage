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

func TestListDialogs(t *testing.T) {
	client := NewClient("https://example.com/", "secret", "operator", roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://example.com/chat/message/getList" {
			t.Fatalf("URL = %s", req.URL.String())
		}
		if req.Method != http.MethodPost {
			t.Fatalf("Method = %s", req.Method)
		}
		if got := req.Header.Get("X-Token"); got != "secret" {
			t.Fatalf("X-Token = %s", got)
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"success": true,
				"result": [
					{
						"clientId": "client-abc",
						"phone": "79001234567",
						"messages": [
							{
								"id": 10,
								"dialogId": 99,
								"whoSend": "client",
								"text": "hello",
								"dateTimeUTC": "2026-03-15 12:00:00"
							},
							{
								"id": 11,
								"dialogId": 99,
								"whoSend": "operator",
								"text": "hi there",
								"dateTimeUTC": "2026-03-15 13:00:00"
							}
						]
					}
				]
			}`)),
		}, nil
	}), 0)

	dialogs, err := client.ListDialogs(context.Background(), time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatalf("ListDialogs() error = %v", err)
	}

	if len(dialogs) != 1 {
		t.Fatalf("expected 1 dialog, got %d", len(dialogs))
	}

	d := dialogs[0]
	if d.ID != 99 {
		t.Fatalf("dialog.ID = %d, want 99", d.ID)
	}
	if d.ClientID != "client-abc" {
		t.Fatalf("dialog.ClientID = %q, want client-abc", d.ClientID)
	}
	if d.Phone != "79001234567" {
		t.Fatalf("dialog.Phone = %q, want 79001234567", d.Phone)
	}
	if len(d.Messages) != 2 {
		t.Fatalf("len(messages) = %d, want 2", len(d.Messages))
	}

	want := time.Date(2026, 3, 15, 13, 0, 0, 0, time.UTC)
	if !d.Messages[1].DateTimeUTC.Equal(want) {
		t.Fatalf("DateTimeUTC = %v, want %v", d.Messages[1].DateTimeUTC, want)
	}
}

func TestListDialogsMultipleClients(t *testing.T) {
	client := NewClient("https://example.com", "secret", "operator", roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"success": true,
				"result": [
					{
						"clientId": "c1",
						"phone": "",
						"messages": [
							{"id": 1, "dialogId": 10, "whoSend": "client",   "text": "hi",    "dateTimeUTC": "2026-03-10 10:00:00"},
							{"id": 2, "dialogId": 10, "whoSend": "operator", "text": "hello", "dateTimeUTC": "2026-03-11 10:00:00"}
						]
					},
					{
						"clientId": "c2",
						"phone": "79009998877",
						"messages": [
							{"id": 3, "dialogId": 20, "whoSend": "client", "text": "hey", "dateTimeUTC": "2026-03-12 10:00:00"}
						]
					}
				]
			}`)),
		}, nil
	}), 0)

	dialogs, err := client.ListDialogs(context.Background(), time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatalf("ListDialogs() error = %v", err)
	}

	if len(dialogs) != 2 {
		t.Fatalf("expected 2 dialogs, got %d", len(dialogs))
	}

	// Find by ID for stable assertions regardless of map iteration order.
	byID := make(map[int64]Dialog, len(dialogs))
	for _, d := range dialogs {
		byID[d.ID] = d
	}

	d10 := byID[10]
	if d10.ClientID != "c1" || d10.Phone != "" || len(d10.Messages) != 2 {
		t.Fatalf("dialog 10 = %+v", d10)
	}

	d20 := byID[20]
	if d20.ClientID != "c2" || d20.Phone != "79009998877" || len(d20.Messages) != 1 {
		t.Fatalf("dialog 20 = %+v", d20)
	}
}

func TestListDialogsNoPhone(t *testing.T) {
	client := NewClient("https://example.com", "secret", "operator", roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"success": true,
				"result": [
					{
						"clientId": "c-no-phone",
						"messages": [
							{"id": 1, "dialogId": 50, "whoSend": "client", "text": "hi", "dateTimeUTC": "2026-03-15 12:00:00"}
						]
					}
				]
			}`)),
		}, nil
	}), 0)

	dialogs, err := client.ListDialogs(context.Background(), time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatalf("ListDialogs() error = %v", err)
	}

	if len(dialogs) != 1 || dialogs[0].Phone != "" {
		t.Fatalf("expected 1 dialog with empty phone, got %+v", dialogs)
	}
}
