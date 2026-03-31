# Tasks: Signal AI Triage Dashboard UI

**Input**: Design documents from `specs/002-signal-triage-ui/`
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, contracts/ ✅

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story. No test tasks are included (not requested in spec).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: Which user story this task belongs to (US1–US4)
- File paths are absolute from repo root `github.com/mshindle/triage`

---

## Phase 1: Setup (Schema & Shared Infrastructure)

**Purpose**: Apply schema migration and update shared types before any story work begins.

- [x] T001 [P] Create `internal/store/migrations/000003_add_feedback_category.up.sql` — `ALTER TABLE feedback_memory ADD COLUMN adjusted_category TEXT;` *(note: created as 000003 since 000002 already existed)*
- [x] T002 [P] Create `internal/store/migrations/000003_add_feedback_category.down.sql` — `ALTER TABLE feedback_memory DROP COLUMN adjusted_category;`
- [x] T003 Update `internal/store/feedback.go` — added `AdjustedCategory *string` to `FeedbackMemory`; updated `InsertCorrection()` to accept `adjustedCategory *string`; updated call site in `handlers.go` to pass `nil`

**Checkpoint**: Migration files exist and `FeedbackMemory` struct is updated. Run `./triage migrate` to apply.

---

## Phase 2: Foundational (Three-Panel Shell — Blocks All Stories)

**Purpose**: The three-panel layout shell, reusable components, and server routes must exist before any user story panel can be wired up.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [x] T004 Rewrite `internal/web/templates/dashboard.templ` — three-panel flex layout with `#conversation-list-panel` (280px), `#thread-panel` (flex-1), `#detail-panel` (360px); `hx-ext="ws"` + `ws-connect="/ws"` on wrapper; `DashboardHandler` updated to call `templates.Dashboard()` with no args
- [x] T005 Update `internal/web/templates/layout.templ` — full-height flex-col body; `#ws-status-dot` in nav bar; ~50 lines vanilla JS for htmx:wsOpen/wsClose events and page-title badge
- [x] T006 Add new routes to `internal/web/server.go` — `GET /conversations`, `GET /conversations/:id/thread`, `GET /messages/:id/detail` added; stub handlers with 200 placeholder HTML added to `handlers.go`
- [x] T007 [P] Create `internal/web/templates/priority_gauge.templ` — progress bar + large score; red/amber/green fill; aria progressbar
- [x] T008 [P] Create `internal/web/templates/category_pill.templ` — small uppercase muted badge; renders nothing when category is empty

**Checkpoint**: `go build -o triage` succeeds. The app starts and shows an empty three-panel layout at `/`. No panels are populated yet.

---

## Phase 3: User Story 1 — Real-Time Message Monitoring (Priority: P1) 🎯 MVP

**Goal**: Operator sees all conversations in the left panel, selects one to load the message thread in the center panel, and receives real-time updates as new messages arrive.

**Independent Test**: Load the dashboard; verify the conversation list is populated with sender identities and priority dots. Select a conversation; verify the thread loads with message bubbles showing priority strips and category pills. Send a test Signal message; verify it appears in the left panel within 3 seconds without a page reload.

- [x] T009 Create `internal/store/conversations.go` — `ConversationSummary` struct with `IsGroup()`/`DisplayName()` methods; `GetConversations()` using CTE + window function for latest-message fields; ordered by `last_message_at DESC`
- [x] T010 Add `GetMessagesByConversation()` to `internal/store/conversations.go` — identity-based routing: `group:{id}` → filter by `group_id`, else filter by `sender_phone`; columns exclude embedding
- [x] T011 [P] Create `internal/web/templates/conversation_list.templ` — `ConversationList()` and `ConversationListBroadcast()` (OOB variant); search bar (disabled stub), sort dropdown, `role="listbox"` list
- [x] T012 [P] Create `internal/web/templates/conversation_item.templ` — avatar with initials, priority dot, category pill, relative timestamp; `url.PathEscape` on identity for safe `hx-get` URL
- [x] T013 Update `internal/web/handlers.go` — `DashboardHandler` calls `GetConversations()`; `ConversationListHandler` fully implemented
- [x] T014 [P] Create `internal/web/templates/message_thread.templ` — thread header, `role="log"` stream, reply composer placeholder
- [x] T015 [P] Create `internal/web/templates/message_bubble.templ` — priority border class, category pill, triage status badge, detail panel click trigger
- [x] T016 Add `ThreadHandler` to `internal/web/handlers.go` — calls `GetMessagesByConversation()`, renders `MessageThread`
- [x] T017 Create `internal/web/templates/header.templ` — filter bar with disabled priority/category/status stubs
- [x] T018 Update `internal/signal/pipeline.go` — broadcasts `ConversationListBroadcast` OOB after triage; logs `stage=broadcast signal_id duration_ms conversations`

