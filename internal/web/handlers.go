package web

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/coder/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/mshindle/triage/internal/signal"
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

func DashboardHandler(pool *pgxpool.Pool) echo.HandlerFunc {
	return func(c echo.Context) error {
		messages, err := store.GetMessages(c.Request().Context(), pool)
		if err != nil {
			zerolog.Ctx(c.Request().Context()).Error().Err(err).Msg("failed to get messages")
			return ErrInternalServer.WithInternal(err)
		}
		component := templates.Dashboard(messages)
		return Render(c, http.StatusOK, component)
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
					zerolog.Ctx(ctx).Error().Err(err).Msg("websocket write failed")
					return ErrServiceUnavailable.WithInternal(err)
				}
			}
		}
	}
}

func FeedbackHandler(pool *pgxpool.Pool, hub *Hub, analyzer *triage.Analyzer) echo.HandlerFunc {
	return func(c echo.Context) error {
		var zl = zerolog.Ctx(c.Request().Context())

		var id int64
		err := echo.PathParamsBinder(c).Int64("id", &id).BindError()
		if err != nil {
			return ErrInvalidID.WithInternal(err)
		}

		direction := c.FormValue("direction")
		adjustedPriority := 50
		switch direction {
		case "high":
			adjustedPriority = 90
		case "low":
			adjustedPriority = 10
		default:
			return ErrInvalidDirection.WithInternal(fmt.Errorf("unknown direction: %q", direction))
		}

		ctx := c.Request().Context()

		// Load message to get SignalID and content
		messages, err := store.GetMessages(ctx, pool)
		if err != nil {
			zl.Error().Err(err).Msg("failed to load messages for feedback")
			return ErrInternalServer.WithInternal(err)
		}

		var msg store.Message
		found := false
		for _, m := range messages {
			if m.ID == id {
				msg = m
				found = true
				break
			}
		}
		if !found {
			return ErrMessageNotFound
		}

		if err := store.UpdateMessagePriority(ctx, pool, id, msg.SignalID, adjustedPriority); err != nil {
			zl.Error().Err(err).Msg("failed to update priority")
			return ErrInternalServer.WithInternal(err)
		}

		// Update our local copy for rendering
		msg.Priority = adjustedPriority

		var embedding []float32
		embedding, err = analyzer.GenerateEmbedding(ctx, msg.SignalID, msg.Content)
		if err != nil {
			zl.Error().Err(err).Msg("failed to generate embedding for feedback")
			return ErrInternalServer.WithInternal(fmt.Errorf("failed to generate embedding: %w", err))
		}
		if err = store.InsertCorrection(ctx, pool, id, msg.SignalID, msg.Content, adjustedPriority, embedding); err != nil {
			zl.Error().Err(err).Msg("failed to insert correction")
		}

		var buf bytes.Buffer
		if err := templates.MessageCard(msg).Render(ctx, &buf); err != nil {
			zl.Error().Err(err).Msg("failed to render updated card")
			return ErrInternalServer.WithInternal(err)
		}

		// Broadcast to all clients
		oob := fmt.Sprintf(`<div hx-swap-oob="outerHTML:#msg-%d">%s</div>`, id, buf.String())
		hub.Broadcast([]byte(oob))

		// Return the card as the primary response body for the HTMX swap
		return c.HTML(http.StatusOK, buf.String())
	}
}

func ReplyHandler(pool *pgxpool.Pool, hub *Hub, sender *signal.Sender) echo.HandlerFunc {
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

		// Load message to get sender details/group id
		messages, err := store.GetMessages(ctx, pool)
		if err != nil {
			zl.Error().Err(err).Msg("failed to load messages for reply")
			return ErrInternalServer.WithInternal(err)
		}

		var msg store.Message
		found := false
		for _, m := range messages {
			if m.ID == id {
				msg = m
				found = true
				break
			}
		}
		if !found {
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

		if status == "delivered" {
			return c.HTML(http.StatusOK, `<span class="text-green-600">Delivered</span>`)
		}
		return c.HTML(http.StatusOK, fmt.Sprintf(`<span class="text-red-600" title="%s">Failed</span>`, errDetail))
	}
}
