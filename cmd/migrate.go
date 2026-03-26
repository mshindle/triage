package cmd

import (
	"fmt"

	"github.com/mshindle/triage/internal/store"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// migrateCmd represents the migrate command
var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "migrate the database",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		dbURL := viper.GetString("db_url")

		// Run migrations before starting the app logic
		if err := store.RunMigrations(dbURL); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}