**Checkpoint**: US1 fully functional. Conversation list populates, thread loads on click, new messages appear in real time. Verify with `go test ./internal/store/... ./internal/web/...`.

---

## Phase 4: User Story 2 — AI Triage Feedback & Correction (Priority: P2)

**Goal**: Operator clicks a message bubble to open the right panel showing the full triage result (score, category, reasoning), submits a priority and/or category correction with optional text, sees the update reflected immediately, and can view all past corrections for that message.

**Independent Test**: Click a triaged message; verify the detail panel shows priority score with gauge, category badge, and reasoning. Submit a feedback correction (change priority + category + add text); verify the priority dot in the conversation list updates and the feedback appears in the history section below.

- [x] T019 Add `GetFeedbackByMessage(ctx context.Context, messageID int64) ([]store.FeedbackMemory, error)` to `internal/store/conversations.go` — returns all feedback entries for a message ordered by `created_at DESC`; also added `GetCategories()` here (needed by T022)
- [x] T020 [P] Create `internal/web/templates/triage_detail.templ` — right panel; accepts `store.Message` and `[]store.FeedbackMemory`; renders: message summary (full content, sender, timestamp, signal_id), a triage result card with `priority_gauge` component, `category_pill`, and a collapsible reasoning block (expanded by default); a feedback form section (`feedbackForm` component); a feedback history list showing past corrections with adjusted priority, adjusted category, feedback text, and timestamp; `id="detail-panel"` on the outer wrapper
- [x] T021 [P] Create `internal/web/templates/feedback_form.templ` — feedback form; accepts `messageID int64`, `currentPriority int`, `currentCategory string`; renders: two quick-action buttons ("Mark High (85)" → sets priority=85, "Mark Low (20)") plus a 0–100 range slider; a category `<select>` dropdown (categories passed as `[]string`); a free-text `<textarea name="text">`; a Submit button; form uses `hx-post="/messages/{id}/feedback"` `hx-target="#detail-panel"` `hx-swap="outerHTML"`; feedback controls have `aria-disabled` when `triage_status != "completed"`; uses templ `script` components for JS event handlers
- [x] T022 Add `DetailHandler` to `internal/web/handlers.go` — fetches `store.Message` by ID (`GetMessageByID` added to `internal/store/messages.go`), fetches `store.GetFeedbackByMessage()`, fetches `store.GetCategories()` for the dropdown, renders `triage_detail.templ` partial; `messageBubble` exported as `MessageBubble` wrapper for handler OOB use; message bubble changed to `hx-swap="outerHTML"` for consistency
- [x] T023 Update `FeedbackHandler` in `internal/web/handlers.go` — parses `priority` as int (fallback to `direction` for backward compat), `category`, and `text` form fields; calls `store.InsertCorrection()` with adjustedCategory; reloads fresh message+feedback+categories; re-renders `triage_detail.templ`; OOB broadcasts `MessageBubble` targeting `#message-{id}`

**Checkpoint**: US2 fully functional. Detail panel opens on message click, feedback form submits, history list grows. Run `go test ./internal/store/... ./internal/web/...`.

---

## Phase 5: User Story 3 — Reply to Message Sender (Priority: P3)

**Goal**: Operator types a reply in the composer at the bottom of the thread, sends it, sees it appear as an outgoing bubble, and watches the delivery status update (pending → sent/failed) when the backend confirms.

**Independent Test**: Select a conversation, type a reply, click Send. Verify an outgoing bubble appears with a "pending" indicator. Verify the delivery status updates to "sent" or "failed" within 5 seconds (requires the Signal bridge to be running).

