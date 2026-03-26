# Quickstart: Signal AI Triage Engine

**Branch**: `001-signal-ai-triage` | **Date**: 2026-03-25

## Prerequisites

- Docker & Docker Compose
- Go 1.24+
- `templ` CLI: `go install github.com/a-h/templ/cmd/templ@latest`
- A Signal account (phone number) registered with signal-cli
- An OpenAI API key

## 1. Configure Environment

Create `.env` in the repository root:

```dotenv
TRIAGE_DB_URL=postgres://admin:password123@localhost:5432/triage_store?sslmode=disable
TRIAGE_SIGNAL_URL=ws://localhost:8080/v1/receive/+1YOUR_PHONE
TRIAGE_LLM_KEY=sk-...your-openai-key...
TRIAGE_LLM_MODEL=gpt-4o-mini
TRIAGE_EMBED_MODEL=text-embedding-3-small
TRIAGE_EMBED_DIMS=768
TRIAGE_FEEDBACK_K=5
TRIAGE_LISTEN_ADDR=:8081
```

## 2. Start Infrastructure

```bash
docker-compose up -d
```

Verify:
```bash
docker-compose ps   # signal-api and db should be "Up"
curl http://localhost:8080/v1/about   # signal-cli-rest-api health check
```

## 3. Link Signal Account

If not already linked:
```bash
# Open a browser to http://localhost:8080/v1/qrcodelink?device_name=triage
# Scan the QR code with your Signal app (Linked Devices → Link New Device)
```

## 4. Build

```bash
go generate ./...    # runs templ generate on all .templ files
go build -o triage .
```

## 5. Run Migrations

```bash
./triage migrate
```

Expected output: `migrations applied` (or `no change` if already up-to-date).

## 6. Start the Server

```bash
./triage serve
```

Expected output:
```
{"level":"info","addr":":8081","msg":"server started"}
{"level":"info","signal_url":"ws://localhost:8080/v1/receive/+1...","msg":"signal listener connected"}
```

## 7. Open the Dashboard

Navigate to `http://localhost:8081` in a browser.

Send a Signal message to your linked number. Within ~3 seconds it should appear
in the dashboard with a priority score, category, and AI reasoning.

## 8. Test the Feedback Loop

1. Find a message in the dashboard you want to re-prioritize.
2. Click "Mark High Priority" or "Mark Low Priority."
3. The priority badge updates immediately.
4. Send a follow-up message with similar content — the AI should assign a priority
   consistent with your correction.

## 9. Test Reply

1. Click the reply input on any message card.
2. Type a reply and press Enter or click Send.
3. The delivery status badge changes to "Delivered" on success.
4. Verify the reply was received on your Signal phone.

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| `signal listener: websocket: bad handshake` | Ensure signal-cli-rest-api is running and `TRIAGE_SIGNAL_URL` phone matches registered number |
| `dirty database version` | Run `migrate -path internal/store/migrations -database "$TRIAGE_DB_URL" force <N>` |
| `vector dimension mismatch` | Ensure `TRIAGE_EMBED_DIMS` matches the vector column size in migrations |
| `triage failed` badge in dashboard | Check logs for LLM error; verify `TRIAGE_LLM_KEY` is valid |
| Reply shows "Failed" | Check logs for signal bridge error; verify phone is in E.164 format |
