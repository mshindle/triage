package web

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"

	"github.com/coder/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/mshindle/triage/internal/store"
	"github.com/mshindle/triage/internal/triage"
	"github.com/mshindle/triage/internal/web/templates"
	"github.com/rs/zerolog"
)

var (
	ErrInternalServer     = echo.NewHTTPError(http.StatusInternalServerError, "Internal Server Error")
	ErrUserCancelled      = echo.NewHTTPError(http.StatusRequestTimeout, "user cancelled")
	ErrServiceUnavailable = echo.NewHTTPError(http.StatusServiceUnavailable, "service unavailable")
	ErrInvalidID          = echo.NewHTTPError(http.StatusBadRequest, "invalid message ID")
	ErrInvalidDirection   = echo.NewHTTPError(http.StatusBadRequest, "invalid direction")
	ErrMessageNotFound    = echo.NewHTTPError(http.StatusNotFound, "message not found")
)

// parseFilters extracts ConversationFilters from an echo.Context's query params.
func parseFilters(c echo.Context) store.ConversationFilters {
	return store.ConversationFilters{
		Priority: c.QueryParams()["priority"],
		Category: c.QueryParam("category"),
		Status:   c.QueryParam("status"),
		Sort:     c.QueryParam("sort"),
	}
}

func DashboardHandler(pool *pgxpool.Pool) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		zl := zerolog.Ctx(ctx)

		filters := parseFilters(c)

		convs, err := store.GetConversations(ctx, pool, filters)
		if err != nil {
			zl.Error().Err(err).Msg("failed to load conversations")
			return ErrInternalServer.WithInternal(err)
		}

		categories, err := store.GetCategories(ctx, pool)
		if err != nil {
			zl.Error().Err(err).Msg("failed to load categories for dashboard")
			categories = nil // non-fatal
		}

		return Render(c, http.StatusOK, templates.Dashboard(convs, categories, filters))
	}
}

// ConversationListHandler serves the left-panel conversation list partial (used by filters/sort).
func ConversationListHandler(pool *pgxpool.Pool) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		filters := parseFilters(c)

		convs, err := store.GetConversations(ctx, pool, filters)
		if err != nil {
			zerolog.Ctx(ctx).Error().Err(err).Msg("failed to load conversations")
			return Render(c, http.StatusOK, templates.ConversationList(nil, store.ConversationFilters{}))
		}
		return Render(c, http.StatusOK, templates.ConversationList(convs, filters))
	}
}

// ThreadHandler serves the center-panel message thread partial.
func ThreadHandler(pool *pgxpool.Pool) echo.HandlerFunc {
	return func(c echo.Context) error {
		identity := c.Param("id")
		ctx := c.Request().Context()
		zl := zerolog.Ctx(ctx)

		msgs, err := store.GetMessagesByConversation(ctx, pool, identity)
		if err != nil {
			zl.Error().Err(err).Str("identity", identity).Msg("failed to load thread")
			return c.HTML(http.StatusOK, `<div id="thread-panel" class="p-8 text-red-400 text-sm">Failed to load conversation. Please try again.</div>`)
		}

		// Collect message IDs for reply lookup.
		msgIDs := make([]int64, len(msgs))
		for i, m := range msgs {
			msgIDs[i] = m.ID
		}

		replies, err := store.GetRepliesByMessageIDs(ctx, pool, msgIDs)
		if err != nil {
			zl.Error().Err(err).Str("identity", identity).Msg("failed to load replies for thread")
			replies = nil // non-fatal
		}

		// Build reply map: message ID → replies.
		replyMap := make(map[int64][]store.Reply)
		for _, r := range replies {
			replyMap[r.OriginalMessageID] = append(replyMap[r.OriginalMessageID], r)
		}

		// Last message ID for the reply composer.
		var lastMsgID int64
		if len(msgs) > 0 {
			lastMsgID = msgs[len(msgs)-1].ID
		}

		return Render(c, http.StatusOK, templates.MessageThread(identity, msgs, replyMap, lastMsgID))
	}
}