- [x] T024 [P] Create `internal/web/templates/reply_bubble.templ` — outgoing reply bubble; accepts `store.Reply`; renders: `id="reply-{id}"`, right-aligned bubble with blue background, content text, timestamp, delivery status indicator (⏳ pending / ✓ sent / ✗ failed with error tooltip); `aria-label` describes delivery status; exported `ReplyBubble` wrapper for handler use
- [x] T025 [P] Create `internal/web/templates/reply_composer.templ` — bottom text input; accepts `messageID int64`; auto-expanding textarea via templ `script` component; form uses `hx-post="/messages/{id}/reply"` `hx-target="#reply-composer"` `hx-swap="outerHTML"`; exported `ReplyComposer` wrapper for handler use
- [x] T026 Update `internal/web/templates/message_thread.templ` — signature updated to `MessageThread(identity, messages, replyMap map[int64][]store.Reply, lastMsgID int64)`; replies rendered after each parent message; composer placeholder replaced with `replyComposer(lastMsgID)`; `GetRepliesByMessageIDs` added to `internal/store/replies.go`; `ThreadHandler` updated to fetch replies and build replyMap
- [x] T027 Update `ReplyHandler` in `internal/web/handlers.go` (note: delivery broadcast happens here, not in pipeline.go) — switched to `GetMessageByID`; renders `ReplyBubble` and `ReplyComposer`; broadcasts `afterbegin:#message-stream` OOB to all clients; primary response is fresh composer + OOB prepend for submitting client; logs `stage=reply_status_broadcast`

**Checkpoint**: US3 fully functional. Replies send and appear in thread. Delivery status updates in real time. `ReplyHandler` is unchanged; only template and pipeline broadcast are added.

---

## Phase 6: User Story 4 — Filtered Conversation View (Priority: P4)

**Goal**: Operator clicks priority/category/status filter chips in the header to narrow the conversation list. Active filter count is shown. "Clear all" restores the full list. Filters persist in the URL.

**Independent Test**: Click "High Priority" filter; verify only conversations with `highest_priority >= 70` appear. Add a category filter; verify both filters apply (AND logic). Click "Clear all"; verify full list returns. Reload the page with `?priority=high` in the URL; verify the filter is pre-applied.

- [x] T028 Update `GetConversations()` in `internal/store/conversations.go` — added `ConversationFilters` struct with `ActiveCount()` method; updated SQL to use HAVING for priority/category/status filters and conditional ORDER BY for sort; pipeline.go updated to pass `ConversationFilters{}`
- [x] T029 Add `GetCategories(ctx context.Context) ([]string, error)` to `internal/store/conversations.go` — already implemented in Phase 4
- [x] T030 Update `internal/web/handlers.go` — added `parseFilters()` helper; `DashboardHandler` parses filters, fetches categories, passes both to template; `ConversationListHandler` parses filters and calls updated `GetConversations()`
- [x] T031 Update `internal/web/templates/header.templ` — fully rewritten with functional filter form: priority checkboxes styled as chips (toggle via `hx-trigger="change"`), category `<select>`, status `<select>`, active count badge, "Clear all" link with `clearAllFilters()` script component; `dashboard.templ` updated to accept and pass `categories []string` and `filters ConversationFilters`
- [x] T032 Update `internal/web/templates/conversation_list.templ` — `ConversationList` and `conversationListContent` accept `filters ConversationFilters`; sort dropdown wired with `hx-include="[name='priority'],[name='category'],[name='status']"` and `hx-push-url="true"`; selected sort option marked based on `filters.Sort`; `ConversationListBroadcast` unchanged (passes empty filters internally)

**Checkpoint**: US4 fully functional. All four filter dimensions work independently and in combination. URL reflects filter state. `go test ./internal/store/... ./internal/web/...`.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Responsive behavior, accessibility audit, cleanup.

