# Quickstart: Signal AI Triage Dashboard UI

**Branch**: `002-signal-triage-ui` | **Date**: 2026-03-30

---

## Prerequisites

- Docker + Docker Compose running (`docker-compose up -d`)
- `.env` file in project root with `TRIAGE_DB_URL`, `TRIAGE_LLM_KEY`, `TRIAGE_SIGNAL_URL`
- Go 1.26 installed
- `templ` CLI installed: `go install github.com/a-h/templ/cmd/templ@latest`

---

## Run Migration 000002

Before starting the server with the new UI, apply the schema change:

```bash
./triage migrate
```

Verify:
```bash
psql "$TRIAGE_DB_URL" -c "\d feedback_memory"
# Should show adjusted_category column
```

---

## Regenerate Templ Components

After editing any `.templ` file, regenerate the Go code:

```bash
templ generate
```

Or run in watch mode during development:

```bash
templ generate --watch
```

---

## Build and Run

```bash
go build -o triage && ./triage serve
```

Open `http://localhost:8081` (or the address set in `TRIAGE_WEB_LISTENADDR`).

---

## Development Workflow

1. Edit `.templ` files in `internal/web/templates/`
2. Run `templ generate` (or keep `--watch` running)
3. Rebuild: `go build -o triage && ./triage serve`
4. Hard-refresh browser to pick up new Tailwind classes (CDN rebuild not needed)

---

## Testing

```bash
# All tests
go test ./...

# Web handlers only
go test ./internal/web/...

# Store queries only (requires running DB)
go test ./internal/store/...
```

---

## Dirty Migration Recovery

If a migration fails partway through:

```bash
migrate -path internal/store/migrations -database "$TRIAGE_DB_URL" force 2
./triage migrate
```

---

## Key Files for This Feature

| File | Purpose |
|------|---------|
| `internal/web/templates/dashboard.templ` | Three-panel shell layout |
| `internal/web/templates/conversation_list.templ` | Left panel |
| `internal/web/templates/message_thread.templ` | Center panel |
| `internal/web/templates/triage_detail.templ` | Right panel |
| `internal/web/handlers.go` | All HTTP handlers |
| `internal/store/conversations.go` | New store queries |
| `internal/store/migrations/000002_add_feedback_category.up.sql` | Schema change |
