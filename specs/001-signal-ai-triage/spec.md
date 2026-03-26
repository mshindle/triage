# Feature Specification: Signal AI Triage Engine

**Feature Branch**: `001-signal-ai-triage`
**Created**: 2026-03-25
**Status**: Draft
**Input**: User description: "Build a Signal AI Triage Engine that acts as an intelligent middleware for Signal messages."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Triage View (Priority: P1)

The operator opens the web dashboard and immediately sees all received Signal messages
displayed in a live-updating stream. Each message is color-coded to reflect its
AI-assigned priority: high-priority messages appear prominently, while low-priority
messages are visually de-emphasized. The stream updates in real time as new messages
arrive, without requiring a page refresh.

**Why this priority**: This is the core value proposition of the entire system. Without a
working triage view, no other feature has context. It is the MVP.

**Independent Test**: Can be fully tested by opening the dashboard while messages arrive
via Signal and verifying that messages appear, are color-coded by priority score, and the
stream updates without a page reload.

**Acceptance Scenarios**:

1. **Given** the dashboard is open, **When** a new Signal message arrives, **Then** the
   message appears in the stream within 3 seconds with a priority score (0–100), a
   category label, and a reasoning summary.
2. **Given** multiple messages from the same Signal Group, **When** displayed on the
   dashboard, **Then** they are grouped together and ordered chronologically within
   their group.
3. **Given** the dashboard has loaded, **When** the user scrolls through the stream,
   **Then** messages are color-coded by tier: High (70–100), Medium (40–69), Low (0–39).

---

### User Story 2 - Semantic Feedback Loop (Priority: P2)

The operator reviews a message the AI has mis-classified. They click a priority
adjustment control (e.g., "Mark High Priority" or "Mark Low Priority") directly on the
message card. The system saves this correction and, on future messages with similar
content or context, applies the user's stated preference—effectively teaching the AI
the operator's personal priorities over time.

**Why this priority**: Without this, the system degrades in usefulness and becomes a
static classifier. The feedback loop is the core differentiator that makes the system
personalized and self-improving.

**Independent Test**: Can be fully tested by submitting a correction on a message, then
receiving a new message with semantically similar content, and verifying the new
message's priority reflects the correction.

**Acceptance Scenarios**:

1. **Given** a message is displayed with a priority score, **When** the user clicks
   "Mark High Priority," **Then** the message's priority is immediately updated on screen
   and the correction is persisted.
2. **Given** a correction has been saved, **When** a semantically similar message arrives
   (same sender, similar topic, similar intent), **Then** the AI's priority assignment
   reflects the user's stated preference rather than the unadjusted baseline.
3. **Given** the user has made multiple corrections over time, **When** new messages
   arrive, **Then** the AI's accuracy on messages similar to previously corrected ones
   improves measurably compared to the initial session.

---

### User Story 3 - Direct Reply from Dashboard (Priority: P3)

The operator reads a high-priority message on the dashboard and wants to respond without
switching to their phone. They type a reply in a text input within the message card and
submit it. The reply is delivered to the original sender (or Signal Group) via the Signal
network as if sent from the operator's registered Signal account.

**Why this priority**: Reduces context switching for the operator. Lower priority because
the triage view and feedback loop deliver standalone value even without reply capability.

**Independent Test**: Can be fully tested by sending a reply from the dashboard and
verifying the recipient receives it via Signal.

**Acceptance Scenarios**:

1. **Given** a message is displayed, **When** the user types a reply and submits,
   **Then** the reply is delivered to the original sender on Signal within 5 seconds.
2. **Given** a message from a Signal Group, **When** the user replies from the dashboard,
   **Then** the reply is delivered to the entire group thread, not as a private message
   to the individual sender.
3. **Given** the Signal bridge is temporarily unavailable, **When** the user attempts to
   send a reply, **Then** an error is surfaced in the dashboard and the reply is not
   silently dropped.

---

### Edge Cases

- What happens when a message arrives but AI triage fails (service unavailable or
  returns invalid output)? The message MUST still be persisted and displayed, with an
  explicit "Triage Failed" indicator rather than a phantom priority score.
