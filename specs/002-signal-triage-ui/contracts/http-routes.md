# HTTP Route Contracts

**Branch**: `002-signal-triage-ui` | **Date**: 2026-03-30

All routes serve htmx partial HTML unless noted. Content-Type is `text/html`.

---

## Existing Routes (updated)

### `GET /`
**Handler**: `DashboardHandler`
**Change**: Now passes `[]ConversationSummary` (not `[]Message`) to `dashboard.templ`. Renders the full three-panel shell with the conversation list populated. Thread and detail panels are empty on initial load.
**Query params**: none on initial load

---

### `POST /messages/:id/feedback`
**Handler**: `FeedbackHandler`
**Change**: Now accepts `adjusted_category` form field in addition to `priority` and `text`. Stores via updated `InsertCorrection()`. Returns a rerendered `triage_detail.templ` partial targeted at `#detail-panel`.

**Request body** (form-encoded):
```
priority=85
category=urgent
text=This is a high priority customer issue
```

**Response**: Full `triage_detail.templ` HTML fragment (htmx swaps `#detail-panel`)

---

### `POST /messages/:id/reply`
**Handler**: `ReplyHandler`
**Change**: None — existing implementation unchanged. Returns updated `reply_bubble.templ` partial.

---

## New Routes

### `GET /conversations`
**Handler**: `ConversationListHandler`
**Purpose**: Returns a rerendered conversation list (left panel) — used by filter chips and sort dropdown.
**Query params**:

| Param | Values | Default |
|-------|--------|---------|
| `priority` | `high`, `medium`, `low` (multi) | all |
| `category` | any category string | all |
| `status` | `triaged`, `failed` | all |
| `sort` | `recent`, `priority`, `unread` | `recent` |

**Response**: Rendered `conversation_list.templ` fragment (htmx swaps `#conversation-list`)

---

### `GET /conversations/:id/thread`
**Handler**: `ThreadHandler`
**Purpose**: Loads the message thread for a given conversation into the center panel.

**Path parameter**: `:id` is `{sender_phone}` for direct conversations, or `group:{group_id}` for group conversations.

**Response**: Rendered `message_thread.templ` fragment (htmx swaps `#thread-panel`)

---

### `GET /messages/:id/detail`
**Handler**: `DetailHandler`
**Purpose**: Loads the triage detail and feedback form for a specific message into the right panel.

**Response**: Rendered `triage_detail.templ` fragment (htmx swaps `#detail-panel`)

---

## Error Responses

All handlers return HTTP 200 with an inline error fragment on failure (htmx swaps the target with an error message). HTTP 4xx/5xx are reserved for catastrophic failures only, as htmx default behavior on error status codes is to not swap the target.

```html
<!-- Example error fragment -->
<div id="thread-panel" class="panel-error">
  <p>Could not load conversation. Please try again.</p>
</div>
```