// DetailHandler returns the right-panel triage detail for a single message.
func DetailHandler(pool *pgxpool.Pool) echo.HandlerFunc {
	return func(c echo.Context) error {
		var id int64
		err := echo.PathParamsBinder(c).Int64("id", &id).BindError()
		if err != nil {
			return ErrInvalidID.WithInternal(err)
		}

		ctx := c.Request().Context()
		zl := zerolog.Ctx(ctx)

		msg, err := store.GetMessageByID(ctx, pool, id)
		if err != nil {
			zl.Error().Err(err).Int64("id", id).Msg("failed to load message for detail")
			return ErrMessageNotFound
		}

		feedbacks, err := store.GetFeedbackByMessage(ctx, pool, id)
		if err != nil {
			zl.Error().Err(err).Int64("id", id).Msg("failed to load feedback for detail")
			feedbacks = nil // non-fatal: show panel without history
		}

		categories, err := store.GetCategories(ctx, pool)
		if err != nil {
			zl.Error().Err(err).Msg("failed to load categories for detail")
			categories = nil // non-fatal: dropdown will be empty
		}

		return Render(c, http.StatusOK, templates.TriageDetail(msg, feedbacks, categories))
	}
}

func WSHandler(hub *Hub) echo.HandlerFunc {
	return func(c echo.Context) error {
		conn, err := websocket.Accept(c.Response().Writer, c.Request(), nil)
		if err != nil {
			return ErrInternalServer.WithInternal(fmt.Errorf("websocket accept failed: %w", err))
		}
		defer conn.Close(websocket.StatusNormalClosure, "shutting down")

		clientCh := make(chan []byte, 10)
		hub.Register(clientCh)
		defer hub.Unregister(clientCh)

		ctx := c.Request().Context()
		for {
			select {
			case <-ctx.Done():
				return ErrUserCancelled
			case msg, ok := <-clientCh:
				if !ok {
					return ErrServiceUnavailable
				}
				err = conn.Write(ctx, websocket.MessageText, msg)
				if err != nil {
					zerolog.Ctx(ctx).Debug().Err(err).Msg("websocket client disconnected")
					return nil
				}
			}
		}
	}
}

func FeedbackHandler(pool *pgxpool.Pool, hub *Hub, analyzer triage.Analyzer) echo.HandlerFunc {
	return func(c echo.Context) error {
		var zl = zerolog.Ctx(c.Request().Context())

		var id int64
		err := echo.PathParamsBinder(c).Int64("id", &id).BindError()
		if err != nil {
			return ErrInvalidID.WithInternal(err)
		}

		// Parse priority: prefer numeric form field, fall back to direction string.
		adjustedPriority := -1
		if raw := c.FormValue("priority"); raw != "" {
			if p, err := strconv.Atoi(raw); err == nil && p >= 0 && p <= 100 {
				adjustedPriority = p
			}
		}
		if adjustedPriority < 0 {
			switch c.FormValue("direction") {
			case "high":
				adjustedPriority = 90
			case "low":
				adjustedPriority = 10
			default:
				return ErrInvalidDirection.WithInternal(fmt.Errorf("no valid priority or direction provided"))
			}
		}

		feedbackText := c.FormValue("text")
		categoryStr := c.FormValue("category")
		var adjustedCategory *string
		if categoryStr != "" {
			adjustedCategory = &categoryStr
		}

		ctx := c.Request().Context()

		// Load message to get SignalID and content.
		msg, err := store.GetMessageByID(ctx, pool, id)
		if err != nil {
			zl.Error().Err(err).Int64("id", id).Msg("failed to load message for feedback")
			return ErrMessageNotFound
		}

		if err := store.UpdateMessagePriority(ctx, pool, id, msg.SignalID, adjustedPriority); err != nil {
			zl.Error().Err(err).Msg("failed to update priority")
			return ErrInternalServer.WithInternal(err)
		}

		var embedding []float32
		embedding, err = analyzer.GenerateEmbedding(ctx, msg.SignalID, msg.Content)
		if err != nil {
			zl.Error().Err(err).Msg("failed to generate embedding for feedback")
			return ErrInternalServer.WithInternal(fmt.Errorf("failed to generate embedding: %w", err))
		}
		if err = store.InsertCorrection(ctx, pool, id, msg.SignalID, feedbackText, adjustedPriority, adjustedCategory, embedding); err != nil {
			zl.Error().Err(err).Msg("failed to insert correction")
		}

		// Reload updated message and feedback history for re-render.
		msg, err = store.GetMessageByID(ctx, pool, id)
		if err != nil {
			zl.Error().Err(err).Int64("id", id).Msg("failed to reload message after feedback")
			return ErrInternalServer.WithInternal(err)
		}
		feedbacks, err := store.GetFeedbackByMessage(ctx, pool, id)
		if err != nil {
			zl.Error().Err(err).Msg("failed to load feedback history after correction")
			feedbacks = nil
		}
		categories, err := store.GetCategories(ctx, pool)
		if err != nil {
			zl.Error().Err(err).Msg("failed to load categories after correction")
			categories = nil
		}

		// OOB broadcast to update the message bubble priority strip for all connected clients.
		var bubBuf bytes.Buffer
		if err := templates.MessageBubble(msg).Render(ctx, &bubBuf); err != nil {
			zl.Error().Err(err).Msg("failed to render message bubble for broadcast")
		} else {
			oob := fmt.Sprintf(`<div hx-swap-oob="outerHTML:#message-%d">%s</div>`, id, bubBuf.String())
			hub.Broadcast([]byte(oob))
		}

		// Return the refreshed detail panel as the primary response (outerHTML swap).
		return Render(c, http.StatusOK, templates.TriageDetail(msg, feedbacks, categories))
	}
}

