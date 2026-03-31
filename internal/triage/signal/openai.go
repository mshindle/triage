package signal

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mshindle/triage/internal/store"
	"github.com/mshindle/triage/internal/triage"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"
	"github.com/rs/zerolog/log"
)

type OpenAIAnalyzer struct {
	client     *openai.Client
	model      string
	embedModel string
	embedDims  int
}

func NewOpenAIAnalyzer(llmKey, llmModel, embedModel string, embedDims int) *OpenAIAnalyzer {
	return &OpenAIAnalyzer{
		client:     new(openai.NewClient(option.WithAPIKey(llmKey))),
		model:      llmModel,
		embedModel: embedModel,
		embedDims:  embedDims,
	}
}

func (a *OpenAIAnalyzer) TriageMessage(ctx context.Context, signalID, content string, feedbackContext string) (triage.Result, error) {
	start := time.Now()
	systemPrompt := "You are a Signal message triage assistant. Output ONLY valid JSON with keys: priority (0-100), category (string), reasoning (string)."
	if feedbackContext != "" {
		systemPrompt = fmt.Sprintf("%s\n\nUse the following past user corrections for context:\n%s", systemPrompt, feedbackContext)
	}

	chatCompletion, err := a.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(content),
		},
		Model: a.model,
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONObject: openai.Ptr(shared.NewResponseFormatJSONObjectParam()),
		},
	})
	if err != nil {
		return triage.Result{}, fmt.Errorf("openai chat completion failed: %w", err)
	}

	var result triage.Result
	if err := json.Unmarshal([]byte(chatCompletion.Choices[0].Message.Content), &result); err != nil {
		return triage.Result{}, fmt.Errorf("failed to unmarshal triage result: %w", err)
	}

	if result.Priority < 0 || result.Priority > 100 {
		return triage.Result{}, fmt.Errorf("priority outside 0-100: %d", result.Priority)
	}

	log.Info().
		Str("stage", "triage").
		Str("signal_id", signalID).
		Int64("duration_ms", time.Since(start).Milliseconds()).
		Int("priority", result.Priority).
		Str("category", result.Category).
		Msg("message triaged")

	return result, nil
}

func (a *OpenAIAnalyzer) BuildFeedbackContext(signalID string, memories []store.FeedbackMemory) string {
	start := time.Now()
	if len(memories) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, m := range memories {
		sb.WriteString(fmt.Sprintf("Past correction: %s -> priority %d\n", m.FeedbackText, m.AdjustedPriority))
	}
	log.Info().
		Str("stage", "triage").
		Str("signal_id", signalID).
		Int64("duration_ms", time.Since(start).Milliseconds()).
		Int("feedback_count", len(memories)).
		Msg("feedback context built")
	return sb.String()
}

func (a *OpenAIAnalyzer) GenerateEmbedding(ctx context.Context, signalID, text string) ([]float32, error) {
	start := time.Now()
	embedding, err := a.client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Input:      openai.EmbeddingNewParamsInputUnion{OfString: openai.String(text)},
		Model:      a.embedModel,
		Dimensions: openai.Int(int64(a.embedDims)),
	})
	if err != nil {
		return nil, fmt.Errorf("openai embedding failed: %w", err)
	}

	// OpenAI Go SDK returns []float64 for embeddings, but we store []float32 for pgvector compatibility
	res := make([]float32, len(embedding.Data[0].Embedding))
	for i, v := range embedding.Data[0].Embedding {
		res[i] = float32(v)
	}

	log.Info().
		Str("stage", "triage").
		Str("signal_id", signalID).
		Int64("duration_ms", time.Since(start).Milliseconds()).
		Msg("embedding generated")

	return res, nil
}
