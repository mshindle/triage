<!--
SYNC IMPACT REPORT
==================
Version change: (none) → 1.0.0 (initial ratification)

Modified principles:
  - N/A (initial ratification — no prior principles)

Added sections:
  - Core Principles (I–V)
  - Technology Constraints
  - Development Workflow
  - Governance

Removed sections:
  - N/A

Template alignment:
  - .specify/templates/plan-template.md       ✅ Compatible — Constitution Check section present; no updates needed
  - .specify/templates/spec-template.md       ✅ Compatible — user story and requirements structure aligns with principles
  - .specify/templates/tasks-template.md      ✅ Compatible — phase/story structure aligns with incremental delivery principle
  - .specify/templates/constitution-template.md ✅ Source template (read-only reference)

Deferred TODOs:
  - None
-->

# Signal AI Triage Engine Constitution

## Core Principles

### I. Message Integrity (NON-NEGOTIABLE)

Every Signal message received by the WebSocket listener MUST be persisted or produce an
explicit, logged error. Silent drops are forbidden. The `messages` table is the system of
record; no triage or feedback step may proceed on a message not yet committed to storage.
Idempotency key: `signal_id` MUST be unique-constrained to prevent duplicate ingestion.

**Rationale**: Loss of a high-priority message is a silent, undetectable failure. The entire
value of the system depends on complete message capture.

### II. Schema-Versioned Storage

All changes to the PostgreSQL schema MUST be made via sequentially numbered `golang-migrate`
files in `internal/store/migrations/`. Direct DDL against a running database is forbidden.
Migration files MUST be embedded in the binary via `//go:embed`. The embedding vector
dimension declared in migrations MUST match the model configured at runtime
(768 for Gemini 1.5, 1536 for OpenAI text-embedding-3-small); mismatches MUST be detected
at startup, not silently truncated.

**Rationale**: Drift between code and schema has caused production incidents. Embedded
migrations ensure the deployed binary is always self-sufficient and auditable.

### III. Structured LLM Contracts

The LLM triage step MUST return validated, structured JSON containing at minimum:
`priority` (integer 0–100), `category` (string), and `reasoning` (string). Any response
that fails JSON parse or schema validation MUST be rejected and logged — the message MUST
NOT appear in the UI with a phantom or zero priority. Prompt changes that affect output
structure MUST be accompanied by updated validation logic.

**Rationale**: Unstructured or partially-parsed LLM output silently corrupts the triage
queue. The `reasoning` field is mandatory because it is the primary audit trail for why
a priority was assigned.

### IV. Simplicity and YAGNI

New abstractions (interfaces, wrappers, helpers) MUST have at least two distinct call sites
before being introduced. Packages MUST remain flat unless a second concrete sub-domain
emerges. Dependency injection frameworks, ORMs, and generated code are forbidden unless
the equivalent hand-written code exceeds 300 lines of true business logic. Configuration
MUST flow through Viper with the `TRIAGE_` prefix; no secondary config mechanism may
be added without removing an existing one.

**Rationale**: This is a single-operator tool. Premature abstraction increases surface area
without delivering user value. Prior templates defaulted to over-engineered structures that
slowed iteration.

### V. Observable Triage Pipeline

Every stage of the ingest → recall → analyze → persist → broadcast pipeline MUST emit
a structured log line (JSON or key=value) containing at minimum: `stage`, `signal_id`,
and `duration_ms`. LLM call latency and embedding query latency MUST be logged separately.
Errors MUST include the originating `signal_id` so any message can be traced end-to-end
from the log stream without a database query.

**Rationale**: The pipeline crosses multiple I/O boundaries (WebSocket, pgvector, LLM API).
Without per-stage observability, diagnosing latency spikes or silent failures requires
guesswork.

## Technology Constraints

- **Runtime**: Go 1.24+ (module path `github.com/mshindle/triage`)
- **CLI/Config**: Cobra for commands; Viper for all configuration with `TRIAGE_` env prefix
- **Database**: PostgreSQL 17 + pgvector extension; no other storage backends permitted
- **Migrations**: `golang-migrate` v4 with `iofs` source driver; files embedded in binary
- **Frontend**: Go Templ (type-safe SSR components) + htmx; no client-side JS frameworks
- **Signal bridge**: `bbernhard/signal-cli-rest-api` via Docker Compose; not vendored into Go
- **External services**: Docker Compose (`docker-compose up -d`) manages all infrastructure;
  the Go binary connects to them but does not manage their lifecycle

## Development Workflow

- **Feature branches**: Created via `speckit` tooling; named `###-feature-slug` with
  sequential numbering
- **Planning gate**: A `plan.md` with a completed Constitution Check MUST exist before
  implementation tasks begin
- **Incremental delivery**: Features MUST be decomposed into independently testable user
  stories; Phase 2 (Foundational) MUST be complete before any user story work begins
- **Migration policy**: Each schema change ships in its own migration file; never modify
  an existing migration that has been committed to `main`
- **Dirty DB recovery**: Use `migrate force <VERSION>` — never delete migration files to
  resolve dirty state

## Governance

This constitution supersedes all other written or verbal development practices for this
repository. Amendments require:

1. A documented rationale referencing a concrete incident or validated requirement
2. Version bump per semantic versioning rules (MAJOR/MINOR/PATCH defined in preamble)
3. Propagation to all dependent `.specify/templates/` files before merge

All PRs that touch `internal/store/migrations/`, `internal/triage/`, or `internal/signal/`
MUST verify compliance with Principles I, II, and III respectively. Complexity exceptions
MUST be documented in the plan's Complexity Tracking table.

**Version**: 1.0.0 | **Ratified**: 2026-03-25 | **Last Amended**: 2026-03-25
