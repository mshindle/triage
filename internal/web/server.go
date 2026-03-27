package web

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/mshindle/triage/internal/signal"
	"github.com/mshindle/triage/internal/triage"
	"github.com/rs/zerolog"
	"github.com/ziflex/lecho/v3"
)

type Server struct {
	router   *echo.Echo
	logger   zerolog.Logger
	pool     *pgxpool.Pool
	hub      *Hub
	analyzer *triage.Analyzer
	sender   *signal.Sender
}

// Option defines a function which configures the server
type Option func(*Server)

func CreateServer(opts ...Option) *Server {
	s := &Server{
		logger: zerolog.Nop(),
	}
	for _, opt := range opts {
		opt(s)
	}

	// init router
	s.router = echo.New()
	s.router.HideBanner = true

	// configure the logger
	logger := lecho.From(s.logger)
	s.router.Logger = logger

	// configure middleware
	s.router.Use(
		middleware.RequestID(),
		lecho.Middleware(lecho.Config{Logger: logger}),
	)

	// configure routes
	s.router.GET("/", DashboardHandler(s.pool))
	s.router.GET("/ws", WSHandler(s.hub))
	s.router.POST("/messages/:id/feedback", FeedbackHandler(s.pool, s.hub, s.analyzer))
	s.router.POST("/messages/:id/reply", ReplyHandler(s.pool, s.hub, s.sender))

	return s
}

func (s *Server) Run(addr string) error {
	return s.router.Start(addr)
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.router.Shutdown(ctx)
}

func WithLogger(logger zerolog.Logger) Option {
	return func(s *Server) {
		s.logger = logger
	}
}

func WithPool(pool *pgxpool.Pool) Option {
	return func(s *Server) {
		s.pool = pool
	}
}

func WithHub(hub *Hub) Option {
	return func(s *Server) {
		s.hub = hub
	}
}

func WithAnalyzer(analyzer *triage.Analyzer) Option {
	return func(s *Server) {
		s.analyzer = analyzer
	}
}

func WithSender(sender *signal.Sender) Option {
	return func(s *Server) {
		s.sender = sender
	}
}
