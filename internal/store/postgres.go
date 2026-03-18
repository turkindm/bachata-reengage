package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/turkindm/bachata-reengage/internal/reminders"
)

type Postgres struct {
	db *sql.DB
}

func Open(ctx context.Context, dsn string) (*Postgres, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	store := &Postgres{db: db}
	if err := store.ensureSchema(ctx); err != nil {
		return nil, err
	}

	return store, nil
}

func (p *Postgres) Close() error {
	return p.db.Close()
}

func (p *Postgres) Get(ctx context.Context, chatID int64) (reminders.ChatState, bool, error) {
	row := p.db.QueryRowContext(ctx, `
		SELECT chat_id, client_id, status, COALESCE(phone, ''), last_client_message_at, first_reminder_at, second_reminder_at, updated_at
		FROM reminder_chats
		WHERE chat_id = $1
	`, chatID)

	var state reminders.ChatState
	var phone string
	var firstReminderAt sql.NullTime
	var secondReminderAt sql.NullTime
	if err := row.Scan(
		&state.ChatID,
		&state.ClientID,
		&state.Status,
		&phone,
		&state.LastClientMessageAt,
		&firstReminderAt,
		&secondReminderAt,
		&state.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return reminders.ChatState{}, false, nil
		}
		return reminders.ChatState{}, false, fmt.Errorf("scan chat state: %w", err)
	}

	state.Phone = phone
	if firstReminderAt.Valid {
		state.FirstReminderAt = &firstReminderAt.Time
	}
	if secondReminderAt.Valid {
		state.SecondReminderAt = &secondReminderAt.Time
	}

	return state, true, nil
}

func (p *Postgres) Save(ctx context.Context, state reminders.ChatState) error {
	_, err := p.db.ExecContext(ctx, `
		INSERT INTO reminder_chats (
			chat_id, client_id, status, phone, last_client_message_at, first_reminder_at, second_reminder_at, updated_at
		) VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, $7, $8)
		ON CONFLICT (chat_id) DO UPDATE SET
			client_id = EXCLUDED.client_id,
			status = EXCLUDED.status,
			phone = EXCLUDED.phone,
			last_client_message_at = EXCLUDED.last_client_message_at,
			first_reminder_at = EXCLUDED.first_reminder_at,
			second_reminder_at = EXCLUDED.second_reminder_at,
			updated_at = EXCLUDED.updated_at
	`, state.ChatID, state.ClientID, state.Status, state.Phone, state.LastClientMessageAt, state.FirstReminderAt, state.SecondReminderAt, state.UpdatedAt.UTC())
	if err != nil {
		return fmt.Errorf("save chat state: %w", err)
	}

	return nil
}

func (p *Postgres) ensureSchema(ctx context.Context) error {
	_, err := p.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS reminder_chats (
			chat_id BIGINT PRIMARY KEY,
			client_id TEXT NOT NULL,
			status TEXT NOT NULL,
			phone TEXT,
			last_client_message_at TIMESTAMPTZ NOT NULL,
			first_reminder_at TIMESTAMPTZ,
			second_reminder_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("ensure schema: %w", err)
	}

	return nil
}

var _ interface {
	Get(context.Context, int64) (reminders.ChatState, bool, error)
	Save(context.Context, reminders.ChatState) error
} = (*Postgres)(nil)

var _ = time.UTC
