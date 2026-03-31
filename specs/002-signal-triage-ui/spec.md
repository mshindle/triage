# Feature Specification: Signal AI Triage Dashboard UI

**Feature Branch**: `002-signal-triage-ui`
**Created**: 2026-03-30
**Status**: Draft
**Input**: User description: "Signal AI Triage Dashboard — three-panel Signal-inspired interface for triaging, categorizing, and prioritizing incoming Signal messages with real-time updates and user feedback"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Real-Time Message Monitoring (Priority: P1)

An operator opens the dashboard and immediately sees all incoming Signal messages organized by sender and group, each displaying an AI-assigned priority level and category. New messages appear in real time without requiring a page refresh. High-priority messages are visually distinct so the operator can act on them immediately.

**Why this priority**: This is the core value of the system — if operators cannot see incoming messages with their triage results in real time, the entire product fails. Everything else builds on this foundation.

**Independent Test**: Can be fully tested by loading the dashboard and observing that messages appear, show priority indicators (high/medium/low), and update automatically when new messages arrive — without any feedback or reply functionality required.

**Acceptance Scenarios**:

1. **Given** the dashboard is open and connected, **When** a new Signal message is received and triaged, **Then** it appears in the conversation list within seconds with its priority level, category, and sender identity visible.
2. **Given** the operator is viewing one conversation, **When** a new message arrives in a different conversation, **Then** that conversation moves to the top of the list with an unread indicator and the operator is notified without losing their current view.
3. **Given** the connection to the backend is lost, **When** the operator looks at the header, **Then** a clear disconnected indicator is shown and the system attempts to reconnect automatically.
4. **Given** the dashboard has multiple conversations, **When** the operator selects a conversation, **Then** the full message thread is shown with each message's priority, category, and triage status visible.

---

### User Story 2 - AI Triage Feedback and Correction (Priority: P2)

An operator reviews the AI's triage decision for a specific message and disagrees with the assigned priority or category. They provide a correction — adjusting the priority score, overriding the category, and optionally adding a free-text explanation. The correction is immediately reflected in the UI and stored for future AI improvement.

**Why this priority**: The feedback loop is what makes the system improve over time. Without correction capability, operators are stuck with AI errors and cannot influence future accuracy. This directly affects operational effectiveness.

**Independent Test**: Can be fully tested by selecting a message, viewing the triage detail panel, submitting a priority and/or category correction, and confirming the updated values appear in the UI — without needing reply functionality.

**Acceptance Scenarios**:

1. **Given** a message is selected, **When** the operator views the triage detail panel, **Then** they see the AI-assigned priority score (0–100), category, and reasoning displayed clearly.
2. **Given** the triage detail panel is open, **When** the operator adjusts the priority and submits feedback, **Then** the message's priority indicator updates immediately in both the detail panel and the conversation list.
3. **Given** the operator has submitted feedback, **When** the same message is viewed again, **Then** the feedback history shows all past corrections with their timestamps.
4. **Given** the triage detail panel is open, **When** the operator overrides the category and adds explanatory text before submitting, **Then** all three pieces of feedback are saved and reflected in the UI.

---

### User Story 3 - Reply to Message Sender (Priority: P3)

An operator reads an incoming message, composes a reply, and sends it directly from the dashboard. The reply appears in the message thread as an outgoing message, and the operator can see whether the reply was delivered successfully.

**Why this priority**: Reply capability is important for operational completeness but is not required for triage and monitoring — the P1 and P2 stories deliver standalone value. Reply builds on the existing thread view.

**Independent Test**: Can be tested by selecting a conversation, typing a reply in the composer, sending it, and confirming the reply bubble appears in the thread with a delivery status indicator.

**Acceptance Scenarios**:

1. **Given** a conversation is selected, **When** the operator types a reply and submits it, **Then** the reply appears in the thread as an outgoing message with a "pending" delivery indicator.
2. **Given** a reply was sent, **When** delivery is confirmed by the backend, **Then** the delivery indicator updates to "sent."
3. **Given** a reply fails to deliver, **When** the backend reports the failure, **Then** the reply bubble shows a failed indicator and the operator can see the error reason.

