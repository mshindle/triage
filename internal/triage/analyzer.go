package triage

import (
	"context"

	"github.com/mshindle/triage/internal/store"
)

type Result struct {
	Priority  int    `json:"priority"`
	Category  string `json:"category"`
	Reasoning string `json:"reasoning"`
}

type Analyzer interface {
	GenerateEmbedding(ctx context.Context, signalID string, content string) ([]float32, error)
	BuildFeedbackContext(signalID string, memories []store.FeedbackMemory) string
	TriageMessage(ctx context.Context, signalID, content string, feedbackContext string) (Result, error)
}
