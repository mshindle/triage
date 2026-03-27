package signal

import (
	"bytes"
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mshindle/triage/internal/store"
	"github.com/mshindle/triage/internal/triage"
	"github.com/mshindle/triage/internal/web/templates"
	"github.com/rs/zerolog/log"
)

type Broadcaster interface {
	Broadcast(msg []byte)
}

type Pipeline struct {
	pool     *pgxpool.Pool
	hub      Broadcaster
	analyzer *triage.Analyzer
	wsURL    string
}

func NewPipeline(receiveURL string, pool *pgxpool.Pool, hub Broadcaster, analyzer *triage.Analyzer) (*Pipeline, error) {
	return &Pipeline{
		wsURL:    receiveURL,
		pool:     pool,
		hub:      hub,
		analyzer: analyzer,
	}, nil
}

func (p *Pipeline) Listen(ctx context.Context) error {
	listener := NewListener(p.wsURL, p.handle)
	return listener.Listen(ctx)
}

func (p *Pipeline) handle(env Envelope) {
	ctx := context.Background()

	msg := store.Message{
		SignalID:     fmt.Sprintf("%s-%d", env.Source, env.Timestamp),
		SenderPhone:  env.Source,
		Content:      env.Content,
		GroupID:      &env.GroupID,
		TriageStatus: "pending",
	}

	id, err := store.InsertMessage(ctx, p.pool, msg)
	if err != nil {
		log.Error().Err(err).Msg("failed to insert message")
		return
	}
	if id == 0 {
		return // duplicate
	}
	msg.ID = id

	embedding, err := p.analyzer.GenerateEmbedding(ctx, env.Content)
	var feedbackContext string
	if err != nil {
		log.Error().Err(err).Msg("failed to generate embedding")
	} else {
		if err := store.UpdateMessageEmbedding(ctx, p.pool, id, embedding); err != nil {
			log.Error().Err(err).Msg("failed to update embedding")
		}
		memories, err := store.RecallSimilar(ctx, p.pool, embedding, 5)
		if err != nil {
			log.Error().Err(err).Msg("failed to recall similar feedback")
		} else {
			feedbackContext = p.analyzer.BuildFeedbackContext(memories)
		}
	}

	result, err := p.analyzer.TriageMessage(ctx, env.Content, feedbackContext)
	if err != nil {
		log.Error().Err(err).Msg("failed to triage message")
		_ = store.UpdateMessageTriage(ctx, p.pool, id, 0, "Unknown", "Triage failed", "failed")
	} else {
		_ = store.UpdateMessageTriage(ctx, p.pool, id, result.Priority, result.Category, result.Reasoning, "completed")
	}

	messages, err := store.GetMessages(ctx, p.pool)
	if err != nil {
		return
	}
	var updatedMsg store.Message
	for _, m := range messages {
		if m.ID == id {
			updatedMsg = m
			break
		}
	}

	var buf bytes.Buffer
	if err := templates.MessageCard(updatedMsg).Render(ctx, &buf); err != nil {
		log.Error().Err(err).Msg("failed to render message card")
		return
	}

	p.hub.Broadcast([]byte(fmt.Sprintf(`<div hx-swap-oob="afterbegin:#message-stream">%s</div>`, buf.String())))
}
