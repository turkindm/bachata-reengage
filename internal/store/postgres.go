package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/turkindm/bachata-reengage/internal/reminders"
)

// Store persists reminder chat states in PostgreSQL.
type Store struct {
	pool *pgxpool.Pool
}

// Open connects to PostgreSQL, pings it, and ensures the schema exists.
func Open(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	s := &Store{pool: pool}
	if err := s.ensureSchema(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return s, nil
}

// Close releases the connection pool.
func (s *Store) Close() error {
	s.pool.Close()
	return nil
}

// Get retrieves chat state by chat ID. Returns (zero, false, nil) when not found.
func (s *Store) Get(ctx context.Context, chatID int64) (reminders.ChatState, bool, error) {
	const q = `
		SELECT chat_id, client_id, status, phone,
		       last_client_message_at, first_reminder_at, second_reminder_at, updated_at
		FROM reminder_chats
		WHERE chat_id = $1`

	var state reminders.ChatState
	err := s.pool.QueryRow(ctx, q, chatID).Scan(
		&state.ChatID,
		&state.ClientID,
		&state.Status,
		&state.Phone,
		&state.LastClientMessageAt,
		&state.FirstReminderAt,
		&state.SecondReminderAt,
		&state.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return reminders.ChatState{}, false, nil
	}
	if err != nil {
		return reminders.ChatState{}, false, fmt.Errorf("get chat state %d: %w", chatID, err)
	}

	return state, true, nil
}

// Save inserts or updates a chat state (upsert by chat_id).
func (s *Store) Save(ctx context.Context, state reminders.ChatState) error {
	const q = `
		INSERT INTO reminder_chats
		    (chat_id, client_id, status, phone,
		     last_client_message_at, first_reminder_at, second_reminder_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (chat_id) DO UPDATE SET
		    status                 = EXCLUDED.status,
		    phone                  = EXCLUDED.phone,
		    first_reminder_at      = EXCLUDED.first_reminder_at,
		    second_reminder_at     = EXCLUDED.second_reminder_at,
		    updated_at             = EXCLUDED.updated_at`

	_, err := s.pool.Exec(ctx, q,
		state.ChatID,
		state.ClientID,
		state.Status,
		state.Phone,
		state.LastClientMessageAt,
		state.FirstReminderAt,
		state.SecondReminderAt,
		state.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save chat state %d: %w", state.ChatID, err)
	}

	return nil
}

func (s *Store) ensureSchema(ctx context.Context) error {
	const ddl = `
		CREATE TABLE IF NOT EXISTS reminder_chats (
			chat_id                BIGINT      PRIMARY KEY,
			client_id              TEXT        NOT NULL,
			status                 TEXT        NOT NULL,
			phone                  TEXT        NOT NULL DEFAULT '',
			last_client_message_at TIMESTAMPTZ NOT NULL,
			first_reminder_at      TIMESTAMPTZ,
			second_reminder_at     TIMESTAMPTZ,
			updated_at             TIMESTAMPTZ NOT NULL,
			created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`

	if _, err := s.pool.Exec(ctx, ddl); err != nil {
		return fmt.Errorf("ensure schema: %w", err)
	}

	return nil
}