func ReplyHandler(pool *pgxpool.Pool, hub *Hub, sender Sender) echo.HandlerFunc {
	return func(c echo.Context) error {
		var zl = zerolog.Ctx(c.Request().Context())
		ctx := c.Request().Context()

		var id int64
		err := echo.PathParamsBinder(c).Int64("id", &id).BindError()
		if err != nil {
			return ErrInvalidID.WithInternal(err)
		}

		content := c.FormValue("content")
		if content == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "reply content cannot be empty")
		}

		// Load message to get sender details/group id.
		msg, err := store.GetMessageByID(ctx, pool, id)
		if err != nil {
			zl.Error().Err(err).Int64("id", id).Msg("failed to load message for reply")
			return ErrMessageNotFound
		}

		replyID, err := store.InsertReply(ctx, pool, id, msg.SignalID, content)
		if err != nil {
			zl.Error().Err(err).Msg("failed to insert reply")
			return ErrInternalServer.WithInternal(err)
		}

		err = sender.SendReply(ctx, msg, content)
		status := "delivered"
		errDetail := ""
		if err != nil {
			zl.Error().Err(err).Msg("failed to send signal reply")
			status = "failed"
			errDetail = err.Error()
		}

		if err := store.UpdateDeliveryStatus(ctx, pool, replyID, msg.SignalID, status, errDetail); err != nil {
			zl.Error().Err(err).Msg("failed to update delivery status")
		}

		// Build reply struct for rendering (avoids an extra DB round-trip).
		reply := store.Reply{
			ID:                replyID,
			OriginalMessageID: id,
			Content:           content,
			DeliveryStatus:    status,
		}
		if errDetail != "" {
			reply.ErrorDetail = &errDetail
		}

		// Render the reply bubble.
		var bubBuf bytes.Buffer
		if err := templates.ReplyBubble(reply).Render(ctx, &bubBuf); err != nil {
			zl.Error().Err(err).Msg("failed to render reply bubble")
			return ErrInternalServer.WithInternal(err)
		}

		// Determine the conversation-specific stream ID.
		var convIdentity string
		if msg.GroupID != nil && *msg.GroupID != "" {
			convIdentity = "group:" + *msg.GroupID
		} else {
			convIdentity = msg.SenderPhone
		}
		streamID := templates.ConvStreamID(convIdentity)

		// OOB: append reply bubble to the open thread for all connected clients.
		oobAppend := fmt.Sprintf(`<div hx-swap-oob="beforeend:#%s">%s</div>`, streamID, bubBuf.String())
		hub.Broadcast([]byte(oobAppend))

		// Render the fresh reply composer (resets the form).
		var compBuf bytes.Buffer
		if err := templates.ReplyComposer(id).Render(ctx, &compBuf); err != nil {
			zl.Error().Err(err).Msg("failed to render reply composer")
			return ErrInternalServer.WithInternal(err)
		}

		// Primary response: fresh composer only. The WebSocket broadcast above
		// already delivers the bubble to all connected clients (including the submitter).
		zl.Info().
			Str("stage", "reply_status_broadcast").
			Str("signal_id", msg.SignalID).
			Int64("reply_id", replyID).
			Str("status", status).
			Msg("reply sent and broadcast")

		return c.HTML(http.StatusOK, compBuf.String())
	}
}
