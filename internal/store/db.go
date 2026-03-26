package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// Open initializes the database connection pool and registers pgvector types.
func Open(ctx context.Context, dbURL string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse db config: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Register pgvector types
	_, err = pool.Exec(ctx, "SELECT '[]'::vector")
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to register pgvector types: %w", err)
	}

	log.Info().Str("stage", "store").Msg("connected to database and registered pgvector types")
	return pool, nil
}
