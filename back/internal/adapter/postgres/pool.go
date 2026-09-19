// Package postgres implements the repository ports on PostgreSQL.
package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
)

type Options struct {
	URL               string
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

// NewPool builds the connection pool and registers the schema enum types.
func NewPool(ctx context.Context, opts Options, log *slog.Logger) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(opts.URL)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}

	if poolCfg.ConnConfig.DefaultQueryExecMode == pgx.QueryExecModeSimpleProtocol {
		return nil, fmt.Errorf(
			"refusing to start with the simple protocol: it builds the query on the client, " +
				"which defeats parameter binding")
	}

	poolCfg.MaxConns = opts.MaxConns
	poolCfg.MinConns = opts.MinConns
	poolCfg.MaxConnLifetime = opts.MaxConnLifetime
	poolCfg.MaxConnIdleTime = opts.MaxConnIdleTime
	poolCfg.HealthCheckPeriod = opts.HealthCheckPeriod

	poolCfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		return registerEnumTypes(ctx, conn, log)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}

func registerEnumTypes(ctx context.Context, conn *pgx.Conn, log *slog.Logger) error {
	for name := range shared.PGEnumTypes() {
		for _, typeName := range []string{name, "_" + name} {
			dataType, err := conn.LoadType(ctx, typeName)
			if err != nil {
				log.Debug("enum type not registered", "type", typeName, "err", err)
				continue
			}
			conn.TypeMap().RegisterType(dataType)
		}
	}
	return nil
}
