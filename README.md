# Signal AI Triage Engine

An intelligent, Go-powered messaging middleware that parses, categorizes, and prioritizes incoming Signal messages. This system mirrors your Signal data and provides a real-time web dashboard with a semantic feedback loop to "teach" the AI your personal priority preferences.

## 🚀 Overview

The system bridges the gap between end-to-end encrypted messaging and automated triage. It links to your Signal account, applies LLM-based reasoning to determine urgency, and provides a web interface for manual override and responding to messages directly.

### Key Features
* **Real-time Ingestion:** Persistent WebSocket connection to Signal via `signal-cli-rest-api`.
* **AI Prioritization:** Automatic scoring ($0-100$) and categorization using LLMs.
* **Semantic Feedback Loop:** Uses **pgvector** to store and retrieve your manual priority corrections, providing few-shot context to the AI for future messages.
* **Modern Web UI:** High-performance SSR (Server-Side Rendering) using **Go Templ** and **htmx** for a reactive, "no-SPA" experience.
* **Direct Response:** Reply to any Signal message or thread directly from the web dashboard.
* **Group Consistency:** Maintains chronological order and channel-specific context (`group_id` tracking).

---

## 🛠 Tech Stack

* **Backend:** Golang 1.24+
* **CLI & Config:** Cobra (Commands) & Viper (Environment/Config)
* **Database:** PostgreSQL 17 + `pgvector` (Relational + Vector storage)
* **Migrations:** `golang-migrate` (Embedded in binary via `iofs`)
* **Signal Bridge:** `signal-cli-rest-api` (Dockerized)
* **Frontend:** Templ (Type-safe components) + htmx (Live updates)

---

## 🏗 Data Flow

1.  **Ingest:** `signal-api` container receives an E2EE message and pushes it via WebSocket to the Go backend.
2.  **Recall:** The Go backend queries `pgvector` for past user feedback semantically similar to the new message.
3.  **Analyze:** The message + past feedback are sent to the LLM. The AI returns a structured JSON triage (Priority, Category, Reasoning).
4.  **Persist:** The message and triage result are saved to Postgres.
5.  **Broadcast:** The new message is pushed to the Web UI using `htmx` WebSockets.
6.  **Feedback:** User clicks "Lower Priority" on the web app. The backend generates an embedding for that correction and saves it to the vector store to inform future analysis.

---

## 🚦 Getting Started

### Prerequisites
* Docker & Docker Compose
* Go 1.24+
* Signal Account (Phone number or linked device)

### 1. Infrastructure Setup
Spin up the Signal bridge and the vector-enabled database:
```bash
docker-compose up -d
```
### 2. Configuration
Create a triage.yaml file in the root directory:
```yaml
database:
  url: "postgres://admin:password123@localhost:5432/triage_store?sslmode=disable"
llm:
  key: "your-api-key"
log:
  level: "debug"
  console: true
signal:
  url: "ws://localhost:8080/v1/receive/+1234567890"
```

### 3. Build and Run
First, generate the UI components, then build the binary:
```bash
# Install templ if you don't have it
go install github.com/a-h/templ/cmd/templ@latest

# Generate Go code from .templ files
go generate ./...

# Build the binary
go build -o triage main.go

# Run migrations and start the server
./triage migrate
./triage serve
```

## 📂 Project Structure
```text
├── cmd/                # Cobra command definitions
├── internal/
│   ├── signal/         # Signal WebSocket and REST API client
│   ├── triage/         # LLM logic and prompt engineering
│   ├── store/          # Postgres/pgvector logic & migrations
│   │   └── migrations/ # .sql migration files (embedded)
│   └── web/            # Templ components and htmx handlers
├── deployments/        # Docker and infrastructure configs
├── main.go             # Entry point
└── README.md
```

## 🛠 Troubleshooting
### Signal Connectivity
 * **Timeout Errors:** If you see process killed as timeout reached, increase the Signal timeout in docker-compose.yml by setting SIGNAL_CLI_CMD_TIMEOUT=120.
 * **QR Code Not Appearing:** Ensure the signal_storage volume is correctly mounted. If linking fails, try switching the MODE from json-rpc to native temporarily to complete the link.

### Database & Migrations
 * **Dirty Database Version:** If a migration is interrupted, golang-migrate will mark the DB as "dirty." Fix this by running:
`migrate -path internal/store/migrations -database "YOUR_DB_URL" force <VERSION_NUMBER>`
 * **Vector Dimension Mismatch:** Ensure the embedding vector(N) size in your migration matches your LLM's output (e.g., 768 for Gemini 1.5, 1536 for OpenAI text-embedding-3-small).
 * **Extension Missing:** If you get type "vector" does not exist, ensure the first migration includes CREATE EXTENSION IF NOT EXISTS vector;.

### Web UI (htmx)
 * **WebSocket Not Connecting:** Ensure ws-connect is placed on a top-level element that is not swapped out by other htmx actions. Swapping the element holding the connection will drop the WebSocket.
 * **Broken Responses:** Check the Go logs. If the LLM returns invalid JSON, the triage step will fail and the message won't appear in the UI.
