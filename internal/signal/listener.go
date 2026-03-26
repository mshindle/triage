package signal

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/rs/zerolog/log"
	"github.com/tidwall/gjson"
)

type Envelope struct {
	Source    string
	Content   string
	GroupID   string
	Timestamp int64
}

type Listener struct {
	wsURL     string
	onMessage func(Envelope)
}

func NewListener(wsURL string, onMessage func(Envelope)) *Listener {
	return &Listener{
		wsURL:     wsURL,
		onMessage: onMessage,
	}
}

func (l *Listener) Listen(ctx context.Context) error {
	backoff := 1 * time.Second
	maxBackoff := 30 * time.Second

	for {
		err := l.connectAndRead(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}

		log.Error().
			Str("stage", "signal_listener").
			Err(err).
			Str("backoff", backoff.String()).
			Msg("connection lost, reconnecting")

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

func (l *Listener) connectAndRead(ctx context.Context) error {
	log.Info().Str("stage", "signal_listener").Str("url", l.wsURL).Msg("connecting to signal bridge")

	c, _, err := websocket.Dial(ctx, l.wsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to dial: %w", err)
	}
	defer c.Close(websocket.StatusNormalClosure, "")

	log.Info().Str("stage", "signal_listener").Msg("connected to signal bridge")

	for {
		var msg json.RawMessage
		err := wsjson.Read(ctx, c, &msg)
		if err != nil {
			return fmt.Errorf("failed to read message: %w", err)
		}

		res := gjson.ParseBytes(msg)

		// Signal bridge (signal-cli-rest-api) sends messages in a specific JSON-RPC style or plain JSON depending on version.
		// We expect the 'envelope' structure.
		envelope := res.Get("params.envelope")
		if !envelope.Exists() {
			// Try root if not in params (some versions)
			envelope = res.Get("envelope")
		}

		if envelope.Exists() {
			source := envelope.Get("source").String()
			timestamp := envelope.Get("timestamp").Int()

			dataMsg := envelope.Get("dataMessage")
			content := dataMsg.Get("message").String()
			groupID := dataMsg.Get("groupInfo.groupId").String()

			if content != "" {
				l.onMessage(Envelope{
					Source:    source,
					Content:   content,
					GroupID:   groupID,
					Timestamp: timestamp,
				})
			}
		}
	}
}
