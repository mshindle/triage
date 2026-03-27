package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
	"github.com/rs/zerolog/log"
)

type FeedbackMemory struct {
	ID                int64
	OriginalMessageID int64
	FeedbackText      string
	AdjustedPriority  int
	Embedding         []float32
	CreatedAt         time.Time
}

func InsertCorrection(ctx context.Context, pool *pgxpool.Pool, messageID int64, signalID string, feedbackText string, adjustedPriority int, embedding []float32) error {
	start := time.Now()
	_, err := pool.Exec(ctx,
		`INSERT INTO feedback_memory (original_message_id, feedback_text, adjusted_priority, embedding)
		 VALUES ($1, $2, $3, $4)`,
		messageID, feedbackText, adjustedPriority, pgvector.NewVector(embedding),
	)
	if err != nil {
		return fmt.Errorf("failed to insert correction: %w", err)
	}

	log.Info().
		Str("stage", "store").
		Str("signal_id", signalID).
		Int64("message_id", messageID).
		Int64("duration_ms", time.Since(start).Milliseconds()).
		Msg("correction inserted")

	return nil
}

func RecallSimilar(ctx context.Context, pool *pgxpool.Pool, signalID string, queryEmbedding []float32, k int) ([]FeedbackMemory, error) {
	start := time.Now()
	rows, err := pool.Query(ctx,
		`SELECT id, original_message_id, feedback_text, adjusted_priority, embedding, created_at
		 FROM feedback_memory
		 ORDER BY embedding <=> $1
		 LIMIT $2`,
		pgvector.NewVector(queryEmbedding), k,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to recall similar feedback: %w", err)
	}
	defer rows.Close()

	var memories []FeedbackMemory
	for rows.Next() {
		var mem FeedbackMemory
		var vec pgvector.Vector
		err := rows.Scan(&mem.ID, &mem.OriginalMessageID, &mem.FeedbackText, &mem.AdjustedPriority, &vec, &mem.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan feedback memory: %w", err)
		}
		mem.Embedding = vec.Slice()
		memories = append(memories, mem)
	}

	log.Info().
		Str("stage", "store").
		Str("signal_id", signalID).
		Int64("duration_ms", time.Since(start).Milliseconds()).
		Int("count", len(memories)).
		Msg("similar feedback recalled")

	return memories, nil
}
