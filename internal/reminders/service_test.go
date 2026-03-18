package reminders

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

type fakeSource struct {
	list    []Message
	dialogs map[int64]Dialog
}

func (f *fakeSource) ListRecentClientMessages(context.Context, time.Time, time.Time) ([]Message, error) {
	return f.list, nil
}

func (f *fakeSource) GetDialog(_ context.Context, id int64) (Dialog, error) {
	return f.dialogs[id], nil
}

type memoryStore struct {
	items map[int64]ChatState
}

func newMemoryStore() *memoryStore {
	return &memoryStore{items: map[int64]ChatState{}}
}

func (m *memoryStore) Get(_ context.Context, chatID int64) (ChatState, bool, error) {
	item, ok := m.items[chatID]
	return item, ok, nil
}

func (m *memoryStore) Save(_ context.Context, state ChatState) error {
	m.items[state.ChatID] = state
	return nil
}

type noopMetrics struct{}

func (noopMetrics) ObserveRun(string)      {}
func (noopMetrics) ObserveCandidates(int)  {}
func (noopMetrics) ObserveFirstReminder()  {}
func (noopMetrics) ObserveSecondReminder() {}
func (noopMetrics) ObserveCancellation()   {}

func TestServiceSendsFirstReminderOnDayThree(t *testing.T) {
	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	store := newMemoryStore()
	service := NewService(&fakeSource{
		list: []Message{{DialogID: 10, WhoSend: "client", SentAt: now.Add(-73 * time.Hour)}},
		dialogs: map[int64]Dialog{
			10: {
				ID:       10,
				ClientID: "c-1",
				Messages: []Message{{DialogID: 10, WhoSend: "client", SentAt: now.Add(-73 * time.Hour)}},
			},
		},
	}, store, zap.NewNop(), noopMetrics{}, func() time.Time { return now }, 8*24*time.Hour)

	if err := service.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	state := store.items[10]
	if state.Status != StatusWaitingSecond || state.FirstReminderAt == nil {
		t.Fatalf("state = %#v", state)
	}
}

func TestServiceSendsSecondReminderOnDaySeven(t *testing.T) {
	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	firstAt := now.Add(-4 * 24 * time.Hour)
	store := newMemoryStore()
	store.items[10] = ChatState{
		ChatID:              10,
		ClientID:            "c-1",
		Status:              StatusWaitingSecond,
		LastClientMessageAt: now.Add(-8 * 24 * time.Hour),
		FirstReminderAt:     &firstAt,
	}

	service := NewService(&fakeSource{
		list: []Message{{DialogID: 10, WhoSend: "client", SentAt: now.Add(-8 * 24 * time.Hour)}},
		dialogs: map[int64]Dialog{
			10: {
				ID:       10,
				ClientID: "c-1",
				Messages: []Message{{DialogID: 10, WhoSend: "client", SentAt: now.Add(-8 * 24 * time.Hour)}},
			},
		},
	}, store, zap.NewNop(), noopMetrics{}, func() time.Time { return now }, 8*24*time.Hour)

	if err := service.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	state := store.items[10]
	if state.Status != StatusCompleted || state.SecondReminderAt == nil {
		t.Fatalf("state = %#v", state)
	}
}

func TestServiceCancelsSecondReminderWhenPhoneArrives(t *testing.T) {
	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	firstAt := now.Add(-4 * 24 * time.Hour)
	store := newMemoryStore()
	store.items[10] = ChatState{
		ChatID:              10,
		ClientID:            "c-1",
		Status:              StatusWaitingSecond,
		LastClientMessageAt: now.Add(-5 * 24 * time.Hour),
		FirstReminderAt:     &firstAt,
	}

	service := NewService(&fakeSource{
		list: []Message{{DialogID: 10, WhoSend: "client", SentAt: now.Add(-5 * 24 * time.Hour)}},
		dialogs: map[int64]Dialog{
			10: {
				ID:       10,
				ClientID: "c-1",
				Phone:    "79001234567",
				Messages: []Message{{DialogID: 10, WhoSend: "client", SentAt: now.Add(-5 * 24 * time.Hour)}},
			},
		},
	}, store, zap.NewNop(), noopMetrics{}, func() time.Time { return now }, 8*24*time.Hour)

	if err := service.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	state := store.items[10]
	if state.Status != StatusPhoneReceived || state.Phone == "" {
		t.Fatalf("state = %#v", state)
	}
}

func TestServiceSkipsDialogsWhereOperatorWroteLast(t *testing.T) {
	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	store := newMemoryStore()
	service := NewService(&fakeSource{
		list: []Message{{DialogID: 10, WhoSend: "client", SentAt: now.Add(-73 * time.Hour)}},
		dialogs: map[int64]Dialog{
			10: {
				ID:       10,
				ClientID: "c-1",
				Messages: []Message{
					{DialogID: 10, WhoSend: "client", SentAt: now.Add(-73 * time.Hour)},
					{DialogID: 10, WhoSend: "operator", SentAt: now.Add(-72 * time.Hour)},
				},
			},
		},
	}, store, zap.NewNop(), noopMetrics{}, func() time.Time { return now }, 8*24*time.Hour)

	if err := service.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(store.items) != 0 {
		t.Fatalf("unexpected state: %#v", store.items)
	}
}
