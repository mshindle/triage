package cmd

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mshindle/triage/internal/config"
	internalsignal "github.com/mshindle/triage/internal/signal"
	"github.com/mshindle/triage/internal/store"
	"github.com/mshindle/triage/internal/triage"
	"github.com/mshindle/triage/internal/web"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"go.uber.org/fx"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the triage server and message listener",
	Run:   runServeCmd,
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().String("listen_addr", ":8081", "address to listen on for the web dashboard")
	serveCmd.Flags().String("signal_url", "", "Signal bridge WebSocket URL")
	serveCmd.Flags().String("signal_rest_url", "", "Signal bridge REST API URL")
	serveCmd.Flags().String("phone", "", "Operator phone number")
	serveCmd.Flags().String("model", "gpt-4o-mini", "OpenAI model for triage")
	serveCmd.Flags().String("embed_model", "text-embedding-3-small", "OpenAI model for embeddings")
	serveCmd.Flags().Int("embed_dims", 768, "Embedding dimensions")

	_ = v.BindPFlag("web.listen_addr", serveCmd.Flags().Lookup("listen_addr"))
	_ = v.BindPFlag("signal.url", serveCmd.Flags().Lookup("signal_url"))
	_ = v.BindPFlag("signal.rest_url", serveCmd.Flags().Lookup("signal_rest_url"))
	_ = v.BindPFlag("signal.phone", serveCmd.Flags().Lookup("phone"))
	_ = v.BindPFlag("llm.model", serveCmd.Flags().Lookup("model"))
	_ = v.BindPFlag("llm.embed_model", serveCmd.Flags().Lookup("embed_model"))
	_ = v.BindPFlag("llm.embed_dims", serveCmd.Flags().Lookup("embed_dims"))
}

func runServeCmd(cmd *cobra.Command, _ []string) {
	fx.New(
		CommonOptions(cmd),
		fx.Provide(
			func(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
				return store.Open(ctx, cfg.Database.URL)
			},
			triage.NewAnalyzer,
			web.NewHub,
			func(cfg *config.Config) *internalsignal.Sender {
				return internalsignal.NewSender(cfg.Signal.SendURL, cfg.Signal.Phone)
			},
			func(hub *web.Hub, analyzer *triage.Analyzer, pool *pgxpool.Pool, cfg *config.Config) (*internalsignal.Pipeline, error) {
				return internalsignal.NewPipeline(cfg.Signal.ReceiveURL, pool, hub, analyzer)
			},
			fx.Annotate(
				web.CreateServer,
				fx.ParamTags(`group:"serverOptions"`),
			),
			asServerOption(web.WithLogger),
			asServerOption(web.WithPool),
			asServerOption(web.WithHub),
			asServerOption(web.WithAnalyzer),
			asServerOption(web.WithSender),
		),
		fx.Invoke(startHub),
		fx.Invoke(startPipeline),
		fx.Invoke(startServer),
	).Run()
}

func startHub(lc fx.Lifecycle, ctx context.Context, hub *web.Hub) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			go hub.Run(ctx)
			return nil
		},
	})
}

func startPipeline(lc fx.Lifecycle, ctx context.Context, pipeline *internalsignal.Pipeline) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			go func() {
				if err := pipeline.Listen(ctx); err != nil && ctx.Err() == nil {
					log.Error().Err(err).Msg("pipeline listener failed")
				}
			}()
			return nil
		},
	})
}

func startServer(lc fx.Lifecycle, server *web.Server, cfg *config.Config, zl zerolog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			log.Info().Str("addr", cfg.Web.ListenAddr).Msg("starting web server")
			go func(address string) {
				if err := server.Run(address); err != nil && !errors.Is(err, http.ErrServerClosed) {
					zlog.Error().Err(err).Msg("http service failed")
				}
			}(cfg.Web.ListenAddr)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			zl.Info().Msg("shutting down http server")
			shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			if err := server.Shutdown(shutdownCtx); err != nil {
				zl.Error().Err(err).Msg("shutdown error")
				return err
			}
			zl.Info().Msg("server shutdown complete")
			return nil
		},
	})
}