- [ ] T033 Add responsive CSS to `internal/web/templates/layout.templ` and `dashboard.templ` — Tailwind breakpoints: `lg:flex` for three-panel desktop; `md:` left panel collapses to icon-width strip; `<md` add a three-tab navigation bar (Conversations / Thread / Detail) with ~20 lines of vanilla JS toggling `hidden` class between the three `<section>` panel wrappers
- [ ] T034 [P] Audit all new templ components for ARIA compliance — verify `role="listbox"` + `role="option"` + `aria-selected` on conversation list, `role="log"` + `aria-live="polite"` on `#message-stream`, `aria-label` on all icon-only buttons, `aria-disabled` on disabled feedback controls, minimum 4.5:1 contrast ratio on all text/background combinations
- [ ] T035 [P] Delete `internal/web/templates/message_card.templ` — the old single-card component is fully superseded by `message_bubble.templ`, `triage_detail.templ`, and `feedback_form.templ`; verify no remaining imports before deletion
- [ ] T036 Validate `specs/002-signal-triage-ui/quickstart.md` instructions against final implementation — run through each step end-to-end; update any commands or file paths that changed during implementation

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately; T001 and T002 are parallel
- **Foundational (Phase 2)**: Depends on Phase 1 completion — **BLOCKS all user stories**
- **US1 (Phase 3)**: Depends on Phase 2 — no other story dependency
- **US2 (Phase 4)**: Depends on Phase 2 + T003 (FeedbackMemory struct) — no US1 dependency
- **US3 (Phase 5)**: Depends on Phase 2 + US1 (thread panel must exist for composer)
- **US4 (Phase 6)**: Depends on Phase 2 + US1 (conversation list must exist for filters)
- **Polish (Phase 7)**: Depends on all story phases complete

### User Story Dependencies

- **US1 (P1)**: No story dependencies — start after Phase 2
- **US2 (P2)**: No story dependencies — start after Phase 2 (FeedbackMemory struct from Phase 1)
- **US3 (P3)**: Depends on US1 (thread panel + `message_thread.templ` must exist)
- **US4 (P4)**: Depends on US1 (conversation list + `GetConversations()` must exist)

### Within Each User Story

- Store queries before handlers (handlers call store)
- Templates before handlers (handlers render templates)
- Templates within a story marked [P] can be created in parallel
- Handler updates are sequential (all in `handlers.go`)

### Parallel Opportunities

- T001 ∥ T002 (different migration files)
- T007 ∥ T008 (different template files, no dependencies)
- After Phase 2: US1 ∥ US2 (independent — different store queries, different templates, different handler additions)
- Within US1: T011 ∥ T012 ∥ T014 ∥ T015 (all different template files)
- Within US2: T020 ∥ T021 (different files)
- Within US3: T024 ∥ T025 (different template files)
- T034 ∥ T035 (different files, no dependencies)

---

## Parallel Example: User Story 1

```bash
# After T009 and T010 (store queries) complete, launch template creation in parallel:
Task T011: "Create internal/web/templates/conversation_list.templ"
Task T012: "Create internal/web/templates/conversation_item.templ"
Task T014: "Create internal/web/templates/message_thread.templ"
Task T015: "Create internal/web/templates/message_bubble.templ"

# Then sequentially:
Task T013: "Update DashboardHandler + implement ConversationListHandler in handlers.go"
Task T016: "Add ThreadHandler to handlers.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001–T003)
2. Complete Phase 2: Foundational (T004–T008) — **critical blocker**
3. Complete Phase 3: US1 (T009–T018)
4. **STOP and VALIDATE**: Run app, open browser, verify conversation list + thread load + real-time updates
5. Demo / ship MVP

### Incremental Delivery

1. Phase 1 + 2 → Three-panel shell visible (empty panels)
2. + US1 → Conversation browsing + real-time monitoring works (MVP)
3. + US2 → Feedback and triage correction works
4. + US3 → Reply sending works
5. + US4 → Filtering works
6. + Polish → Responsive + accessibility complete

---

## Notes

- [P] tasks = different files, safe to parallelize
- `handlers.go` is a single file — all handler additions are sequential even if [P] not marked
- `conversations.go` is a new file — functions added sequentially (same file)
- After adding each templ component, run `templ generate` before `go build`
- `message_card.templ` must NOT be deleted until US1 templates fully replace its functionality (T035 is last)
- Total tasks: **36** across 7 phases