- How does the system handle duplicate message delivery (e.g., reconnecting after a
  dropped connection)? Messages with identical identifiers MUST NOT appear more than
  once in the dashboard.
- What if the operator submits a reply to a contact whose number is invalid or who
  has blocked the account? An error MUST be surfaced; the failure MUST be logged.
- What happens to messages received while the dashboard is closed? They MUST be
  persisted and visible when the dashboard is next opened.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST ingest all incoming Signal messages for the registered
  account in real time via the Signal bridge.
- **FR-002**: Every ingested message MUST be persisted before any triage or display step
  occurs. No message may be silently dropped.
- **FR-003**: The system MUST assign each message a priority score (integer 0–100), a
  category label, and a plain-language reasoning summary using an AI model.
- **FR-004**: The dashboard MUST display messages in a live-updating stream without
  requiring a page refresh.
- **FR-005**: Messages from the same Signal Group MUST be displayed grouped together and
  ordered chronologically within their group.
- **FR-006**: Messages MUST be visually differentiated by priority tier: High (70–100),
  Medium (40–69), Low (0–39).
- **FR-007**: The user MUST be able to manually override a message's priority from the
  dashboard (e.g., "Mark High Priority," "Mark Low Priority").
- **FR-008**: Each priority override MUST be persisted as a correction linked to the
  original message.
- **FR-009**: The system MUST use persisted corrections to semantically influence AI
  priority assignments on future messages with similar content or context.
- **FR-010**: The user MUST be able to compose and send a reply to any message directly
  from the dashboard.
- **FR-011**: Replies sent from the dashboard MUST be delivered via the Signal bridge
  using the operator's registered Signal account.
- **FR-012**: Group messages replied to from the dashboard MUST be delivered to the
  group thread, not as a private message to the individual sender.
- **FR-013**: Any AI triage failure MUST be surfaced to the user with an explicit
  indicator; the message MUST still be visible on the dashboard.
- **FR-014**: Any reply delivery failure MUST be surfaced to the user; failures MUST NOT
  be silently discarded.

### Key Entities

- **Message**: Represents a received Signal communication. Attributes: unique identifier,
  sender identity, text content, group membership (if any), timestamp, AI-assigned
  priority score, AI-assigned category, AI reasoning, triage status.
- **Correction**: Represents a user-submitted priority override for a specific message.
  Attributes: reference to original message, user-specified priority direction,
  semantic representation of the original message content, timestamp.
- **Reply**: Represents an outbound message composed from the dashboard. Attributes:
  reference to original message, reply content, delivery status, timestamp.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: New messages appear on the dashboard within 3 seconds of being received
  by the Signal bridge under normal network conditions.
- **SC-002**: 100% of received messages are persisted; zero messages are silently
  dropped, even when AI triage fails.
- **SC-003**: After 10 or more corrections have been submitted, the AI's priority
  assignments on similar subsequent messages match the user's stated preferences at
  least 80% of the time.
- **SC-004**: Replies composed in the dashboard are delivered to Signal recipients within
  5 seconds of submission under normal network conditions.
- **SC-005**: The dashboard remains functional (messages display, corrections can be
  submitted) even when the AI triage service is temporarily unavailable.
- **SC-006**: Messages within the same Signal Group are always presented grouped and in
  chronological order, with no ordering regressions across sessions.

## Assumptions

- The system is a single-operator personal tool; no multi-user access control or web
  dashboard authentication is required (the service is assumed to run in a trusted
  local or private network environment).
- The Signal account is pre-registered and linked to the Signal bridge before the
  system starts; account registration is out of scope.
- The AI model used for triage is accessible and configured via environment variables;
  model selection and provisioning are out of scope.
- The priority category taxonomy (e.g., Personal, Work, Urgent, Spam, Informational)
  is determined by the AI based on message content; no fixed taxonomy is required for
  the initial version.
- Messages received while the service is offline are not retroactively ingested;
  only messages received while the service is running are triaged.
- The system handles text-based Signal messages; media attachments (images, files,
  voice notes) are stored by reference but triage is based on available text content
  and message metadata only.
