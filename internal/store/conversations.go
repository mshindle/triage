package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// ConversationFilters holds optional filter/sort parameters for GetConversations.
type ConversationFilters struct {
	Priority []string // "high", "medium", "low" — multiple values are OR'd
	Category string   // exact match on last message category; empty = no filter
	Status   string   // "completed", "pending", "failed"; empty = no filter
	Sort     string   // "recent" (default) or "priority"
}

// ActiveCount returns the number of non-default active filters.
func (f ConversationFilters) ActiveCount() int {
	n := len(f.Priority)
	if f.Category != "" {
		n++
	}
	if f.Status != "" {
		n++
	}
	return n
}

// ConversationSummary is derived via GROUP BY from the messages table.
// Identity is "group:{group_id}" for group chats or "{sender_phone}" for direct.
type ConversationSummary struct {
	Identity         string
	SenderPhone      string
	GroupID          *string
	LastMessageAt    time.Time
	HighestPriority  int
	MessageCount     int
	LastPreview      string
	LastCategory     string
	LastTriageStatus string
}

// IsGroup reports whether the conversation is a Signal group chat.
func (c ConversationSummary) IsGroup() bool {
	return strings.HasPrefix(c.Identity, "group:")
}

// DisplayName returns the group_id (sans prefix) for group chats, or the sender phone for direct.
func (c ConversationSummary) DisplayName() string {
	if c.IsGroup() {
		return strings.TrimPrefix(c.Identity, "group:")
	}
	return c.SenderPhone
}

// GetConversations returns all conversations grouped from the messages table.
// Filters and sort order are applied based on the provided ConversationFilters.
func GetConversations(ctx context.Context, pool *pgxpool.Pool, filters ConversationFilters) ([]ConversationSummary, error) {
	q := `
		WITH conv_key AS (
			SELECT *,
				CASE WHEN group_id IS NOT NULL AND group_id != ''
				     THEN 'group:' || group_id
				     ELSE sender_phone
				END AS conv_id
			FROM messages
		),
		ranked AS (
			SELECT *,
				ROW_NUMBER() OVER (PARTITION BY conv_id ORDER BY created_at DESC) AS rn
			FROM conv_key
		)
		SELECT
			conv_id,
			MAX(sender_phone)                              AS sender_phone,
			MAX(group_id)                                  AS group_id,
			MAX(created_at)                                AS last_message_at,
			MAX(priority)                                  AS highest_priority,
			COUNT(*)                                       AS message_count,
			MAX(CASE WHEN rn = 1 THEN LEFT(content, 80) END) AS last_preview,
			MAX(CASE WHEN rn = 1 THEN category END)        AS last_category,
			MAX(CASE WHEN rn = 1 THEN triage_status END)   AS last_triage_status
		FROM ranked
		GROUP BY conv_id`

	var having []string
	var args []interface{}
	argN := 1

	// Priority filter — literals only, no SQL injection risk.
	if len(filters.Priority) > 0 {
		var pConds []string
		for _, p := range filters.Priority {
			switch p {
			case "high":
				pConds = append(pConds, "MAX(priority) >= 70")
			case "medium":
				pConds = append(pConds, "(MAX(priority) >= 40 AND MAX(priority) < 70)")
			case "low":
				pConds = append(pConds, "MAX(priority) < 40")
			}
		}
		if len(pConds) > 0 {
			having = append(having, "("+strings.Join(pConds, " OR ")+")")
		}
	}

	// Category filter.
	if filters.Category != "" {
		having = append(having, fmt.Sprintf("MAX(CASE WHEN rn = 1 THEN category END) = $%d", argN))
		args = append(args, filters.Category)
		argN++
	}

	// Status filter.
	if filters.Status != "" {
		having = append(having, fmt.Sprintf("MAX(CASE WHEN rn = 1 THEN triage_status END) = $%d", argN))
		args = append(args, filters.Status)
		argN++ //nolint:ineffassign // argN kept for future additions
	}

	if len(having) > 0 {
		q += " HAVING " + strings.Join(having, " AND ")
	}

	if filters.Sort == "priority" {
		q += " ORDER BY MAX(priority) DESC, MAX(created_at) DESC"
	} else {
		q += " ORDER BY MAX(created_at) DESC"
	}

	rows, err := pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query conversations: %w", err)
	}
	defer rows.Close()

	var convs []ConversationSummary
	for rows.Next() {
		var c ConversationSummary
		var lastCategory, lastStatus *string
		if err := rows.Scan(
			&c.Identity, &c.SenderPhone, &c.GroupID,
			&c.LastMessageAt, &c.HighestPriority, &c.MessageCount,
			&c.LastPreview, &lastCategory, &lastStatus,
		); err != nil {
			return nil, fmt.Errorf("failed to scan conversation: %w", err)
		}
		if lastCategory != nil {
			c.LastCategory = *lastCategory
		}
		if lastStatus != nil {
			c.LastTriageStatus = *lastStatus
		}
		convs = append(convs, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row error in conversations: %w", err)
	}
	return convs, nil
}

