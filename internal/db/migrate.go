package db

import (
	"errors"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"musica-colaborativa-api/migrations"
)

func migrator(url string) (*migrate.Migrate, error) {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("fuente de migraciones: %w", err)
	}
	// El driver de golang-migrate para pgx v5 se registra con el esquema pgx5://
	for _, p := range []string{"postgresql://", "postgres://"} {
		if strings.HasPrefix(url, p) {
			url = "pgx5://" + strings.TrimPrefix(url, p)
			break
		}
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, url)
	if err != nil {
		return nil, fmt.Errorf("iniciar migraciones: %w", err)
	}
	return m, nil
}

// Migrate aplica las migraciones pendientes. Debe recibir la URL del pooler en
// modo sesión (DATABASE_MIGRATE_URL), ya que usa advisory locks.
func Migrate(url string) error {
	m, err := migrator(url)
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("aplicar migraciones: %w", err)
	}
	return nil
}

// MigrateDown revierte todas las migraciones (uso en tests/mantenimiento).
func MigrateDown(url string) error {
	m, err := migrator(url)
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("revertir migraciones: %w", err)
	}
	return nil
}
