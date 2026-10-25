//go:build integration

package db

import (
	"context"
	"os"
	"testing"
	"time"
)

// Requiere TEST_DATABASE_URL y TEST_DATABASE_MIGRATE_URL apuntando a una BD
// de pruebas (NO producción): el test revierte las migraciones al final.
func TestMigrateAndTablesExist(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	migURL := os.Getenv("TEST_DATABASE_MIGRATE_URL")
	if url == "" || migURL == "" {
		t.Skip("TEST_DATABASE_URL / TEST_DATABASE_MIGRATE_URL no configuradas")
	}

	if err := Migrate(migURL); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = MigrateDown(migURL) })
	// Segunda corrida: no debe reaplicar ni fallar.
	if err := Migrate(migURL); err != nil {
		t.Fatalf("re-migrar: %v", err)
	}

	d, err := Connect(context.Background(), url, Options{MaxConns: 5})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := d.Ping(context.Background(), 2*time.Second); err != nil {
		t.Fatal(err)
	}

	for _, table := range []string{"sessions", "queue_items"} {
		var exists bool
		err := d.Pool().QueryRow(context.Background(),
			`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1)`,
			table).Scan(&exists)
		if err != nil || !exists {
			t.Fatalf("tabla %s no existe (err=%v)", table, err)
		}
	}
}