// GetMessagesByConversation returns all messages for a conversation ordered oldest-first.
// identity is "group:{group_id}" or "{sender_phone}".
func GetMessagesByConversation(ctx context.Context, pool *pgxpool.Pool, identity string) ([]Message, error) {
	start := time.Now()
	const cols = `id, signal_id, sender_phone, content, group_id, priority, category, reasoning, triage_status, created_at`

	var queryStr string
	var args []interface{}
	if strings.HasPrefix(identity, "group:") {
		groupID := strings.TrimPrefix(identity, "group:")
		queryStr = `SELECT ` + cols + ` FROM messages WHERE group_id = $1 ORDER BY created_at ASC`
		args = []interface{}{groupID}
	} else {
		queryStr = `SELECT ` + cols + ` FROM messages WHERE sender_phone = $1 AND (group_id IS NULL OR group_id = '') ORDER BY created_at ASC`
		args = []interface{}{identity}
	}

	rows, err := pool.Query(ctx, queryStr, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages by conversation: %w", err)
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(
			&m.ID, &m.SignalID, &m.SenderPhone, &m.Content, &m.GroupID,
			&m.Priority, &m.Category, &m.Reasoning, &m.TriageStatus, &m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row error in messages: %w", err)
	}

	log.Info().
		Str("stage", "store").
		Str("identity", identity).
		Int("count", len(msgs)).
		Int64("duration_ms", time.Since(start).Milliseconds()).
		Msg("messages by conversation loaded")

	return msgs, nil
}

// GetFeedbackByMessage returns all feedback corrections for a message, newest first.
func GetFeedbackByMessage(ctx context.Context, pool *pgxpool.Pool, messageID int64) ([]FeedbackMemory, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, original_message_id, feedback_text, adjusted_priority, adjusted_category, created_at
		 FROM feedback_memory
		 WHERE original_message_id = $1
		 ORDER BY created_at DESC`,
		messageID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query feedback by message: %w", err)
	}
	defer rows.Close()

	var feedbacks []FeedbackMemory
	for rows.Next() {
		var f FeedbackMemory
		if err := rows.Scan(&f.ID, &f.OriginalMessageID, &f.FeedbackText, &f.AdjustedPriority, &f.AdjustedCategory, &f.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan feedback: %w", err)
		}
		feedbacks = append(feedbacks, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row error in feedback: %w", err)
	}
	return feedbacks, nil
}

// GetCategories returns all distinct non-empty message categories, sorted alphabetically.
func GetCategories(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	rows, err := pool.Query(ctx,
		`SELECT DISTINCT category FROM messages WHERE category IS NOT NULL AND category != '' ORDER BY category`,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query categories: %w", err)
	}
	defer rows.Close()

	var cats []string
	for rows.Next() {
		var cat string
		if err := rows.Scan(&cat); err != nil {
			return nil, fmt.Errorf("failed to scan category: %w", err)
		}
		cats = append(cats, cat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row error in categories: %w", err)
	}
	return cats, nil
}
