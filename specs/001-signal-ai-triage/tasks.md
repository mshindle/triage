# Tasks: Signal AI Triage Engine

**Input**: Design documents from `specs/001-signal-ai-triage/`
**Prerequisites**: plan.md ✅, spec.md ✅, data-model.md ✅, research.md ✅, contracts/ ✅

## Format: `[ID] [P?] [Story?] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: Which user story this task belongs to (US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: Add missing dependencies and configure code generation.

- [x] T001 Run `go get github.com/rs/zerolog@v1.34.0 github.com/jackc/pgx/v5 github.com/pgvector/pgvector-go github.com/a-h/templ github.com/coder/websocket` to promote zerolog to a direct dep and add new dependencies to go.mod
- [x] T002 [P] Create `generate.go` at the repository root with `//go:generate templ generate ./internal/web/templates/...`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before any user story can be implemented.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [x] T003 [P] Create `internal/store/migrations/000002_add_triage_status.up.sql`: ALTER TABLE messages ADD COLUMN triage_status TEXT NOT NULL DEFAULT 'pending'; CREATE INDEX ON feedback_memory USING hnsw (embedding vector_cosine_ops); CREATE TABLE replies (id SERIAL PRIMARY KEY, original_message_id INT REFERENCES messages(id), content TEXT NOT NULL, delivery_status TEXT NOT NULL DEFAULT 'pending', error_detail TEXT, created_at TIMESTAMPTZ DEFAULT NOW())
- [x] T004 [P] Create `internal/store/db.go`: Open(dbURL string) (*pgxpool.Pool, error) using pgxpool.New, register pgvector types with pgvector.RegisterTypes(ctx, pool), return pool; include zerolog log line on successful connection
- [x] T005 Create `internal/store/messages.go`: define Message struct matching schema (ID, SignalID, SenderPhone, Content, GroupID, Priority, Category, Reasoning, TriageStatus, Embedding, CreatedAt); implement InsertMessage(ctx, pool, msg) (int64, error) using INSERT ... ON CONFLICT (signal_id) DO NOTHING RETURNING id; GetMessages(ctx, pool) ([]Message, error) ordered by created_at DESC; UpdateMessageTriage(ctx, pool, id int64, priority int, category, reasoning, status string) error; UpdateMessagePriority(ctx, pool, id int64, priority int) error — all with zerolog stage=store logging
- [x] T006 [P] Create `internal/triage/analyzer.go`: define TriageResult struct {Priority int, Category string, Reasoning string}; Analyzer struct holding openai.Client and config (model, embedModel, embedDims); NewAnalyzer(cfg) *Analyzer; TriageMessage(ctx, content string, feedbackContext string) (TriageResult, error) — calls OpenAI chat completions with system prompt instructing JSON-only output of {priority, category, reasoning}, strict json.Unmarshal validation, returns error if priority outside 0–100; GenerateEmbedding(ctx, text string) ([]float32, error) — calls OpenAI embeddings API with configured dimensions; zerolog logs stage=triage with signal_id and duration_ms
- [x] T007 [P] Create `internal/signal/listener.go`: Listener struct holding wsURL string and onMessage func(envelope); NewListener(wsURL string, onMessage func(Envelope)) *Listener; Envelope struct {Source string, Content string, GroupID string, Timestamp int64}; Listen(ctx) error — connects to wsURL using coder/websocket, reads JSON-RPC frames, parses `envelope.source`, `envelope.dataMessage.message`, `envelope.dataMessage.groupInfo.groupId` into Envelope, calls onMessage; reconnect loop with exponential backoff (1s→2s→4s→…→30s max); zerolog logs stage=signal_listener with each connect, disconnect, reconnect attempt

**Checkpoint**: Run `go build ./...` — must compile without errors before proceeding to user stories.

---

## Phase 3: User Story 1 - Triage View (Priority: P1) 🎯 MVP

**Goal**: Operator opens dashboard and sees live-updating stream of triaged messages, color-coded by priority.

**Independent Test**: Start `./triage serve`, open `http://localhost:8081`, send a Signal message, verify it appears within 3 seconds with priority badge and color-coding without page refresh.

### Implementation for User Story 1

