# Research: Signal AI Triage Dashboard UI

**Branch**: `002-signal-triage-ui` | **Date**: 2026-03-30

---

## 1. Conversation Grouping Strategy

**Decision**: Derive conversations from the `messages` table using a SQL `GROUP BY (sender_phone, group_id)` query — no new `conversations` table.

**Rationale**: At the target scale of ≤500 conversations and ≤500,000 total messages (500 × 1,000), a grouped query with proper indexing returns in single-digit milliseconds. Adding a materialized `conversations` table would require triggers or periodic refreshes to stay in sync with new messages, violating Principle IV (YAGNI) and Principle II (every schema change must ship as a migration). The existing indexes on `sender_phone` and `created_at` are sufficient.

**Query shape** (`GetConversations`):
```sql
SELECT
    sender_phone,
    group_id,
    MAX(created_at)            AS last_message_at,
    MAX(priority)              AS highest_priority,
    COUNT(*)                   AS message_count,
    -- latest category via subquery or DISTINCT ON
    (SELECT category FROM messages m2
     WHERE (m2.sender_phone = m.sender_phone OR (m2.group_id IS NOT NULL AND m2.group_id = m.group_id))
     ORDER BY m2.created_at DESC LIMIT 1) AS last_category
FROM messages m
GROUP BY sender_phone, group_id
ORDER BY MAX(created_at) DESC
```

**Alternatives considered**:
- Materialized view — rejected; adds DDL complexity, requires refresh strategy, overkill at this scale
- Separate `conversations` table — rejected; same reasons; extra migration with no query benefit at target scale

---

## 2. Three-Panel Layout — htmx Panel Switching

**Decision**: Use htmx partial loads with `hx-target` for panel transitions. No JavaScript required for panel selection.

**Rationale**: The constitution mandates "Go Templ + htmx; no client-side JS frameworks." htmx's `hx-get` + `hx-target` is the natural mechanism for loading server-rendered partials into named regions. This keeps all rendering server-side (templ) and all interaction declarative (htmx attributes).

**Interaction flow**:
1. **Select conversation** → conversation item carries `hx-get="/conversations/{id}/thread" hx-target="#thread-panel" hx-swap="innerHTML"`. Server renders `message_thread.templ` partial.
2. **Select message** → message bubble carries `hx-get="/messages/{id}/detail" hx-target="#detail-panel" hx-swap="innerHTML"`. Server renders `triage_detail.templ` partial.
3. **Submit feedback** → feedback form carries `hx-post="/messages/{id}/feedback" hx-target="#detail-panel" hx-swap="innerHTML"`. Server rerenders the detail panel with updated values.
4. **Send reply** → reply composer carries `hx-post="/messages/{id}/reply" hx-target="#reply-{id}" hx-swap="outerHTML"`. Server rerenders the reply bubble with updated delivery status.

**Alternative considered**: JavaScript-driven panel state (single-page app style) — rejected; unnecessary complexity, violates constitution's "no JS frameworks" guidance, and server-side rendering handles all display states natively.

---

## 3. Filter and Sort — URL-Based State

**Decision**: Filter and sort state live in URL query parameters. Filter chip clicks use `hx-get="/conversations?priority=high&category=urgent&sort=recent" hx-target="#conversation-list" hx-push-url="true"`.

**Rationale**: URL-based state is bookmarkable, back-button safe, and requires zero JavaScript. htmx's `hx-push-url` keeps the address bar in sync. The server reads query params in `ConversationListHandler` and applies WHERE clauses. `hx-include="[name='sort']"` on filter chips serializes all active controls.

**Filter implementation**:
- Priority filter: `WHERE priority >= 70` (high), `priority BETWEEN 40 AND 69` (medium), `priority < 40` (low)
- Category filter: `WHERE category = $1` (dynamic, sourced from `GetCategories()`)
- Status filter: `WHERE triage_status = $1` ('triaged' maps to 'completed' in DB, 'failed' maps to 'failed')
- Sort: `ORDER BY created_at DESC` (recent), `ORDER BY priority DESC` (priority), `ORDER BY unread_count DESC` (unread — computed in GROUP BY)

