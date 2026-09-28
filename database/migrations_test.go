package database

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func testMigrationLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newMigrationTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// TestMigrationManagerAppliesRealMigrations runs the actual migration files
// shipped in the repo's migrations/ directory against a fresh database, since
// that's the surface every other test's fixture schema quietly assumes stays
// correct.
func TestMigrationManagerAppliesRealMigrations(t *testing.T) {
	db := newMigrationTestDB(t)
	manager := &MigrationManager{db: db, logger: testMigrationLogger(), migrationsPath: "../migrations"}
	ctx := context.Background()

	if err := manager.ApplyMigrations(ctx); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	for _, table := range []string{"categories", "expenses", "classification_rules", "audit_log"} {
		var name string
		if err := db.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name); err != nil {
			t.Errorf("expected table %q to exist after migrations: %v", table, err)
		}
	}

	count, err := manager.GetAppliedMigrationCount(ctx)
	if err != nil {
		t.Fatalf("GetAppliedMigrationCount failed: %v", err)
	}
	entries, err := os.ReadDir("../migrations")
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	if count != len(entries) {
		t.Errorf("expected %d applied migrations (one per file), got %d", len(entries), count)
	}

	// Re-applying must be a no-op: already-applied migrations are skipped.
	if err := manager.ApplyMigrations(ctx); err != nil {
		t.Fatalf("second ApplyMigrations failed: %v", err)
	}
	secondCount, err := manager.GetAppliedMigrationCount(ctx)
	if err != nil || secondCount != count {
		t.Errorf("expected idempotent re-run to leave count at %d, got %d (err=%v)", count, secondCount, err)
	}
}

func TestMigrationManagerToleratesMissingDirectory(t *testing.T) {
	db := newMigrationTestDB(t)
	manager := &MigrationManager{db: db, logger: testMigrationLogger(), migrationsPath: filepath.Join(t.TempDir(), "does-not-exist")}

	if err := manager.ApplyMigrations(context.Background()); err != nil {
		t.Fatalf("expected a missing migrations directory to be tolerated, got: %v", err)
	}
	count, err := manager.GetAppliedMigrationCount(context.Background())
	if err != nil || count != 0 {
		t.Fatalf("expected zero applied migrations, got count=%d err=%v", count, err)
	}
}

func TestMigrationManagerRollsBackOnInvalidSQL(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "001_bad.sql"), []byte("NOT VALID SQL;"), 0o644); err != nil {
		t.Fatalf("write bad migration: %v", err)
	}
	db := newMigrationTestDB(t)
	manager := &MigrationManager{db: db, logger: testMigrationLogger(), migrationsPath: dir}

	if err := manager.ApplyMigrations(context.Background()); err == nil {
		t.Fatal("expected invalid migration SQL to return an error")
	}
	count, err := manager.GetAppliedMigrationCount(context.Background())
	if err != nil || count != 0 {
		t.Fatalf("expected the failed migration to not be recorded, got count=%d err=%v", count, err)
	}
}

func TestMigrationManagerIgnoresNonSQLFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("not a migration"), 0o644); err != nil {
		t.Fatalf("write non-sql file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "001_create.sql"), []byte("CREATE TABLE widgets (id INTEGER PRIMARY KEY);"), 0o644); err != nil {
		t.Fatalf("write migration: %v", err)
	}
	db := newMigrationTestDB(t)
	manager := &MigrationManager{db: db, logger: testMigrationLogger(), migrationsPath: dir}

	if err := manager.ApplyMigrations(context.Background()); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}
	count, err := manager.GetAppliedMigrationCount(context.Background())
	if err != nil || count != 1 {
		t.Fatalf("expected exactly 1 applied migration (README.md ignored), got count=%d err=%v", count, err)
	}
}