- [ ] T008 [P] [US1] Create `internal/web/templates/layout.templ`: base HTML document component with htmx CDN script tag, htmx WebSocket extension script tag (`htmx-ext-ws`), and a `{children}` slot for page body
- [ ] T009 [P] [US1] Create `internal/web/templates/message_card.templ`: MessageCard(msg store.Message) templ component — renders a `<div id="msg-{msg.ID}">` with sender, content, priority badge (integer 0–100), category label, reasoning summary, and CSS class `priority-high` (70–100), `priority-medium` (40–69), or `priority-low` (0–39); if msg.TriageStatus == "failed" render a "Triage Failed" indicator instead of priority badge (FR-013)
- [ ] T010 [P] [US1] Create `internal/web/templates/dashboard.templ`: Dashboard(messages []store.Message) templ component using layout.templ; includes `<div hx-ext="ws" ws-connect="/ws"><div id="message-stream">` with initial messages rendered via MessageCard loop; the ws-connect div MUST NOT be a swap target for any htmx action
- [ ] T011 [P] [US1] Create `internal/web/hub.go`: Hub struct with clients map[chan []byte]struct{} and broadcast chan []byte; NewHub() *Hub; Run(ctx) — goroutine that reads from broadcast channel and writes to all client channels, removing disconnected clients; Register(ch chan []byte); Unregister(ch chan []byte) — all operations serialized through the Run goroutine via select
- [ ] T012 [US1] Create `internal/web/handlers.go`: DashboardHandler(pool *pgxpool.Pool, hub *Hub) http.HandlerFunc — loads messages via store.GetMessages, renders dashboard.templ, returns 200; WSHandler(hub *Hub) http.HandlerFunc — upgrades to WebSocket, registers a per-client channel with hub, reads from channel and writes HTML frames to client, unregisters on disconnect
- [ ] T013 [US1] Create `internal/web/server.go`: Server struct holding pool, hub, analyzer, listener; New(pool, hub, analyzer, listener) *Server; RegisterRoutes(mux *http.ServeMux) — registers GET / → DashboardHandler, GET /ws → WSHandler; no other routes yet
- [ ] T014 [US1] Create `cmd/serve.go`: Cobra `serve` subcommand; on Run: open db pool via store.Open(viper.GetString("db_url")); create hub and start hub.Run in goroutine; create analyzer via triage.NewAnalyzer from viper config; create listener with onMessage callback (see T015); register server routes; start listener in goroutine; start http.ListenAndServe on viper.GetString("listen_addr")
- [ ] T015 [US1] Wire end-to-end pipeline in `cmd/serve.go` onMessage callback: (1) call store.InsertMessage — if error log and return; (2) call analyzer.GenerateEmbedding on content, store embedding via store.UpdateMessageEmbedding; (3) call analyzer.TriageMessage — on error call store.UpdateMessageTriage with triage_status="failed"; on success call store.UpdateMessageTriage with complete status; (4) load updated message from store; (5) render MessageCard templ component to bytes; (6) wrap in `<div hx-swap-oob="afterbegin:#message-stream">` and call hub.Broadcast
- [ ] T016 [US1] Run `go generate ./...` to produce `*_templ.go` files, then `go build -o triage .` — fix any compilation errors; run `./triage migrate` and `./triage serve`, verify dashboard loads at http://localhost:8081 and shows existing messages

**Checkpoint**: User Story 1 is fully functional — messages arrive, get triaged, appear in dashboard within 3 seconds, color-coded by priority, without page refresh.

---

## Phase 4: User Story 2 - Semantic Feedback Loop (Priority: P2)

**Goal**: Operator clicks priority correction on a message card; AI uses that correction as context for future similar messages.

**Independent Test**: Click "Mark High Priority" on a message, verify priority badge updates immediately; send a semantically similar message and confirm its AI-assigned priority is higher than it would be without the correction.

### Implementation for User Story 2