---

## 4. WebSocket Broadcast — Keep Existing HTMX OOB Pattern

**Decision**: Retain the existing hub broadcast pattern (server pushes HTMX OOB HTML to all connected clients). Extend to support three event types rendered as OOB fragments.

**Rationale**: The existing `hub.Broadcast([]byte)` mechanism is already implemented and working. Switching to JSON events would require a custom JavaScript event dispatcher — additional JS complexity for no added user value. HTMX OOB swaps (`hx-swap-oob`) can target any element by ID, giving the server precise control over what updates where.

**Broadcast event types**:

| Event | OOB target | Rendered component |
|-------|------------|--------------------|
| New message arrives | `#conversation-list` (prepend) | Updated `conversation_item.templ` |
| Triage update | `#message-{id}` (replace) | Updated `message_bubble.templ` |
| Reply status update | `#reply-{id}` (replace) | Updated `reply_bubble.templ` |

**Vanilla JS still needed for**:
- WebSocket connection status dot (toggle CSS class on connect/disconnect)
- Browser page-title badge update when new high-priority message arrives outside active conversation (tab title: `(1) Signal Triage`)

These two behaviors are not achievable with htmx alone and require minimal vanilla JS (~30 lines total) in `layout.templ`.

**Alternative considered**: Full JSON WebSocket protocol with custom JS dispatcher (as described in the input design spec) — rejected for this implementation; adds ~150 lines of JS, conflicts with htmx OOB approach, violates YAGNI at this scale.

---

## 5. Feedback Category Override — Migration 000002

**Decision**: Add `adjusted_category TEXT` column to `feedback_memory` via `000002_add_feedback_category.up.sql`. Update `InsertCorrection()` to accept an optional category parameter.

**Rationale**: The spec requires category correction as part of feedback (FR-005). The current `feedback_memory` schema only stores `adjusted_priority`. The column is nullable so existing rows are unaffected. The `store.FeedbackMemory` struct gains `AdjustedCategory *string`.

**Migration**:
```sql
-- 000002_add_feedback_category.up.sql
ALTER TABLE feedback_memory ADD COLUMN adjusted_category TEXT;

-- 000002_add_feedback_category.down.sql
ALTER TABLE feedback_memory DROP COLUMN adjusted_category;
```

---

## 6. Responsive Layout Strategy

**Decision**: CSS-only three-panel layout using Tailwind's `hidden`, `md:block`, `lg:flex` utilities. No JavaScript for panel visibility on desktop. On mobile/tablet, JavaScript tab-switching (three `<section>` elements, one visible at a time) driven by navigation tab bar clicks.

**Rationale**: Desktop (≥1024px) shows all three panels simultaneously via flexbox. Tablet (768–1023px) collapses the left panel to icon width and slides the right panel as an overlay. Mobile (<768px) shows one panel at a time. CSS handles the desktop case; minimal JS (~20 lines) manages the mobile/tablet panel-switching state.

**Alternative considered**: Full CSS media queries only — the mobile panel-switching (tap conversation → see thread → tap message → see detail) requires JS to manage which section is visible; CSS alone cannot track "which panel is active" state.

---

## 7. Accessibility

**Decision**: Use semantic HTML + ARIA attributes in templ components. No third-party accessibility library needed.

**Key attributes**:
- Conversation list: `role="listbox"`, each item `role="option"` + `aria-selected`
- Message thread: `role="log"` + `aria-live="polite"` for real-time message stream
- Priority indicators: text label alongside color dot (e.g., "High" span with `sr-only` class)
- Icon-only buttons: `aria-label` on each
- Reply composer: `aria-label="Reply to {sender}"` on textarea
- Focus management: htmx `hx-focus` on panel swap targets where appropriate
