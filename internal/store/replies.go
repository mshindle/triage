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

// GetRepliesByMessageIDs returns all replies for the given message IDs, ordered by creation time.
func GetRepliesByMessageIDs(ctx context.Context, pool *pgxpool.Pool, messageIDs []int64) ([]Reply, error) {
	if len(messageIDs) == 0 {
		return nil, nil
	}
	rows, err := pool.Query(ctx,
		`SELECT id, original_message_id, content, delivery_status, error_detail, created_at
		 FROM replies
		 WHERE original_message_id = ANY($1)
		 ORDER BY created_at ASC`,
		messageIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query replies by message ids: %w", err)
	}
	defer rows.Close()

	var replies []Reply
	for rows.Next() {
		var r Reply
		if err := rows.Scan(&r.ID, &r.OriginalMessageID, &r.Content, &r.DeliveryStatus, &r.ErrorDetail, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan reply: %w", err)
		}
		replies = append(replies, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row error in replies: %w", err)
	}
	return replies, nil
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
