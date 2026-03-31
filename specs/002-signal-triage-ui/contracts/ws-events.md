# WebSocket Event Contract

**Branch**: `002-signal-triage-ui` | **Date**: 2026-03-30

The WebSocket endpoint is `GET /ws`. The server only sends; the client only receives. All interactive actions (feedback, reply) use standard HTTP POST via htmx.

---

## Server → Client: HTMX OOB HTML Fragments

All server-push events are rendered HTML fragments with `hx-swap-oob` attributes. The client-side htmx WebSocket extension (`hx-ext="ws"`) processes them automatically.

### Event 1: New Message (or Triage Completion)

Triggered when a new message completes triage. Updates the conversation list.

```html
<li id="conv-{sender_phone|group_id}"
    hx-swap-oob="outerHTML:#conv-{sender_phone|group_id}, afterbegin:#conversation-list">
  <!-- Rendered conversation_item.templ -->
</li>
```

**Behavior**: If the conversation already exists in the list, the OOB `outerHTML` swap replaces it in place (updating priority dot, preview, timestamp). If it doesn't exist yet, `afterbegin:#conversation-list` prepends it.

---

### Event 2: Triage Update (Re-score)

Triggered when a message receives a triage update that changes its priority or category.

```html
<div id="message-{id}" hx-swap-oob="outerHTML:#message-{id}">
  <!-- Rendered message_bubble.templ with updated priority/category -->
</div>
```

**Behavior**: Replaces the message bubble in the thread panel. If the triage detail panel is showing this message, it is also updated via a second OOB fragment:

```html
<div id="detail-panel" hx-swap-oob="outerHTML:#detail-panel">
  <!-- Rendered triage_detail.templ with updated values -->
</div>
```

---

### Event 3: Reply Delivery Status

Triggered when the Signal bridge confirms delivery or reports failure for an outgoing reply.

```html
<div id="reply-{id}" hx-swap-oob="outerHTML:#reply-{id}">
  <!-- Rendered reply_bubble.templ with updated delivery_status -->
</div>
```

---

## Client-Side JavaScript Events (non-htmx)

Two behaviors require minimal vanilla JS:

### Connection Status Indicator

```javascript
// On WebSocket open:
document.getElementById('ws-status-dot').className = 'dot dot-connected';

// On WebSocket close:
document.getElementById('ws-status-dot').className = 'dot dot-disconnected';
// Auto-reload after 3 seconds to attempt reconnect
setTimeout(() => location.reload(), 3000);
```

### Page Title Badge

```javascript
// On new high-priority message while tab is not focused:
// (parsed from a small JSON metadata annotation on the OOB fragment)
// <div data-event="new_message" data-priority="85" hx-swap-oob="...">
document.title = document.title.startsWith('(')
  ? document.title.replace(/\(\d+\)/, `(${count})`)
  : `(1) Signal Triage`;
```

No WebSocket message parsing beyond htmx's built-in OOB processing is needed.
