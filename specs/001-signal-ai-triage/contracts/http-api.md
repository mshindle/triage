# HTTP API Contract: Signal AI Triage Engine

**Branch**: `001-signal-ai-triage` | **Date**: 2026-03-25

All endpoints are served by the Go `triage serve` process (default `:8081`).
The dashboard is a server-rendered application; API endpoints serve htmx partial
responses (HTML fragments) unless noted.

---

## Dashboard

### `GET /`

Returns the full dashboard HTML page (initial load).

**Response**: `200 OK` — Full HTML document including:
- Message stream container (`id="message-stream"`)
- htmx WebSocket connection (`hx-ext="ws"`, `ws-connect="/ws"`)

---

## Real-Time Updates

### `GET /ws`

WebSocket upgrade endpoint. The dashboard connects here via htmx to receive
live HTML fragments for new messages and status updates.

**Protocol**: See `contracts/websocket.md`

---

## Feedback (Priority Correction)

### `POST /messages/{id}/feedback`

Submits a priority correction for a message. Returns an updated message card fragment.

**Path parameters**:
- `id` — integer, message ID

**Request body** (`application/x-www-form-urlencoded`):
```
direction=high   # or direction=low
```

**Response**: `200 OK` — HTML fragment: updated message card with new priority badge.

**Error responses**:
- `404 Not Found` — message ID does not exist
- `400 Bad Request` — invalid `direction` value
- `500 Internal Server Error` — database write failed (logged)

**Side effects**:
1. `messages.priority` is updated in the database
2. A new `feedback_memory` record is created with embedding of the message content
3. The updated message card is broadcast to all connected WebSocket clients via the hub

---

## Reply

### `POST /messages/{id}/reply`

Sends a reply to the original sender (or group) via the Signal bridge.

**Path parameters**:
- `id` — integer, message ID

**Request body** (`application/x-www-form-urlencoded`):
```
content=Hello%2C+thanks+for+your+message
```

**Response**: `200 OK` — HTML fragment: updated reply status badge on the message card.

**Error responses**:
- `404 Not Found` — message ID does not exist
- `400 Bad Request` — empty content
- `502 Bad Gateway` — Signal bridge returned an error (error surfaced in badge,
  not silently dropped per FR-014)
- `500 Internal Server Error` — database write or bridge call failed unexpectedly

**Side effects**:
1. A `replies` record is created with `delivery_status = 'pending'`
2. A `POST /v2/send` is made to the signal-cli-rest-api container
3. On success: `delivery_status` updated to `'delivered'`
4. On failure: `delivery_status` updated to `'failed'`, `error_detail` populated

**Group routing logic**:
- If `messages.group_id IS NOT NULL`: reply is sent to the group (base64 group ID
  in `groupId` field of the Signal API payload)
- If `messages.group_id IS NULL`: reply is sent as a direct message to
  `messages.sender_phone`

---

## Signal Bridge (Outbound — called by this service)

This service calls the signal-cli-rest-api container. Documented here for
implementation reference.

### `POST http://signal-api:8080/v2/send`

**Direct message**:
```json
{
  "message": "reply text",
  "number": "+1operator",
  "recipients": ["+1recipient"]
}
```

**Group message**:
```json
{
  "message": "reply text",
  "number": "+1operator",
  "groupId": "base64groupId=="
}
```

**Success**: `201 Created`
**Error**: `4xx/5xx` with JSON error body
