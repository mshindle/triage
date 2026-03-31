package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
	"github.com/rs/zerolog/log"
)

type Message struct {
	ID           int64
	SignalID     string
	SenderPhone  string
	Content      string
	GroupID      *string
	Priority     int
	Category     string
	Reasoning    string
	TriageStatus string
	Embedding    []float32
	CreatedAt    time.Time
}

func InsertMessage(ctx context.Context, pool *pgxpool.Pool, msg Message) (int64, error) {
	start := time.Now()
	var id int64
	err := pool.QueryRow(ctx,
		`INSERT INTO messages (signal_id, sender_phone, content, group_id, priority, category, reasoning, triage_status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT (signal_id) DO NOTHING
		 RETURNING id`,
		msg.SignalID, msg.SenderPhone, msg.Content, msg.GroupID, msg.Priority, msg.Category, msg.Reasoning, msg.TriageStatus,
	).Scan(&id)

	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, nil // Already exists
		}
		return 0, fmt.Errorf("failed to insert message: %w", err)
	}

	log.Info().
		Str("stage", "store").
		Str("signal_id", msg.SignalID).
		Int64("duration_ms", time.Since(start).Milliseconds()).
		Msg("message inserted")

	return id, nil
}

func GetMessageByID(ctx context.Context, pool *pgxpool.Pool, id int64) (Message, error) {
	var msg Message
	err := pool.QueryRow(ctx,
		`SELECT id, signal_id, sender_phone, content, group_id, priority, category, reasoning, triage_status, created_at
		 FROM messages WHERE id = $1`,
		id,
	).Scan(&msg.ID, &msg.SignalID, &msg.SenderPhone, &msg.Content, &msg.GroupID,
		&msg.Priority, &msg.Category, &msg.Reasoning, &msg.TriageStatus, &msg.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return Message{}, fmt.Errorf("message not found: %d", id)
		}
		return Message{}, fmt.Errorf("failed to get message by id: %w", err)
	}
	return msg, nil
}

func GetMessages(ctx context.Context, pool *pgxpool.Pool) ([]Message, error) {
	start := time.Now()
	rows, err := pool.Query(ctx,
		`SELECT id, signal_id, sender_phone, content, group_id, priority, category, reasoning, triage_status, created_at
		 FROM messages
		 ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		err := rows.Scan(
			&msg.ID, &msg.SignalID, &msg.SenderPhone, &msg.Content, &msg.GroupID,
			&msg.Priority, &msg.Category, &msg.Reasoning, &msg.TriageStatus, &msg.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		messages = append(messages, msg)
	}

	log.Info().
		Str("stage", "store").
		Int64("duration_ms", time.Since(start).Milliseconds()).
		Int("count", len(messages)).
		Msg("messages retrieved")

	return messages, nil
}

func UpdateMessageTriage(ctx context.Context, pool *pgxpool.Pool, id int64, signalID string, priority int, category, reasoning, status string) error {
	start := time.Now()
	_, err := pool.Exec(ctx,
		`UPDATE messages
		 SET priority = $1, category = $2, reasoning = $3, triage_status = $4
		 WHERE id = $5`,
		priority, category, reasoning, status, id,
	)
	if err != nil {
		return fmt.Errorf("failed to update message triage: %w", err)
	}

	log.Info().
		Str("stage", "store").
		Str("signal_id", signalID).
		Int64("id", id).
		Int64("duration_ms", time.Since(start).Milliseconds()).
		Msg("message triage updated")

	return nil
}

func UpdateMessagePriority(ctx context.Context, pool *pgxpool.Pool, id int64, signalID string, priority int) error {
	start := time.Now()
	_, err := pool.Exec(ctx,
		"UPDATE messages SET priority = $1 WHERE id = $2",
		priority, id,
	)
	if err != nil {
		return fmt.Errorf("failed to update message priority: %w", err)
	}

	log.Info().
		Str("stage", "store").
		Str("signal_id", signalID).
		Int64("id", id).
		Int64("duration_ms", time.Since(start).Milliseconds()).
		Msg("message priority updated")

	return nil
}

func UpdateMessageEmbedding(ctx context.Context, pool *pgxpool.Pool, id int64, signalID string, embedding []float32) error {
	start := time.Now()
	_, err := pool.Exec(ctx,
		"UPDATE messages SET embedding = $1 WHERE id = $2",
		pgvector.NewVector(embedding), id,
	)
	if err != nil {
		return fmt.Errorf("failed to update message embedding: %w", err)
	}

	log.Info().
		Str("stage", "store").
		Str("signal_id", signalID).
		Int64("id", id).
		Int64("duration_ms", time.Since(start).Milliseconds()).
		Msg("message embedding updated")

	return nil
}
