# Data Model: Signal AI Triage Dashboard UI

**Branch**: `002-signal-triage-ui` | **Date**: 2026-03-30

---

## Existing Tables (unchanged except where noted)

### `messages`

| Column | Type | Notes |
|--------|------|-------|
| id | BIGSERIAL PK | |
| signal_id | TEXT UNIQUE NOT NULL | Idempotency key |
| sender_phone | TEXT NOT NULL | E.164 format |
| content | TEXT NOT NULL | |
| group_id | TEXT | NULL for direct messages |
| priority | INT DEFAULT 50 | 0–100; ≥70 high, 40–69 medium, <40 low |
| category | TEXT | AI-assigned label |
| reasoning | TEXT | AI explanation |
| triage_status | TEXT | 'pending' \| 'completed' \| 'failed' |
| embedding | vector(768) | Semantic search vector |
| created_at | TIMESTAMPTZ | |

### `feedback_memory`

| Column | Type | Notes |
|--------|------|-------|
| id | BIGSERIAL PK | |
| original_message_id | BIGINT FK → messages.id | |
| feedback_text | TEXT | Free-form operator explanation |
| adjusted_priority | INT | Operator-corrected priority |
| **adjusted_category** | **TEXT** | **New (migration 000002) — nullable** |
| embedding | vector(768) | |
| created_at | TIMESTAMPTZ | |

### `replies`

| Column | Type | Notes |
|--------|------|-------|
| id | BIGSERIAL PK | |
| original_message_id | BIGINT FK → messages.id | |
| content | TEXT | |
| delivery_status | TEXT | 'pending' \| 'delivered' \| 'failed' |
| error_detail | TEXT | NULL unless failed |
| created_at | TIMESTAMPTZ | |

---

## Migration

**File**: `internal/store/migrations/000002_add_feedback_category.up.sql`

```sql
ALTER TABLE feedback_memory ADD COLUMN adjusted_category TEXT;
```

**Rollback**: `internal/store/migrations/000002_add_feedback_category.down.sql`

```sql
ALTER TABLE feedback_memory DROP COLUMN adjusted_category;
```

---

## New Derived Type: ConversationSummary

Not stored in the database — returned by `GetConversations()` as a Go struct.

```go
// ConversationSummary represents the left-panel conversation list entry.
// Derived via SQL GROUP BY from the messages table.
type ConversationSummary struct {
    SenderPhone      string
    GroupID          *string   // nil for direct messages
    LastMessageAt    time.Time
    HighestPriority  int       // max(priority) across conversation
    MessageCount     int
    UnreadCount      int       // messages since last viewed (approximated by created_at)
    LastCategory     string    // category of most recent message
    LastPreview      string    // first 80 chars of most recent message content
    TriageStatus     string    // status of most recent message
}
```

**Identity rule**: A conversation is uniquely identified by `(sender_phone, group_id)`. Direct messages have `group_id = nil`; group conversations have a non-nil `group_id`. The UI displays the group_id (or a derived label) when `group_id` is non-nil, otherwise the sender_phone.

---

## Updated Go Struct: FeedbackMemory

```go
type FeedbackMemory struct {
    ID                int64
    OriginalMessageID int64
    FeedbackText      string
    AdjustedPriority  int
    AdjustedCategory  *string   // New: nil if no category correction given
    Embedding         []float32
    CreatedAt         time.Time
}
```

---

## New Store Queries (`internal/store/conversations.go`)

### `GetConversations(ctx, filters ConversationFilters) ([]ConversationSummary, error)`

Groups messages by `(sender_phone, group_id)`. Supports optional filtering by priority tier, category, and triage status. Sort options: `recent` (default), `priority`, `unread`.

### `GetMessagesByConversation(ctx, senderPhone string, groupID *string) ([]Message, error)`

Returns all messages for a conversation ordered by `created_at ASC`. Used to render the center thread panel.

### `GetFeedbackByMessage(ctx, messageID int64) ([]FeedbackMemory, error)`

Returns all feedback entries for a message ordered by `created_at DESC`. Used to render the feedback history section in the right panel.

### `GetCategories(ctx) ([]string, error)`

Returns distinct, non-null category values from the messages table. Used to populate the dynamic category filter chips in the header.

---

## WebSocket Broadcast Payloads (server → client, HTMX OOB HTML)

These are not JSON — they are rendered HTML fragments with `hx-swap-oob` directives, consistent with the existing broadcast pattern.

| Event | hx-swap-oob target | Content |
|-------|--------------------|---------|
| New message + triage complete | `afterbegin:#conversation-list` | Rendered `conversation_item.templ` |
| Triage update (re-score) | `outerHTML:#message-{id}` | Rendered `message_bubble.templ` |
| Reply delivery status change | `outerHTML:#reply-{id}` | Rendered `reply_bubble.templ` |
