# Implementation Plan: Signal AI Triage Dashboard UI

**Branch**: `002-signal-triage-ui` | **Date**: 2026-03-30 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `specs/002-signal-triage-ui/spec.md`

## Summary

Replace the single-page message-card feed with a three-panel Signal-inspired dashboard: a conversation list (left), message thread (center), and triage detail + feedback panel (right). All server-side rendering uses Go Templ + htmx; panel switching and filter navigation use htmx partial loads targeting named panel elements. Real-time updates continue via the existing WebSocket broadcast hub using htmx OOB swaps. A lightweight vanilla-JS layer handles delivery-status updates and connection-state indicators. One new migration adds `adjusted_category` to `feedback_memory` to support category-override feedback. No new packages, no schema redesign.

## Technical Context

**Language/Version**: Go 1.26
**Primary Dependencies**: Echo v4 (HTTP), coder/websocket v1.8.14, templ v0.3.1001, htmx 2.0 (CDN), Uber Fx v1.24 (existing DI), zerolog v1.34
**Storage**: PostgreSQL 17 + pgvector; no new tables — `messages`, `feedback_memory`, `replies` cover all data needs; one new column via migration 000002
**Testing**: `go test ./...`; htmx partial endpoints tested via HTTP handler tests with rendered HTML assertions
**Target Platform**: Linux server; operator uses a desktop browser (Chrome/Firefox/Safari)
**Project Type**: web-service
**Performance Goals**: Conversation list renders ≤500 conversations; message thread renders ≤1,000 messages; both must be visually ready within 3 seconds of page load
**Constraints**: No client-side JS frameworks; no ORMs; no new config keys; templ + htmx for all interactive UI; vanilla JS only where htmx cannot reach (delivery status dot, WS connection indicator)
**Scale/Scope**: ~500 conversations, ~1,000 messages per thread; single concurrent operator

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I — Message Integrity | ✅ PASS | Feature is UI-only; no changes to ingestion, pipeline, or `signal_id` uniqueness |
| II — Schema-Versioned Storage | ✅ PASS | `adjusted_category` column added via `000002_add_feedback_category.up.sql`; no DDL outside migrations |
| III — Structured LLM Contracts | ✅ PASS | Triage analyzer not touched |
| IV — YAGNI | ✅ PASS with care | 12 new templ components all have distinct, non-speculative use sites; new store queries each called from exactly one handler; no new abstractions for their own sake |
| V — Observable Pipeline | ✅ PASS with care | Broadcast hub events should log `stage=broadcast`, `signal_id`, and `duration_ms`; existing pipeline log calls are not changed |

No violations requiring Complexity Tracking justification.

## Project Structure

### Documentation (this feature)

```text
specs/002-signal-triage-ui/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/
│   ├── ws-events.md     # WebSocket event protocol (server → client)
│   └── http-routes.md   # HTMX partial endpoint contracts
└── tasks.md             # Phase 2 output (/speckit.tasks — not created here)
```

### Source Code (repository root)

```text
internal/web/
├── server.go                     # Updated: new HTMX partial routes
├── handlers.go                   # Updated: ConversationListHandler, ThreadHandler, DetailHandler; updated FeedbackHandler (category support)
├── hub.go                        # Unchanged
├── render.go                     # Unchanged
└── templates/
    ├── layout.templ               # Updated: Tailwind config, WS JS, connection status
    ├── dashboard.templ            # Rewritten: three-panel flex layout
    ├── header.templ               # New: app title + filter bar + connection status
    ├── conversation_list.templ    # New: left panel with search + sort + items
    ├── conversation_item.templ    # New: single conversation row (avatar, priority dot, preview)
    ├── message_thread.templ       # New: center panel — thread header + bubbles + composer
    ├── message_bubble.templ       # New: incoming message bubble with priority strip
    ├── reply_bubble.templ         # New: outgoing reply bubble with delivery indicator
    ├── reply_composer.templ       # New: textarea + send button (htmx POST)
    ├── triage_detail.templ        # New: right panel — summary + triage card + feedback + history
    ├── feedback_form.templ        # New: priority adjuster + category dropdown + textarea + submit
    ├── priority_gauge.templ       # New: reusable priority bar (0–100 → colored fill)
    └── category_pill.templ        # New: reusable category badge
    # message_card.templ           → deleted (replaced by above)

internal/store/
├── conversations.go               # New: GetConversations(), GetMessagesByConversation(), GetFeedbackByMessage(), GetCategories()
└── migrations/
    ├── 000001_initial.up.sql      # Existing (unchanged)
    └── 000002_add_feedback_category.up.sql  # New
```

**Structure Decision**: Single project layout — this feature is purely additive within `internal/web/` and `internal/store/`. No new packages or submodules required.
