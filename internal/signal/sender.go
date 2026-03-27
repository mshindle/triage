package signal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/mshindle/triage/internal/store"
	"github.com/rs/zerolog/log"
)

type Sender struct {
	endpoint    string
	phoneNumber string
}

func NewSender(sendURL string, phone string) *Sender {
	return &Sender{
		endpoint:    sendURL,
		phoneNumber: phone,
	}
}

type sendRequest struct {
	Message    string   `json:"message"`
	Number     string   `json:"number"`
	Recipients []string `json:"recipients,omitempty"`
	GroupID    string   `json:"groupId,omitempty"`
}

func (s *Sender) SendReply(ctx context.Context, msg store.Message, content string) error {
	start := time.Now()

	reqBody := sendRequest{
		Message: content,
		Number:  s.phoneNumber,
	}

	if msg.GroupID != nil && *msg.GroupID != "" {
		reqBody.GroupID = *msg.GroupID
	} else {
		reqBody.Recipients = []string{msg.SenderPhone}
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal send request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("signal bridge returned non-2xx status: %d", resp.StatusCode)
	}

	log.Info().
		Str("stage", "signal_sender").
		Str("signal_id", msg.SignalID).
		Int64("duration_ms", time.Since(start).Milliseconds()).
		Msg("reply sent")

	return nil
}
