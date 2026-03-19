package reminders

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
)

const (
	StatusWaitingSecond = "waiting_second"
	StatusCompleted     = "completed"
	StatusPhoneReceived = "phone_received"

	firstReminderText = "Подскажите, вы ещё в поиске авто?\n" +
		"У нас как раз появилось несколько интересных вариантов — часть уже добавили в профиль.\n" +
		"Могу отправить фото и честно рассказать, какие действительно стоят внимания."

	secondReminderText = "Е.АВТО на связи 🤙 и в нашем профиле появилось уже более 30 новых объявлений. Выбери свой!"
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
	ListDialogs(context.Context, time.Time, time.Time) ([]Dialog, error)
	SendMessage(ctx context.Context, clientID, text string) error
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

type ChatState struct {
	ChatID           int64
	ClientID         string
	Status           string
	Phone            string
	LastMessageAt    time.Time
	FirstReminderAt  *time.Time
	SecondReminderAt *time.Time
	UpdatedAt        time.Time
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
	dryRun      bool
}

func NewService(source Source, store Store, logger *zap.Logger, metrics Metrics, now func() time.Time, lookback time.Duration, dryRun bool) *Service {
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
		secondDelay: 96 * time.Hour, // 4 days after first reminder = day 7
		dryRun:      dryRun,
	}
}

func (s *Service) Run(ctx context.Context) error {
	now := s.now().UTC()
	dialogs, err := s.source.ListDialogs(ctx, now.Add(-s.lookback), now)
	if err != nil {
		s.metrics.ObserveRun("failed")
		return fmt.Errorf("list dialogs: %w", err)
	}

	s.metrics.ObserveCandidates(len(dialogs))

	for _, dialog := range dialogs {
		if err := s.processDialog(ctx, now, dialog); err != nil {
			s.metrics.ObserveRun("failed")
			return err
		}
	}

	s.metrics.ObserveRun("success")
	return nil
}

func (s *Service) processDialog(ctx context.Context, now time.Time, dialog Dialog) error {
	lastMsg, ok := lastMessage(dialog.Messages)
	if !ok {
		return nil
	}

	state, exists, err := s.store.Get(ctx, dialog.ID)
	if err != nil {
		return fmt.Errorf("load state for dialog %d: %w", dialog.ID, err)
	}

	phone := strings.TrimSpace(dialog.Phone)

	// First reminder: no state yet, phone absent, 3+ days since last message (from anyone).
	if !exists && phone == "" && now.Sub(lastMsg.SentAt) >= s.firstDelay {
		firstAt := now
		state = ChatState{
			ChatID:          dialog.ID,
			ClientID:        dialog.ClientID,
			Status:          StatusWaitingSecond,
			LastMessageAt:   lastMsg.SentAt,
			FirstReminderAt: &firstAt,
			UpdatedAt:       now,
		}
		if err := s.send(ctx, dialog, firstReminderText); err != nil {
			return fmt.Errorf("send first reminder for dialog %d: %w", dialog.ID, err)
		}
		if err := s.store.Save(ctx, state); err != nil {
			return fmt.Errorf("save first reminder state for dialog %d: %w", dialog.ID, err)
		}
		s.metrics.ObserveFirstReminder()
		s.logger.Info("sent first reminder",
			zap.Int64("chat_id", dialog.ID),
			zap.String("client_id", dialog.ClientID),
			zap.Time("last_message_at", lastMsg.SentAt),
			zap.Bool("dry_run", s.dryRun),
		)
		return nil
	}

	if !exists || state.Status != StatusWaitingSecond {
		return nil
	}

	// Cancel: phone appeared between first and second reminder.
	if phone != "" {
		state.Status = StatusPhoneReceived
		state.Phone = phone
		state.UpdatedAt = now
		if err := s.store.Save(ctx, state); err != nil {
			return fmt.Errorf("save phone-received state for dialog %d: %w", dialog.ID, err)
		}
		s.metrics.ObserveCancellation()
		s.logger.Info("cancelled second reminder: phone received",
			zap.Int64("chat_id", dialog.ID),
			zap.String("client_id", dialog.ClientID),
			zap.String("phone", phone),
		)
		return nil
	}

	// Second reminder: 4+ days since first reminder (= ~day 7 from last message).
	if now.Sub(*state.FirstReminderAt) < s.secondDelay {
		return nil
	}

	secondAt := now
	state.Status = StatusCompleted
	state.SecondReminderAt = &secondAt
	state.UpdatedAt = now
	if err := s.send(ctx, dialog, secondReminderText); err != nil {
		return fmt.Errorf("send second reminder for dialog %d: %w", dialog.ID, err)
	}
	if err := s.store.Save(ctx, state); err != nil {
		return fmt.Errorf("save second reminder state for dialog %d: %w", dialog.ID, err)
	}
	s.metrics.ObserveSecondReminder()
	s.logger.Info("sent second reminder",
		zap.Int64("chat_id", dialog.ID),
		zap.String("client_id", dialog.ClientID),
		zap.Time("last_message_at", lastMsg.SentAt),
		zap.Bool("dry_run", s.dryRun),
	)
	return nil
}

// send sends a message or logs it in dry-run mode.
func (s *Service) send(ctx context.Context, dialog Dialog, text string) error {
	if s.dryRun {
		s.logger.Info("[DRY RUN] would send message",
			zap.Int64("dialog_id", dialog.ID),
			zap.String("client_id", dialog.ClientID),
			zap.String("text", text),
		)
		return nil
	}
	return s.source.SendMessage(ctx, dialog.ClientID, text)
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