- [ ] T017 [P] [US2] Create `internal/store/feedback.go`: FeedbackMemory struct matching schema (ID, OriginalMessageID, FeedbackText, AdjustedPriority, Embedding, CreatedAt); InsertCorrection(ctx, pool, messageID int64, feedbackText string, adjustedPriority int, embedding []float32) error; RecallSimilar(ctx, pool, queryEmbedding []float32, k int) ([]FeedbackMemory, error) — SELECT ... ORDER BY embedding <=> $1 LIMIT $2 (cosine distance on feedback_memory HNSW index)
- [ ] T018 [US2] Update `internal/triage/analyzer.go`: add BuildFeedbackContext(memories []store.FeedbackMemory) string — formats memories as "Past correction: [text] → priority [N]" lines; update TriageMessage signature to accept feedbackContext string and prepend it to the system prompt when non-empty; zerolog log stage=triage with feedback_count field
- [ ] T019 [US2] Update `internal/web/templates/message_card.templ`: add "Mark High Priority" button `<button hx-post="/messages/{msg.ID}/feedback" hx-vals='{"direction":"high"}' hx-target="#msg-{msg.ID}" hx-swap="outerHTML">` and "Mark Low Priority" button with direction=low; only render buttons when TriageStatus != "failed"
- [ ] T020 [US2] Add FeedbackHandler(pool *pgxpool.Pool, hub *Hub, analyzer *triage.Analyzer) http.HandlerFunc to `internal/web/handlers.go`: parse {id} from URL path and direction from form; map direction to adjustedPriority (high→90, low→10); call store.UpdateMessagePriority; load message, call analyzer.GenerateEmbedding, call store.InsertCorrection; render updated MessageCard to bytes; broadcast `<div hx-swap-oob="outerHTML:#msg-{id}">` fragment via hub; return updated card as response body for direct htmx swap
- [ ] T021 [US2] Register `POST /messages/{id}/feedback` route in `internal/web/server.go` Server.RegisterRoutes, passing pool, hub, and analyzer to FeedbackHandler
- [ ] T022 [US2] Update the onMessage pipeline in `cmd/serve.go`: before calling analyzer.TriageMessage, call store.RecallSimilar(ctx, pool, embedding, feedbackK) and analyzer.BuildFeedbackContext; pass feedbackContext into TriageMessage call

**Checkpoint**: User Stories 1 AND 2 work independently — triage view functional, feedback corrections persist and influence future AI decisions.

---

## Phase 5: User Story 3 - Direct Reply from Dashboard (Priority: P3)

**Goal**: Operator types a reply in the dashboard and it is delivered via Signal to the original sender or group.

**Independent Test**: Click reply on a message card, type text, submit; verify the Signal recipient receives the message and the delivery status badge shows "Delivered". Test with unavailable bridge to verify "Failed" badge appears.

### Implementation for User Story 3

