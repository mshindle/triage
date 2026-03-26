# Research: Signal AI Triage Engine

**Branch**: `001-signal-ai-triage` | **Date**: 2026-03-25

## LLM Provider & Embedding Model

**Decision**: OpenAI via `openai-go/v3` (already in go.mod) for both triage reasoning
and embeddings.
- **Triage model**: `gpt-4o-mini` — structured JSON output via response format API
- **Embedding model**: `text-embedding-3-small` with `dimensions: 768` — matches the
  existing 768-dim schema without requiring a migration

**Rationale**: `openai-go/v3` is already present in `go.mod`. Using it for both triage
and embeddings avoids a second API dependency. OpenAI's `text-embedding-3-small` supports
dimension truncation to 768, preserving schema compatibility.

**Alternatives considered**:
- Google Gemini (text-embedding-004, native 768-dim) — requires `google.golang.org/genai`
  dependency not yet in go.mod
- Separate embedding provider — unnecessary complexity (YAGNI principle)

---

## WebSocket Client (Signal Bridge)

**Decision**: `golang.org/x/net/websocket` or `github.com/coder/websocket` (lightweight,
stdlib-friendly). Given the YAGNI principle, `coder/websocket` is preferred — it wraps
stdlib `net/http`, has no heavy dependencies, and handles reconnect logic cleanly.

**Rationale**: The Signal bridge exposes a single persistent WebSocket endpoint at
`ws://<host>/v1/receive/<phone>`. We need a client that reconnects on drop and processes
incoming JSON-RPC messages.

**Alternatives considered**:
- `gorilla/websocket` — archived (maintenance concern), heavier
- stdlib `net/http` upgrade — doesn't handle the read loop cleanly

---

## HTTP Router

**Decision**: Standard library `net/http` with `http.ServeMux`.

**Rationale**: The dashboard has ~5 routes. Introducing a router adds a dependency with
no proportional benefit at this scale. YAGNI.

**Alternatives considered**:
- `go-chi/chi` — reasonable but unnecessary
- `gin-gonic/gin` — too heavy, not idiomatic at this scope

---

## Database Driver

**Decision**: `github.com/jackc/pgx/v5` replacing `lib/pq`.

**Rationale**: pgx/v5 is the recommended driver for pgvector-go, supports context
cancellation properly, and has better performance. The `pgvector/pgvector-go` companion
library provides the `pgtype.Vector` type for clean embedding insertion/retrieval.

**Alternatives considered**:
- Keep `lib/pq` — lacks pgvector type support; raw `[]float32` marshaling needed;
  not recommended by pgvector-go maintainers

---

## Structured Logging

**Decision**: `github.com/rs/zerolog v1.34.0` (already in go.sum, needs direct dep).

**Rationale**: Already transitively present. Zero-allocation structured logging. JSON
output compatible with log aggregators. Meets Principle V requirement for key=value
or JSON log lines with `stage`, `signal_id`, `duration_ms` fields.

---

## Templ (Frontend Components)

**Decision**: `github.com/a-h/templ` for all HTML components; `go generate` step runs
`templ generate` before build.

**Rationale**: Type-safe SSR templates with Go. No JavaScript build pipeline needed.
htmx attributes are embedded directly in Templ component parameters.

**Pattern**: Generated `*_templ.go` files are committed to the repo. `templ generate`
must be run after any `.templ` file change.

---

## Real-Time Dashboard Updates

**Decision**: htmx WebSocket extension (`hx-ext="ws"`, `ws-connect="/ws"`).

**Rationale**: The server maintains a broadcast hub. When a new message is triaged, the
hub pushes an HTML fragment (rendered via Templ) to all connected clients. htmx swaps
the fragment into the DOM via `hx-swap-oob="afterbegin:#message-stream"`.

**Key constraint**: The element with `ws-connect` MUST NOT be swapped out by other htmx
actions (will drop the WebSocket connection).

---

## Schema Gap: Missing Migration 000002

The existing schema (000001) lacks:
1. **`triage_status` column on `messages`** — needed for FR-013 (surface triage failures)
   with values: `pending`, `complete`, `failed`
2. **HNSW index on `feedback_memory.embedding`** — needed for efficient similarity search
   in the feedback recall step (index absent in 000001)
3. **`replies` table** — needed to track outbound reply delivery status (FR-014)

Migration 000002 must add these before any implementation proceeds.

---

## Signal Bridge: Outbound Replies

**Decision**: `POST /v2/send` REST endpoint on the signal-cli-rest-api container.

**Payload**:
```json
{
  "message": "reply text",
  "number": "+1operator_phone",
  "recipients": ["+1recipient"] // or groupId for groups
}
```

**Group detection**: If `group_id` is set on the message, use the group ID in the
`recipients` array (as base64 group ID), not the sender phone.

---

## LLM Triage Prompt Design

The system prompt instructs the model to return strictly:
```json
{
  "priority": <int 0-100>,
  "category": "<string>",
  "reasoning": "<string>"
}
```

Few-shot context is prepended as a system message containing the top-K semantically
similar feedback entries (retrieved via pgvector cosine similarity on the new message's
embedding). K=5 is the default; configurable via `TRIAGE_FEEDBACK_K`.

**Validation**: Any response that fails `json.Unmarshal` into the struct, or where
`priority` is outside 0–100, MUST set `triage_status = 'failed'` and log the raw
response at DEBUG level.
