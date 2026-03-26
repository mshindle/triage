package store

import (
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/rs/zerolog/log"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func RunMigrations(dbURL string) error {
	// 1. Create a source driver from the embedded filesystem
	d, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create iofs driver: %w", err)
	}

	// 2. Initialize the migrator
	m, err := migrate.NewWithSourceInstance("iofs", d, dbURL)
	if err != nil {
		return fmt.Errorf("failed to initialize migrator: %w", err)
	}

	// 3. Run 'Up' migrations
	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			log.Info().Msg("database schema is up to date")
			return nil
		}
		return fmt.Errorf("failed to run up migrations: %w", err)
	}

	log.Info().Msg("migrations applied successfully")
	return nil
}
