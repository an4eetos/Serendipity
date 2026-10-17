// Package database opens the Postgres connection pool.
package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"

	"github.com/an4eetos/serendipity/backend/infra/config"
)

// NewPool opens the pool. The app only starts once the database answers, and
// the pool closes when the app stops.
func NewPool(lc fx.Lifecycle, cfg config.Config) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(context.Background(), cfg.Database.URL)
	if err != nil {
		return nil, err
	}
	lc.Append(fx.Hook{
		OnStart: pool.Ping,
		OnStop: func(context.Context) error {
			pool.Close()
			return nil
		},
	})
	return pool, nil
}
