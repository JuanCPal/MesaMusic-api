package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB envuelve el pool de conexiones a PostgreSQL.
type DB struct {
	pool *pgxpool.Pool
}

// Options configura el pool.
type Options struct {
	MaxConns int32
	MinConns int32
}

// Connect crea el pool y hace ping. Usa simple_protocol para ser compatible
// con el pooler de Supabase en modo transacción (sin prepared statements).
func Connect(ctx context.Context, url string, opts Options) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("DATABASE_URL inválida: %w", err)
	}
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	if opts.MaxConns > 0 {
		cfg.MaxConns = opts.MaxConns
	}
	if opts.MinConns >= 0 && opts.MinConns <= cfg.MaxConns {
		cfg.MinConns = opts.MinConns
	}
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("crear pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping a la base de datos: %w", err)
	}
	return &DB{pool: pool}, nil
}

// Pool expone el pool para los repositorios futuros.
func (d *DB) Pool() *pgxpool.Pool { return d.pool }

// Ping verifica la conexión con un timeout corto.
func (d *DB) Ping(ctx context.Context, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return d.pool.Ping(ctx)
}

// Close cierra el pool.
func (d *DB) Close() {
	d.pool.Close()
}
