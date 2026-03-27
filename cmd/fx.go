package cmd

import (
	"context"

	"github.com/ipfans/fxlogger"
	"github.com/mshindle/triage/internal/config"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

// CommonOptions returns a set of common fx.Options to be used across multiple cobra commands.
func CommonOptions(cmd *cobra.Command) fx.Option {
	return fx.Options(
		fx.Supply(v),
		fx.Supply(zlog),
		fx.Supply(
			fx.Annotate(
				cmd.Context(),
				fx.As(new(context.Context)),
			),
		),
		fx.Provide(
			config.LoadConfig,
		),
		fx.WithLogger(
			func(logger zerolog.Logger) fxevent.Logger {
				return fxlogger.WithZerolog(logger)()
			},
		),
	)
}

func asServerOption(f any) any {
	return fx.Annotate(
		f,
		fx.ResultTags(`group:"serverOptions"`),
	)
}
