package signal

import (
	"bytes"
	"context"
	"fmt"
	"time"

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
	analyzer triage.Analyzer
	wsURL    string
}

func NewPipeline(receiveURL string, pool *pgxpool.Pool, hub Broadcaster, analyzer triage.Analyzer) (*Pipeline, error) {
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
	start := time.Now()
	ctx := context.Background()

	signalID := fmt.Sprintf("%s-%d", env.Source, env.Timestamp)
	msg := store.Message{
		SignalID:     signalID,
		SenderPhone:  env.Source,
		Content:      env.Content,
		GroupID:      &env.GroupID,
		TriageStatus: "pending",
	}

	id, err := store.InsertMessage(ctx, p.pool, msg)
	if err != nil {
		log.Error().Str("signal_id", signalID).Err(err).Msg("failed to insert message")
		return
	}
	if id == 0 {
		return // duplicate
	}
	msg.ID = id

	embedding, err := p.analyzer.GenerateEmbedding(ctx, signalID, env.Content)
	var feedbackContext string
	if err != nil {
		log.Error().Str("signal_id", signalID).Err(err).Msg("failed to generate embedding")
	} else {
		if err := store.UpdateMessageEmbedding(ctx, p.pool, id, signalID, embedding); err != nil {
			log.Error().Str("signal_id", signalID).Err(err).Msg("failed to update embedding")
		}
		memories, err := store.RecallSimilar(ctx, p.pool, signalID, embedding, 5)
		if err != nil {
			log.Error().Str("signal_id", signalID).Err(err).Msg("failed to recall similar feedback")
		} else {
			feedbackContext = p.analyzer.BuildFeedbackContext(signalID, memories)
		}
	}

	result, err := p.analyzer.TriageMessage(ctx, signalID, env.Content, feedbackContext)
	if err != nil {
		log.Error().Str("signal_id", signalID).Err(err).Msg("failed to triage message")
		_ = store.UpdateMessageTriage(ctx, p.pool, id, signalID, 0, "Unknown", "Triage failed", "failed")
		msg.Priority = 0
		msg.Category = "Unknown"
		msg.Reasoning = "Triage failed"
		msg.TriageStatus = "failed"
	} else {
		_ = store.UpdateMessageTriage(ctx, p.pool, id, signalID, result.Priority, result.Category, result.Reasoning, "completed")
		msg.Priority = result.Priority
		msg.Category = result.Category
		msg.Reasoning = result.Reasoning
		msg.TriageStatus = "completed"
	}

	// Determine the conversation identity for thread targeting.
	var convIdentity string
	if env.GroupID != "" {
		convIdentity = "group:" + env.GroupID
	} else {
		convIdentity = env.Source
	}

	// Broadcast the new message bubble to clients with this conversation open.
	// hx-swap-oob silently ignores the target if it doesn't exist in the client DOM.
	var bubBuf bytes.Buffer
	if err := templates.MessageBubble(msg).Render(ctx, &bubBuf); err != nil {
		log.Error().Str("signal_id", signalID).Err(err).Msg("failed to render message bubble for broadcast")
	} else {
		streamID := templates.ConvStreamID(convIdentity)
		oobBubble := fmt.Sprintf(`<div hx-swap-oob="beforeend:#%s">%s</div>`, streamID, bubBuf.String())
		p.hub.Broadcast([]byte(oobBubble))
		log.Info().
			Str("stage", "thread_broadcast").
			Str("signal_id", signalID).
			Str("stream_id", streamID).
			Int64("duration_ms", time.Since(start).Milliseconds()).
			Msg("message bubble broadcasted to thread")
	}

	// Fetch the updated conversation list and broadcast an OOB swap for the left panel.
	// Broadcasts always use the full unfiltered list.
	convs, err := store.GetConversations(ctx, p.pool, store.ConversationFilters{})
	if err != nil {
		log.Error().Str("signal_id", signalID).Err(err).Msg("failed to get conversations for broadcast")
		// Continue without broadcasting — not fatal.
	} else {
		var buf bytes.Buffer
		if err := templates.ConversationListBroadcast(convs).Render(ctx, &buf); err != nil {
			log.Error().Str("signal_id", signalID).Err(err).Msg("failed to render conversation list broadcast")
		} else {
			p.hub.Broadcast(buf.Bytes())
			log.Info().
				Str("stage", "broadcast").
				Str("signal_id", signalID).
				Int("conversations", len(convs)).
				Int64("duration_ms", time.Since(start).Milliseconds()).
				Msg("conversation list broadcasted")
		}
	}

	log.Info().
		Str("stage", "pipeline").
		Str("signal_id", signalID).
		Int64("duration_ms", time.Since(start).Milliseconds()).
		Msg("pipeline complete")
}