---

### User Story 4 - Filtered Conversation View (Priority: P4)

An operator wants to focus only on high-priority messages or a specific category. They use the filter controls in the header to narrow the conversation list to only matching items, without losing their current selection or needing to reload the page.

**Why this priority**: Filtering improves efficiency for operators managing high volumes, but the dashboard is still functional without it. It is an enhancement to the core monitoring workflow.

**Independent Test**: Can be tested by applying a priority filter and confirming that only matching conversations appear in the list, and that clearing the filter restores the full list.

**Acceptance Scenarios**:

1. **Given** the operator activates a "High Priority" filter, **When** the filter is applied, **Then** only conversations with at least one high-priority message are shown in the list.
2. **Given** multiple filters are active (e.g., priority + category), **When** viewing the list, **Then** only conversations matching all active filters are shown, and an active filter count is displayed.
3. **Given** filters are active, **When** the operator clicks "Clear all," **Then** all filters are removed and the full conversation list is restored.
4. **Given** a filter is active and a new message arrives that does not match the filter, **When** the message is received, **Then** it does not appear in the filtered list until the filter is cleared.

---

### Edge Cases

- What happens when the real-time connection drops mid-session? The operator should see a clear disconnected state and automatic reconnection should be attempted.
- What happens if a message arrives while triage is still in progress (not yet scored)? The message appears in the conversation list with a visible "pending triage" state. The operator can open the detail panel to view the message content and sender, but all feedback controls are disabled until triage completes.
- What happens if the operator submits feedback while offline? The submission should fail gracefully with a user-visible error; no silent data loss.
- How does the conversation list behave when there are hundreds of conversations? The dashboard must remain responsive with up to 500 conversations in the list and up to 1,000 messages in a single thread, without pagination being required.
- What happens when a reply fails to deliver to the recipient? The delivery failure should be surfaced clearly in the thread with the reason where available.
- How are group messages distinguished from direct messages in the same list? Each conversation item must clearly indicate whether it is a direct sender or a group.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The dashboard MUST display all incoming Signal messages organized by sender and group in a conversation list with visual priority indicators (high, medium, low).
- **FR-002**: The dashboard MUST update the conversation list and message threads in real time when new messages arrive, without requiring a page reload.
- **FR-003**: Users MUST be able to select a conversation to view the full message thread for that sender or group.
- **FR-004**: Users MUST be able to select an individual message within a thread to view its full triage detail, including priority score, category, and AI reasoning.
- **FR-005**: Users MUST be able to submit feedback on a message's triage result, including priority adjustment, category override, and free-text explanation. Feedback controls MUST be disabled for messages whose triage is still pending.
- **FR-006**: The dashboard MUST persist feedback and display a history of all corrections for each message.
- **FR-007**: Users MUST be able to compose and send a reply to the sender of a selected message.
- **FR-008**: The dashboard MUST display reply delivery status (pending, sent, failed) and surface failure reasons for each outgoing reply.
- **FR-009**: Users MUST be able to filter the conversation list by priority level, category, and triage status.
- **FR-010**: The dashboard MUST display a real-time connection status indicator showing whether the live update connection is active or disconnected.
- **FR-011**: The dashboard MUST be usable across desktop, tablet, and mobile screen sizes, adapting its layout for each.
- **FR-012**: All interactive elements MUST be keyboard-navigable, and color-coding MUST always be paired with a text or icon indicator so the UI is usable without color perception.
- **FR-013**: The conversation list MUST support sorting by recency, priority (high to low), and unread status.
- **FR-014**: The dashboard MUST show unread message counts per conversation so operators can distinguish read from unread items at a glance.
- **FR-015**: Priority information MUST meet a minimum color contrast ratio of 4.5:1 for all displayed text.

### Key Entities

