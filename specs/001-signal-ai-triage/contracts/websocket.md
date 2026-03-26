# WebSocket Contract: Signal AI Triage Engine

**Branch**: `001-signal-ai-triage` | **Date**: 2026-03-25

## Overview

The dashboard maintains a persistent WebSocket connection to `/ws`. The server sends
HTML fragments when new messages are triaged or statuses change. The client (htmx)
swaps fragments into the DOM using out-of-band swaps (`hx-swap-oob`).

The connection is one-directional for updates (server → browser). User actions
(feedback, reply) use standard HTTP POST endpoints.

---

## Connection

```
ws://localhost:8081/ws
```

Initiated by htmx on the dashboard page:
```html
<div hx-ext="ws" ws-connect="/ws">
  <div id="message-stream">
    <!-- message cards populated here -->
  </div>
</div>
```

**Important**: The element with `ws-connect` MUST NOT be the target of any htmx swap.
Swapping this element disconnects the WebSocket.

---

## Server → Client Messages

All messages are HTML fragments. The server never sends JSON over this channel.

### New Message Card

Sent when a new message completes triage (or arrives with `triage_status = failed`).

```html
<div hx-swap-oob="afterbegin:#message-stream">
  <!-- rendered MessageCard Templ component -->
  <div id="msg-{id}" class="message-card priority-{high|medium|low}" ...>
    ...
  </div>
</div>
```

The `hx-swap-oob` attribute causes htmx to prepend the fragment to `#message-stream`
without touching any other part of the page.

### Priority Update (after feedback)

Sent to all connected clients when a message's priority is updated via feedback.

```html
<div hx-swap-oob="outerHTML:#msg-{id}">
  <!-- re-rendered MessageCard with updated priority badge -->
</div>
```

### Reply Status Update

Sent when a reply's delivery status changes to `delivered` or `failed`.

```html
<div hx-swap-oob="outerHTML:#reply-status-{reply_id}">
  <!-- re-rendered ReplyStatus Templ component -->
</div>
```

---

## Hub Architecture

The Go server maintains a broadcast hub:

```
signal listener → pipeline → hub.Broadcast(fragment)
                                  ↓
                          all connected ws clients
```

- New clients register with the hub on WebSocket upgrade
- Disconnected clients are unregistered automatically
- The hub is safe for concurrent broadcast (goroutine-per-client write)

---

## Inbound WebSocket (Signal Bridge)

This service connects to the signal-cli-rest-api as a **client**, not a server.

**Endpoint**: `ws://signal-api:8080/v1/receive/{phone}`

**Message format** (JSON-RPC from signal-cli):
```json
{
  "envelope": {
    "source": "+1sender",
    "sourceDevice": 1,
    "timestamp": 1234567890123,
    "dataMessage": {
      "timestamp": 1234567890123,
      "message": "Hello world",
      "groupInfo": {
        "groupId": "base64groupId=="
      }
    }
  }
}
```

Fields used by the ingestion pipeline:
- `envelope.source` → `messages.sender_phone`
- `envelope.dataMessage.message` → `messages.content`
- `envelope.dataMessage.groupInfo.groupId` → `messages.group_id` (null if absent)
- `envelope.timestamp` → used to construct `signal_id` (if no dedicated message ID)

**Reconnect policy**: On disconnect, the listener MUST attempt reconnection with
exponential backoff (1s, 2s, 4s, max 30s). All reconnect attempts MUST be logged
with `stage=signal_listener`.
