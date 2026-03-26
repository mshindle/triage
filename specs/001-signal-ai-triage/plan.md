# Implementation Plan: Signal AI Triage Engine

**Branch**: `001-signal-ai-triage` | **Date**: 2026-03-25 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/001-signal-ai-triage/spec.md`

## Summary

Build a Go web service that ingests Signal messages via WebSocket, runs AI triage
(priority 0–100, category, reasoning) using OpenAI, persists all results to
PostgreSQL+pgvector, and serves a real-time dashboard via Go Templ + htmx. A semantic
feedback loop stores user priority corrections as vector embeddings and uses them as
few-shot context for future triage decisions.

## Technical Context

**Language/Version**: Go 1.26 (module `github.com/mshindle/triage`)
**Primary Dependencies**: cobra v1.10.2, viper v1.21.0, golang-migrate v4, openai-go v3,
  a-h/templ, rs/zerolog v1.34.0, jackc/pgx/v5, pgvector/pgvector-go, coder/websocket
**Storage**: PostgreSQL 17 + pgvector (768-dim embeddings, HNSW indexes)
**Testing**: `go test ./...` (stdlib)
**Target Platform**: Linux server (Docker Compose environment)
**Project Type**: web-service + cli
**Performance Goals**: <3s message-to-dashboard latency; <5s reply delivery
**Constraints**: Single-operator, trusted network (no dashboard auth); stateful (DB-backed)
**Scale/Scope**: Single Signal account, personal use (~hundreds of messages/day)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Message Integrity | ✅ PASS | `signal_id TEXT UNIQUE` in migration 000001; persist-first flow required in `internal/signal/listener.go` |
| II. Schema-Versioned Storage | ✅ PASS (with action) | golang-migrate + embedded FS in place; migration 000002 required for `triage_status`, `feedback_memory` HNSW index, and `replies` table before serve command is implemented |
| III. Structured LLM Contracts | ✅ PASS (design) | Schema defines output shape; strict `json.Unmarshal` validation with `triage_status=failed` on error must be implemented in `internal/triage/analyzer.go` |
| IV. Simplicity and YAGNI | ✅ PASS | No ORM; stdlib HTTP mux; Templ is an explicit tech constraint exception; Cobra+Viper already in place |
| V. Observable Triage Pipeline | ✅ PASS (with action) | rs/zerolog in go.sum; must be promoted to direct dep; every pipeline stage must log `stage`, `signal_id`, `duration_ms` |

**Post-design re-check**: All gates pass. Migration 000002 and zerolog promotion are
blocking tasks for Phase 2 (Foundational).

## Project Structure

### Documentation (this feature)

```text
specs/001-signal-ai-triage/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/
│   ├── http-api.md      # HTTP endpoint contracts
│   └── websocket.md     # WebSocket protocol (dashboard + Signal bridge)
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
cmd/
├── root.go              # Cobra root, Viper config (exists)
├── migrate.go           # triage migrate command (exists)
└── serve.go             # triage serve command (new)

internal/
├── store/
│   ├── migrate.go       # Migration runner (exists)
│   ├── db.go            # pgx connection pool + pgvector setup (new)
│   ├── messages.go      # Message CRUD + embedding upsert (new)
│   ├── feedback.go      # FeedbackMemory insert + similarity recall (new)
│   ├── replies.go       # Reply CRUD + delivery status update (new)
│   └── migrations/
│       ├── 000001_init_schema.up.sql    (exists)
│       └── 000002_add_triage_status.up.sql  (new)
├── signal/
│   ├── listener.go      # WebSocket client → Signal bridge, reconnect loop (new)
│   └── sender.go        # REST POST /v2/send to Signal bridge (new)
├── triage/
│   └── analyzer.go      # OpenAI triage + embedding calls, JSON validation (new)
└── web/
    ├── server.go         # net/http ServeMux, route registration (new)
    ├── hub.go            # WebSocket broadcast hub (new)
    ├── handlers.go       # HTTP handlers: dashboard, feedback, reply (new)
    └── templates/
        ├── layout.templ          # Base HTML layout (new)
        ├── message_card.templ    # Single message card component (new)
        └── dashboard.templ       # Full dashboard page (new)
```

**Structure Decision**: Standard Go project layout (`cmd/` + `internal/`). Single
deployable binary. All packages are flat within their domain. No sub-packages until
a second concrete sub-domain emerges (YAGNI / Principle IV).

## Complexity Tracking

> No constitution violations requiring justification.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| Templ code generation | Explicit technology constraint; Go Templ requires `templ generate` build step | Not a complexity exception — mandated by constitution tech constraints |
