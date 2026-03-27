package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

type Reply struct {
	ID                int64
	OriginalMessageID int64
	Content           string
	DeliveryStatus    string
	ErrorDetail       *string
	CreatedAt         time.Time
}

func InsertReply(ctx context.Context, pool *pgxpool.Pool, messageID int64, signalID string, content string) (int64, error) {
	start := time.Now()
	var id int64
	err := pool.QueryRow(ctx,
		"INSERT INTO replies (original_message_id, content) VALUES ($1, $2) RETURNING id",
		messageID, content,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to insert reply: %w", err)
	}

	log.Info().
		Str("stage", "store").
		Str("signal_id", signalID).
		Int64("duration_ms", time.Since(start).Milliseconds()).
		Msg("reply inserted")

	return id, nil
}

func UpdateDeliveryStatus(ctx context.Context, pool *pgxpool.Pool, replyID int64, signalID string, status, errDetail string) error {
	start := time.Now()
	_, err := pool.Exec(ctx,
		"UPDATE replies SET delivery_status = $1, error_detail = $2 WHERE id = $3",
		status, errDetail, replyID,
	)
	if err != nil {
		return fmt.Errorf("failed to update delivery status: %w", err)
	}

	log.Info().
		Str("stage", "store").
		Str("signal_id", signalID).
		Int64("duration_ms", time.Since(start).Milliseconds()).
		Str("status", status).
		Msg("delivery status updated")

	return nil
}
