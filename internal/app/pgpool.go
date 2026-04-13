package app

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func initPgPool(dsn string) (*pgxpool.Pool, error) {
	pgConf, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.ParseConfig: %w", err)
	}

	pgConf.MaxConns = 8
	pgConf.MinConns = 2
	pgConf.MaxConnLifetime = time.Hour
	pgConf.MaxConnIdleTime = 10 * time.Minute
	pgConf.HealthCheckPeriod = time.Minute

	var pgpool *pgxpool.Pool

	pgpool, err = pgxpool.NewWithConfig(context.Background(), pgConf)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.NewWithConfig: %w", err)
	}

	return pgpool, nil
}