- [ ] T023 [P] [US3] Create `internal/signal/sender.go`: Sender struct holding restBaseURL and phoneNumber (operator's number from config); NewSender(baseURL, phone string) *Sender; SendReply(ctx, msg store.Message, content string) error — if msg.GroupID != "" POST to /v2/send with groupId field, else POST with recipients=[msg.SenderPhone]; marshal JSON body, execute HTTP POST to signal bridge, return error on non-2xx response; zerolog log stage=signal_sender with signal_id and duration_ms
- [ ] T024 [P] [US3] Create `internal/store/replies.go`: Reply struct matching schema (ID, OriginalMessageID, Content, DeliveryStatus, ErrorDetail, CreatedAt); InsertReply(ctx, pool, messageID int64, content string) (int64, error); UpdateDeliveryStatus(ctx, pool, replyID int64, status, errDetail string) error
- [ ] T025 [US3] Update `internal/web/templates/message_card.templ`: add reply form `<form hx-post="/messages/{msg.ID}/reply" hx-target="#reply-status-{msg.ID}" hx-swap="innerHTML"><textarea name="content"></textarea><button type="submit">Send</button></form>`; add delivery status span `<span id="reply-status-{msg.ID}">` (empty initially; swapped on reply response)
- [ ] T026 [US3] Add ReplyHandler(pool *pgxpool.Pool, hub *Hub, sender *signal.Sender) http.HandlerFunc to `internal/web/handlers.go`: parse {id} from URL, parse content from form body; return 400 on empty content; load message from store; call store.InsertReply → replyID; call sender.SendReply; on success call store.UpdateDeliveryStatus(replyID, "delivered", ""); on bridge error call store.UpdateDeliveryStatus(replyID, "failed", err.Error()); render and return delivery status HTML fragment; log failure with zerolog at error level
- [ ] T027 [US3] Register `POST /messages/{id}/reply` route in `internal/web/server.go` Server.RegisterRoutes; add sender *signal.Sender field to Server struct; create Sender in cmd/serve.go using TRIAGE_SIGNAL_REST_URL and TRIAGE_PHONE viper config keys

**Checkpoint**: All three user stories independently functional — triage view, feedback loop, and direct reply all work end-to-end.

---

## Phase N: Polish & Cross-Cutting Concerns

**Purpose**: Complete observability, error surfacing, and validate the full system against success criteria.

- [ ] T028 [P] Audit all pipeline stages in `internal/signal/listener.go`, `internal/triage/analyzer.go`, `internal/store/messages.go`, `internal/store/feedback.go`: ensure every function emits a zerolog JSON log line with fields `stage`, `signal_id`, and `duration_ms` (use time.Now() before/after calls) per Constitution Principle V
- [ ] T029 [P] Add CSS classes to `internal/web/templates/layout.templ` or a static stylesheet: `.priority-high { border-left: 4px solid #dc2626; }` (red), `.priority-medium { border-left: 4px solid #f59e0b; }` (amber), `.priority-low { border-left: 4px solid #16a34a; }` (green), `.triage-failed { border-left: 4px solid #6b7280; opacity: 0.7; }` — verifies SC color-coding requirement
- [ ] T030 Run `go generate ./... && go build -o triage .` for a clean final build; run `./triage migrate` against a fresh database; run through quickstart.md validation steps end-to-end to confirm all six success criteria (SC-001 through SC-006) are met

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 completion; T003, T004, T006, T007 can run in parallel; T005 depends on T004
- **User Story 1 (Phase 3)**: Depends on ALL of Phase 2; T008, T009, T010, T011 can run in parallel; T012 depends on T011; T013 depends on T012; T014 depends on T013; T015 depends on T014, T005, T006; T016 depends on T015
- **User Story 2 (Phase 4)**: Depends on Phase 3 completion; T017 can run in parallel with T018–T019; T020 depends on T017, T018, T019; T021 depends on T020; T022 depends on T018
- **User Story 3 (Phase 5)**: Depends on Phase 3 completion (not Phase 4); T023, T024 can run in parallel; T025 depends on Phase 3 T009; T026 depends on T023, T024, T025; T027 depends on T026
- **Polish (Phase N)**: Depends on all desired user stories complete

### Within Each User Story

- Templ components before handlers (handlers import generated templ functions)
- Hub before handlers (handlers reference Hub type)
- Handlers before server (server registers handlers)
- Server before serve command (command instantiates server)
- Store functions before command wiring (pipeline requires store functions)

---

## Parallel Opportunities

```bash
# Phase 2: Launch all independent foundational tasks together
Task: "Create internal/store/migrations/000002_add_triage_status.up.sql" (T003)
Task: "Create internal/store/db.go" (T004)
Task: "Create internal/triage/analyzer.go" (T006)
Task: "Create internal/signal/listener.go" (T007)
# Then T005 (messages.go) after T004

# Phase 3: Launch all templ + hub tasks together
Task: "Create internal/web/templates/layout.templ" (T008)
Task: "Create internal/web/templates/message_card.templ" (T009)
Task: "Create internal/web/templates/dashboard.templ" (T010)
Task: "Create internal/web/hub.go" (T011)
# Then T012 (handlers) after T011 + templ files exist

# Phase 4: Feedback store + template update in parallel
Task: "Create internal/store/feedback.go" (T017)
Task: "Update internal/web/templates/message_card.templ" (T019)
# Then T018 (update analyzer) + T020 (handler) after T017 + T019
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational — run `go build ./...` checkpoint
3. Complete Phase 3: User Story 1 (T008–T016)
4. **STOP and VALIDATE**: Open dashboard, send a Signal message, confirm it appears with triage within 3 seconds
5. Deploy/demo if ready — full message ingestion and triage view is the core value

### Incremental Delivery

1. Phase 1 + Phase 2 → Foundation ready (`go build ./...` passes)
2. Phase 3 → Triage View → validate SC-001, SC-002, SC-005, SC-006 → **MVP complete**
3. Phase 4 → Feedback Loop → validate SC-003
4. Phase 5 → Reply → validate SC-004
5. Phase N → Polish → validate all success criteria together

---

## Notes

- `[P]` tasks operate on different files with no blocking dependencies — safe to parallelize
- `[USn]` label maps each task to a specific user story for traceability
- Templ-generated `*_templ.go` files must be committed after `go generate`
- After any `.templ` file change: re-run `go generate ./...` before building
- Migration 000002 must be applied (via `./triage migrate`) before `./triage serve`
- The `ws-connect` div in dashboard.templ MUST NOT be a swap target — swapping it disconnects the WebSocket (see contracts/websocket.md)
