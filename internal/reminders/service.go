package reminders

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"go.uber.org/zap"
)

const (
	StatusWaitingSecond = "waiting_second"
	StatusCompleted     = "completed"
	StatusPhoneReceived = "phone_received"
)

type Message struct {
	DialogID int64
	WhoSend  string
	SentAt   time.Time
}

type Dialog struct {
	ID       int64
	ClientID string
	Phone    string
	Messages []Message
}

type Source interface {
	ListRecentClientMessages(context.Context, time.Time, time.Time) ([]Message, error)
	GetDialog(context.Context, int64) (Dialog, error)
}

type Store interface {
	Get(context.Context, int64) (ChatState, bool, error)
	Save(context.Context, ChatState) error
}

type Metrics interface {
	ObserveRun(string)
	ObserveCandidates(int)
	ObserveFirstReminder()
	ObserveSecondReminder()
	ObserveCancellation()
}

type Service struct {
	source      Source
	store       Store
	logger      *zap.Logger
	metrics     Metrics
	now         func() time.Time
	lookback    time.Duration
	firstDelay  time.Duration
	secondDelay time.Duration
}

type ChatState struct {
	ChatID              int64
	ClientID            string
	Status              string
	Phone               string
	LastClientMessageAt time.Time
	FirstReminderAt     *time.Time
	SecondReminderAt    *time.Time
	UpdatedAt           time.Time
}

func NewService(source Source, store Store, logger *zap.Logger, metrics Metrics, now func() time.Time, lookback time.Duration) *Service {
	if now == nil {
		now = time.Now
	}

	return &Service{
		source:      source,
		store:       store,
		logger:      logger,
		metrics:     metrics,
		now:         now,
		lookback:    lookback,
		firstDelay:  72 * time.Hour,
		secondDelay: 168 * time.Hour,
	}
}

func (s *Service) Run(ctx context.Context) error {
	now := s.now().UTC()
	candidates, err := s.source.ListRecentClientMessages(ctx, now.Add(-s.lookback), now)
	if err != nil {
		s.metrics.ObserveRun("failed")
		return fmt.Errorf("list candidate messages: %w", err)
	}

	latest := latestByDialog(candidates)
	s.metrics.ObserveCandidates(len(latest))

	for _, msg := range latest {
		if err := s.processDialog(ctx, now, msg); err != nil {
			s.metrics.ObserveRun("failed")
			return err
		}
	}

	s.metrics.ObserveRun("success")
	return nil
}

func (s *Service) processDialog(ctx context.Context, now time.Time, candidate Message) error {
	dialog, err := s.source.GetDialog(ctx, candidate.DialogID)
	if err != nil {
		return fmt.Errorf("get dialog %d: %w", candidate.DialogID, err)
	}

	lastMessage, ok := lastMessage(dialog.Messages)
	if !ok || lastMessage.WhoSend != "client" {
		return nil
	}

	state, exists, err := s.store.Get(ctx, dialog.ID)
	if err != nil {
		return fmt.Errorf("load state for dialog %d: %w", dialog.ID, err)
	}

	phone := strings.TrimSpace(dialog.Phone)
	if phone == "" && !exists && now.Sub(lastMessage.SentAt) >= s.firstDelay {
		firstAt := now
		state = ChatState{
			ChatID:              dialog.ID,
			ClientID:            dialog.ClientID,
			Status:              StatusWaitingSecond,
			LastClientMessageAt: lastMessage.SentAt,
			FirstReminderAt:     &firstAt,
			UpdatedAt:           now,
		}
		if err := s.store.Save(ctx, state); err != nil {
			return fmt.Errorf("save first reminder state for dialog %d: %w", dialog.ID, err)
		}
		s.metrics.ObserveFirstReminder()
		s.logger.Info("simulated first reminder",
			zap.Int64("chat_id", dialog.ID),
			zap.String("client_id", dialog.ClientID),
			zap.Time("last_client_message_at", lastMessage.SentAt),
		)
		return nil
	}

	if !exists || state.Status != StatusWaitingSecond {
		return nil
	}

	if phone != "" {
		state.Status = StatusPhoneReceived
		state.Phone = phone
		state.UpdatedAt = now
		if err := s.store.Save(ctx, state); err != nil {
			return fmt.Errorf("save phone-received state for dialog %d: %w", dialog.ID, err)
		}
		s.metrics.ObserveCancellation()
		s.logger.Info("cancelled second reminder because phone was received",
			zap.Int64("chat_id", dialog.ID),
			zap.String("client_id", dialog.ClientID),
			zap.String("phone", phone),
		)
		return nil
	}

	if now.Sub(lastMessage.SentAt) < s.secondDelay {
		return nil
	}

	secondAt := now
	state.Status = StatusCompleted
	state.SecondReminderAt = &secondAt
	state.UpdatedAt = now
	if err := s.store.Save(ctx, state); err != nil {
		return fmt.Errorf("save second reminder state for dialog %d: %w", dialog.ID, err)
	}

	s.metrics.ObserveSecondReminder()
	s.logger.Info("simulated second reminder",
		zap.Int64("chat_id", dialog.ID),
		zap.String("client_id", dialog.ClientID),
		zap.Time("last_client_message_at", lastMessage.SentAt),
	)
	return nil
}

func latestByDialog(messages []Message) []Message {
	byDialog := make(map[int64]Message, len(messages))
	for _, msg := range messages {
		if current, ok := byDialog[msg.DialogID]; !ok || msg.SentAt.After(current.SentAt) {
			byDialog[msg.DialogID] = msg
		}
	}

	items := make([]Message, 0, len(byDialog))
	for _, msg := range byDialog {
		items = append(items, msg)
	}

	slices.SortFunc(items, func(a, b Message) int {
		switch {
		case a.SentAt.Before(b.SentAt):
			return -1
		case a.SentAt.After(b.SentAt):
			return 1
		default:
			return 0
		}
	})

	return items
}

func lastMessage(messages []Message) (Message, bool) {
	if len(messages) == 0 {
		return Message{}, false
	}

	last := messages[0]
	for _, msg := range messages[1:] {
		if msg.SentAt.After(last.SentAt) {
			last = msg
		}
	}

	return last, true
}