- **Conversation**: A grouping of messages from a single sender phone number or Signal group. Carries identity (sender or group name), unread count, highest-priority message indicator, and a preview of the most recent message.
- **Message**: An individual Signal message with its content, sender identity, timestamp, AI-assigned priority score (0–100), category label, triage status, and AI reasoning. May be incoming (from a contact) or outgoing (an operator reply).
- **Triage Result**: The AI's assessment attached to an incoming message — includes a numeric priority score, a category label, a reasoning explanation, and a status (triaged or failed). When a new triage result arrives for a message, it replaces the previous result; only the latest assessment is displayed.
- **Feedback**: An operator correction to a triage result — records the adjusted priority, corrected category, free-text explanation, and timestamp. Multiple feedback entries may exist per message.
- **Reply**: An outgoing message sent by the operator in response to an incoming message. Tracks content, delivery status, and any error details.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Operators can identify all high-priority messages within 10 seconds of opening the dashboard, without scrolling past low-priority items, at a scale of up to 500 concurrent conversations.
- **SC-002**: New messages and triage updates appear in the dashboard within 3 seconds of being processed by the backend.
- **SC-003**: Operators can complete a triage feedback submission (priority + category + text) in under 60 seconds from reading the message.
- **SC-004**: The dashboard is fully functional on screen sizes from 320px wide (mobile) through standard desktop widths, with no loss of core functionality on any supported size.
- **SC-005**: All primary operator actions (view thread, submit feedback, send reply, apply filter) are completable using only a keyboard.
- **SC-006**: 90% of operators can locate and submit feedback for a specific message on their first attempt without guidance.
- **SC-007**: Reply delivery status is visible to the operator within 5 seconds of the backend confirming or failing the send.
- **SC-008**: Connection loss is surfaced to the operator within 5 seconds of the disconnect occurring.

## Clarifications

### Session 2026-03-30

- Q: Does the dashboard need to alert the operator to high-priority messages when the browser tab is not actively focused? → A: No — operator is expected to keep the dashboard tab focused; no browser/OS notification or out-of-focus alerting required.
- Q: What is the expected steady-state volume of conversations the dashboard must handle performantly? → A: Medium — up to ~500 conversations, up to ~1,000 messages per thread.
- Q: What level of data privacy protection is required for sender phone numbers and message content displayed in the dashboard? → A: No masking required — display phone numbers and content in full; access is controlled at the network level.
- Q: When an updated triage result arrives for an already-triaged message, what should happen to the original result? → A: Replace silently — show only the latest triage result; no history of prior AI assessments is kept.
- Q: Can the operator interact with (view detail / submit feedback on) a message that is still pending triage? → A: Read-only — the detail panel opens showing message content and sender, but feedback controls are disabled until triage completes.

## Assumptions

- A single operator uses the dashboard at a time; multi-user collaboration (e.g., message claiming, assignment to team members) is out of scope.
- The operator is expected to keep the dashboard browser tab open and focused during active monitoring; no browser/OS push notifications or background alerts are required.
- The backend triage pipeline is already operational and delivers message data and AI results; this feature covers only the operator-facing UI layer.
- All messages visible in the dashboard have already been ingested from Signal; the UI does not initiate or manage Signal connections directly.
- The existing data model (Message, FeedbackMemory, Reply) covers all entity needs; no new storage requirements are introduced by this feature.
- The dashboard does not require login or user-level authentication in the initial version; access control is handled at the network or infrastructure level. No PII masking, redaction, or audit logging of viewed messages is required.
- Operator replies are sent through the existing Signal integration; the UI submits reply content and displays the resulting delivery status only.
- Category values are dynamic and sourced from the backend; the UI does not maintain a hardcoded category list.
- Real-time updates are delivered via a push mechanism already supported by the backend; the UI consumes events rather than polling.
- The expected scale ceiling is ~500 concurrent conversations and ~1,000 messages per thread; designs beyond this volume are out of scope.
