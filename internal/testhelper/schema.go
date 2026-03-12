package testhelper

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ApplyTestSchema ensures the full schema is present in the test database.
// It is idempotent: if the "users" table already exists the function returns
// nil immediately so repeated calls (e.g. across parallel subtests) are safe.
func ApplyTestSchema(ctx context.Context, pool *pgxpool.Pool) error {
	var exists bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = 'users'
		)
	`).Scan(&exists)
	if err != nil {
		return fmt.Errorf("ApplyTestSchema: check schema existence: %w", err)
	}
	if exists {
		return nil
	}

	sql, err := readMigrationSQL()
	if err != nil {
		return fmt.Errorf("ApplyTestSchema: %w", err)
	}

	if _, err := pool.Exec(ctx, sql); err != nil {
		return fmt.Errorf("ApplyTestSchema: exec migration: %w", err)
	}
	return nil
}

// readMigrationSQL locates migrations/000001_init_schema.up.sql relative to
// this source file using runtime.Caller(0).
//
// File layout:
//
//	internal/testhelper/schema.go  ← runtime.Caller(0)
//	internal/testhelper/          ← filepath.Dir once
//	internal/                     ← filepath.Dir twice
//	<repo root>/                  ← filepath.Dir three times
//	migrations/000001_init_schema.up.sql
func readMigrationSQL() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("readMigrationSQL: runtime.Caller returned no file info")
	}

	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
	migPath := filepath.Join(repoRoot, "migrations", "000001_init_schema.up.sql")

	data, err := os.ReadFile(migPath)
	if err != nil {
		return "", fmt.Errorf("readMigrationSQL: read %s: %w", migPath, err)
	}
	return string(data), nil
}
