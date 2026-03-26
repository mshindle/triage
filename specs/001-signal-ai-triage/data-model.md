# Data Model: Signal AI Triage Engine

**Branch**: `001-signal-ai-triage` | **Date**: 2026-03-25

## Entities

---

### Message

The primary record for every Signal communication received by the system.
Persisted before any triage processing; never deleted.

| Field          | Type            | Constraints                        | Description |
|----------------|-----------------|------------------------------------|-------------|
| id             | integer         | PK, auto-increment                 | Surrogate key |
| signal_id      | text            | UNIQUE, NOT NULL                   | Signal-assigned message identifier (idempotency key per Principle I) |
| sender_phone   | text            | NOT NULL                           | E.164 sender phone number |
| content        | text            | NOT NULL                           | Raw message text |
| group_id       | text            | nullable                           | Signal group identifier; NULL for direct messages |
| priority       | integer         | DEFAULT 50, CHECK 0–100            | AI-assigned or user-overridden priority score |
| category       | text            | nullable                           | AI-assigned category (e.g., Personal, Work, Urgent) |
| reasoning      | text            | nullable                           | Plain-language rationale from AI triage |
| triage_status  | text            | DEFAULT 'pending'                  | Pipeline status: `pending`, `complete`, `failed` |
| embedding      | vector(768)     | nullable                           | Semantic embedding for feedback recall |
| created_at     | timestamptz     | DEFAULT now()                      | Ingestion timestamp (used for chronological ordering) |

**Indexes**:
- `UNIQUE` on `signal_id` — prevents duplicate ingestion
- `HNSW` on `embedding vector_cosine_ops` — fast feedback recall (migration 000001)
- `BTREE` on `created_at DESC` — chronological list queries (migration 000001)
- `BTREE` on `group_id` — group filtering (migration 000001)

**State transitions for `triage_status`**:
```
pending → complete   (LLM returns valid JSON, stored successfully)
pending → failed     (LLM unavailable, JSON invalid, or priority out of range)
complete → complete  (user override via feedback — priority updated, status stays complete)
```

---

### FeedbackMemory

Represents a user-submitted priority correction for a specific message.
Each correction is embedded and stored to influence future triage decisions.

| Field               | Type        | Constraints              | Description |
|---------------------|-------------|--------------------------|-------------|
| id                  | integer     | PK, auto-increment       | Surrogate key |
| original_message_id | integer     | FK → messages(id)        | The message being corrected |
| feedback_text       | text        | NOT NULL                 | Human-readable description of the correction (e.g., "Mark High Priority: message about urgent outage") |
| adjusted_priority   | integer     | CHECK 0–100              | The user-specified target priority |
| embedding           | vector(768) | NOT NULL                 | Embedding of `feedback_text` for similarity recall |
| created_at          | timestamptz | DEFAULT now()            | Correction timestamp |

**Indexes**:
- `HNSW` on `embedding vector_cosine_ops` — fast similarity retrieval (migration 000002)
- `BTREE` on `original_message_id` — lookup corrections by message

**Relationship**: Many corrections can exist for a single message (user may adjust
priority multiple times; each adjustment creates a new record).

---

### Reply

Tracks outbound messages sent from the dashboard back through the Signal bridge.
Provides delivery status visibility (FR-014: failures must not be silently discarded).

| Field               | Type        | Constraints              | Description |
|---------------------|-------------|--------------------------|-------------|
| id                  | integer     | PK, auto-increment       | Surrogate key |
| original_message_id | integer     | FK → messages(id)        | The message being replied to |
| content             | text        | NOT NULL                 | Reply text |
| delivery_status     | text        | DEFAULT 'pending'        | `pending`, `delivered`, `failed` |
| error_detail        | text        | nullable                 | Error description if delivery_status = 'failed' |
| created_at          | timestamptz | DEFAULT now()            | Submission timestamp |

**State transitions for `delivery_status`**:
```
pending → delivered  (Signal bridge returns 2xx)
pending → failed     (Signal bridge returns error or network timeout)
```

---

## Schema Migration Plan

| Migration   | Change                                          | Reason |
|-------------|-------------------------------------------------|--------|
| 000001 (existing) | Enable pgvector; create messages, feedback_memory; HNSW on messages | Baseline schema |
| 000002 (new) | Add `triage_status` to messages; add HNSW on feedback_memory; create replies table | FR-013, FR-014, feedback recall performance |

---

## Entity Relationships

```
messages (1) ←──── (N) feedback_memory
messages (1) ←──── (N) replies
```

- A message may have zero or more corrections (feedback_memory records)
- A message may have zero or more replies
- feedback_memory and replies are never orphaned; cascade behavior:
  - Corrections: retain even if message is archived (embeddings are the persistent
    teaching signal)
  - Replies: retain for audit trail

---

## Vector Dimensions

All embedding columns use `vector(768)`, compatible with:
- OpenAI `text-embedding-3-small` with `dimensions: 768`
- Google `text-embedding-004` (native 768-dim)

Changing the model to one with a different native dimension MUST be accompanied by
a new migration that drops and recreates embedding columns and indexes.
