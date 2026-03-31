# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Is

Signal AI Triage Engine — a Go backend that ingests Signal messages via WebSocket, scores them with an LLM (0–100 priority), stores results in PostgreSQL + pgvector, and serves a real-time web dashboard via Go Templ + htmx.

## Commands

```bash
# Build
go build -o triage

# Run database migrations
./triage migrate

# Start the server
./triage serve

# Infrastructure (PostgreSQL + pgvector + signal-cli-rest-api)
docker-compose up -d

# Run tests
go test ./...

# Run a single test
go test ./internal/store/... -run TestMigrations

# Fix dirty migration state
migrate -path internal/store/migrations -database "$TRIAGE_DB_URL" force <VERSION_NUMBER>
```

## Configuration

All config is via environment variables with `TRIAGE_` prefix (loaded by Viper). Create a `.env` file:

```dotenv
TRIAGE_DB_URL="postgres://admin:password123@localhost:5432/triage_store?sslmode=disable"
TRIAGE_SIGNAL_URL="ws://localhost:8080/v1/receive/+1234567890"
TRIAGE_LLM_KEY="your-api-key"
```

## Architecture

**Data flow:**
1. `signal-cli-rest-api` container (Docker) → WebSocket → Go backend (`internal/signal/`)
2. Backend queries pgvector for semantically similar past feedback (`internal/store/`)
3. Message + feedback → LLM → structured JSON triage (`internal/triage/`)
4. Result saved to Postgres; broadcast to web UI via htmx WebSocket (`internal/web/`)
5. User feedback generates embeddings stored in `feedback_memory` table for future few-shot context

**Key packages:**
- `cmd/` — Cobra CLI commands; config initialized via Viper with `TRIAGE_` env prefix
- `internal/store/` — Postgres + pgvector queries; migrations embedded via `embed.FS` (iofs driver)
- `internal/store/migrations/` — SQL migration files; embeddings are 768-dimensional (Gemini 1.5 default; change to 1536 for OpenAI text-embedding-3-small)
- `internal/signal/` — Signal WebSocket client and REST API integration
- `internal/triage/` — LLM prompt engineering, structured JSON triage response parsing
- `internal/web/` — Templ components and htmx handlers (SSR, no SPA)

**Database schema:**
- `messages`: id, signal_id, sender_phone, content, category, priority, reasoning, group_id, embedding (vector(768)), created_at
- `feedback_memory`: id, original_message_id (FK → messages), feedback_text, embedding (vector(768)), created_at
- Both tables have HNSW vector indexes for fast similarity search

**Migrations** are embedded in the binary via `//go:embed migrations/*.sql` in `internal/store/migrate.go`. Add new migrations as `000002_*.up.sql` (and optionally `*.down.sql`).

## Troubleshooting

- **Dirty DB version:** Migration interrupted → run `migrate force <VERSION>` (see Commands above)
- **Vector dimension mismatch:** Embedding size in migration must match LLM output (768 vs 1536)
- **Signal timeout:** Set `SIGNAL_CLI_CMD_TIMEOUT=120` in docker-compose.yml
- **htmx WebSocket drops:** Don't place `ws-connect` on elements that get swapped by other htmx actions

## Active Technologies
- Go 1.26 (module `github.com/mshindle/triage`) + cobra v1.10.2, viper v1.21.0, golang-migrate v4, openai-go v3, (001-signal-ai-triage)
- PostgreSQL 17 + pgvector (768-dim embeddings, HNSW indexes) (001-signal-ai-triage)
- Go 1.26 + Echo v4 (HTTP), coder/websocket v1.8.14, templ v0.3.1001, htmx 2.0 (CDN), Uber Fx v1.24 (existing DI), zerolog v1.34 (002-signal-triage-ui)
- PostgreSQL 17 + pgvector; no new tables — `messages`, `feedback_memory`, `replies` cover all data needs; one new column via migration 000002 (002-signal-triage-ui)

## Recent Changes
- 001-signal-ai-triage: Added Go 1.26 (module `github.com/mshindle/triage`) + cobra v1.10.2, viper v1.21.0, golang-migrate v4, openai-go v3,
