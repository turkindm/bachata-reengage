package reminders

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

type fakeSource struct {
	dialogs []Dialog
	sent    []string // collected texts from SendMessage calls
}

func (f *fakeSource) ListDialogs(_ context.Context, _, _ time.Time) ([]Dialog, error) {
	return f.dialogs, nil
}

func (f *fakeSource) SendMessage(_ context.Context, _, text string) error {
	f.sent = append(f.sent, text)
	return nil
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

func newService(src *fakeSource, st *memoryStore) *Service {
	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	return NewService(src, st, zap.NewNop(), noopMetrics{}, func() time.Time { return now }, 8*24*time.Hour, 72*time.Hour, 96*time.Hour, false, 0)
}

func TestServiceSendsFirstReminderOnDayThree(t *testing.T) {
	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	st := newMemoryStore()
	src := &fakeSource{
		dialogs: []Dialog{{
			ID:       10,
			ClientID: "c-1",
			Messages: []Message{{DialogID: 10, WhoSend: "client", SentAt: now.Add(-73 * time.Hour)}},
		}},
	}
	svc := NewService(src, st, zap.NewNop(), noopMetrics{}, func() time.Time { return now }, 8*24*time.Hour, 72*time.Hour, 96*time.Hour, false, 0)

	if err := svc.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	state := st.items[10]
	if state.Status != StatusWaitingSecond || state.FirstReminderAt == nil {
		t.Fatalf("unexpected state = %#v", state)
	}
	if len(src.sent) != 1 || src.sent[0] != firstReminderText {
		t.Fatalf("unexpected sent messages = %v", src.sent)
	}
}

func TestServiceSkipsFirstReminderBefore72Hours(t *testing.T) {
	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	st := newMemoryStore()
	src := &fakeSource{
		dialogs: []Dialog{{
			ID:       10,
			ClientID: "c-1",
			Messages: []Message{{DialogID: 10, WhoSend: "client", SentAt: now.Add(-48 * time.Hour)}},
		}},
	}
	svc := NewService(src, st, zap.NewNop(), noopMetrics{}, func() time.Time { return now }, 8*24*time.Hour, 72*time.Hour, 96*time.Hour, false, 0)

	if err := svc.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(st.items) != 0 {
		t.Fatalf("expected no state, got %#v", st.items)
	}
	if len(src.sent) != 0 {
		t.Fatalf("expected no messages sent, got %v", src.sent)
	}
}

func TestServiceSendsFirstReminderAfterOperatorWroteLast(t *testing.T) {
	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	st := newMemoryStore()
	src := &fakeSource{
		dialogs: []Dialog{{
			ID:       10,
			ClientID: "c-1",
			Messages: []Message{
				{DialogID: 10, WhoSend: "client", SentAt: now.Add(-73 * time.Hour)},
				{DialogID: 10, WhoSend: "operator", SentAt: now.Add(-72 * time.Hour)},
			},
		}},
	}
	svc := NewService(src, st, zap.NewNop(), noopMetrics{}, func() time.Time { return now }, 8*24*time.Hour, 72*time.Hour, 96*time.Hour, false, 0)

	if err := svc.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Last message is from operator 72h ago — exactly at firstDelay threshold, should fire.
	state := st.items[10]
	if state.Status != StatusWaitingSecond || state.FirstReminderAt == nil {
		t.Fatalf("unexpected state = %#v", state)
	}
	if len(src.sent) != 1 {
		t.Fatalf("expected 1 message sent, got %v", src.sent)
	}
}

func TestServiceSendsSecondReminderOnDaySeven(t *testing.T) {
	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	// First reminder sent exactly 4 days ago → second reminder threshold met.
	firstAt := now.Add(-4 * 24 * time.Hour)
	st := newMemoryStore()
	st.items[10] = ChatState{
		ChatID:          10,
		ClientID:        "c-1",
		Status:          StatusWaitingSecond,
		LastMessageAt:   now.Add(-7 * 24 * time.Hour),
		FirstReminderAt: &firstAt,
	}

	src := &fakeSource{
		dialogs: []Dialog{{
			ID:       10,
			ClientID: "c-1",
			Messages: []Message{{DialogID: 10, WhoSend: "client", SentAt: now.Add(-7 * 24 * time.Hour)}},
		}},
	}
	svc := NewService(src, st, zap.NewNop(), noopMetrics{}, func() time.Time { return now }, 8*24*time.Hour, 72*time.Hour, 96*time.Hour, false, 0)

	if err := svc.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	state := st.items[10]
	if state.Status != StatusCompleted || state.SecondReminderAt == nil {
		t.Fatalf("unexpected state = %#v", state)
	}
	if len(src.sent) != 1 || src.sent[0] != secondReminderText {
		t.Fatalf("unexpected sent messages = %v", src.sent)
	}
}

func TestServiceSkipsSecondReminderBefore4Days(t *testing.T) {
	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	// First reminder sent only 3 days ago — too early for second.
	firstAt := now.Add(-3 * 24 * time.Hour)
	st := newMemoryStore()
	st.items[10] = ChatState{
		ChatID:          10,
		ClientID:        "c-1",
		Status:          StatusWaitingSecond,
		LastMessageAt:   now.Add(-6 * 24 * time.Hour),
		FirstReminderAt: &firstAt,
	}

	src := &fakeSource{
		dialogs: []Dialog{{
			ID:       10,
			ClientID: "c-1",
			Messages: []Message{{DialogID: 10, WhoSend: "client", SentAt: now.Add(-6 * 24 * time.Hour)}},
		}},
	}
	svc := NewService(src, st, zap.NewNop(), noopMetrics{}, func() time.Time { return now }, 8*24*time.Hour, 72*time.Hour, 96*time.Hour, false, 0)

	if err := svc.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	state := st.items[10]
	if state.Status != StatusWaitingSecond {
		t.Fatalf("expected status waiting_second, got %q", state.Status)
	}
	if len(src.sent) != 0 {
		t.Fatalf("expected no messages sent, got %v", src.sent)
	}
}

func TestServiceCancelsSecondReminderWhenPhoneArrives(t *testing.T) {
	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	firstAt := now.Add(-4 * 24 * time.Hour)
	st := newMemoryStore()
	st.items[10] = ChatState{
		ChatID:          10,
		ClientID:        "c-1",
		Status:          StatusWaitingSecond,
		LastMessageAt:   now.Add(-5 * 24 * time.Hour),
		FirstReminderAt: &firstAt,
	}

	src := &fakeSource{
		dialogs: []Dialog{{
			ID:       10,
			ClientID: "c-1",
			Phone:    "79001234567",
			Messages: []Message{{DialogID: 10, WhoSend: "client", SentAt: now.Add(-5 * 24 * time.Hour)}},
		}},
	}
	svc := NewService(src, st, zap.NewNop(), noopMetrics{}, func() time.Time { return now }, 8*24*time.Hour, 72*time.Hour, 96*time.Hour, false, 0)

	if err := svc.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	state := st.items[10]
	if state.Status != StatusPhoneReceived || state.Phone == "" {
		t.Fatalf("unexpected state = %#v", state)
	}
	if len(src.sent) != 0 {
		t.Fatalf("expected no messages sent on cancellation, got %v", src.sent)
	}
}

func TestServiceSkipsAlreadyCompletedDialogs(t *testing.T) {
	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	secondAt := now.Add(-1 * 24 * time.Hour)
	st := newMemoryStore()
	st.items[10] = ChatState{
		ChatID:           10,
		ClientID:         "c-1",
		Status:           StatusCompleted,
		SecondReminderAt: &secondAt,
	}

	src := &fakeSource{
		dialogs: []Dialog{{
			ID:       10,
			ClientID: "c-1",
			Messages: []Message{{DialogID: 10, WhoSend: "client", SentAt: now.Add(-8 * 24 * time.Hour)}},
		}},
	}

	if err := newService(src, st).Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	state := st.items[10]
	if state.Status != StatusCompleted {
		t.Fatalf("unexpected state = %#v", state)
	}
	if len(src.sent) != 0 {
		t.Fatalf("expected no messages sent, got %v", src.sent)
	}
}

func TestServiceDryRunDoesNotCallSendMessage(t *testing.T) {
	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	st := newMemoryStore()
	src := &fakeSource{
		dialogs: []Dialog{{
			ID:       10,
			ClientID: "c-1",
			Messages: []Message{{DialogID: 10, WhoSend: "client", SentAt: now.Add(-73 * time.Hour)}},
		}},
	}
	svc := NewService(src, st, zap.NewNop(), noopMetrics{}, func() time.Time { return now }, 8*24*time.Hour, 72*time.Hour, 96*time.Hour, true, 0)

	if err := svc.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// State should be saved but no real API call made.
	state := st.items[10]
	if state.Status != StatusWaitingSecond {
		t.Fatalf("unexpected state = %#v", state)
	}
	if len(src.sent) != 0 {
		t.Fatalf("dry run should not call SendMessage, got %v", src.sent)
	}
}
