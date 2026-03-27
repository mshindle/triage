package cmd

import (
	"context"

	"github.com/mshindle/triage/internal/config"
	"github.com/mshindle/triage/internal/store"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"go.uber.org/fx"
)

// migrateCmd represents the migrate command
var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "migrate the database",
	Long:  ``,
	Run: func(cmd *cobra.Command, _ []string) {
		fx.New(
			CommonOptions(cmd),
			fx.Invoke(runMigration),
		).Run()
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}

func runMigration(lc fx.Lifecycle, cfg *config.Config, zl zerolog.Logger, shutdowner fx.Shutdowner) {
	lc.Append(fx.Hook{
		OnStart: func(startCtx context.Context) error {
			dbURL := cfg.Database.URL
			zl.Info().Str("db", dbURL).Msg("migration starting")
			if err := store.RunMigrations(dbURL); err != nil {
				zl.Error().Err(err).Msg("migration failed")
				return err
			}
			return shutdowner.Shutdown()
		},
	})
}
