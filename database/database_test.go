package database

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

// withRepoRootCWD temporarily changes the working directory to the repo root
// (one level up from this package), since Initialize resolves its
// "migrations" directory relative to the process's CWD, and running `go
// test` sets CWD to the package directory. The original directory is
// restored after the test.
func withRepoRootCWD(t *testing.T) {
	t.Helper()
	original, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	if err := os.Chdir(".."); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(original); err != nil {
			t.Fatalf("failed to restore working directory: %v", err)
		}
	})
}

func TestInitializeAppliesMigrationsAndSeeds(t *testing.T) {
	withRepoRootCWD(t)
	dbPath := filepath.Join(t.TempDir(), "test.db")
	logger := testMigrationLogger()

	db, err := Initialize(context.Background(), Config{DBPath: dbPath}, logger)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer Close(db, logger)

	var categoryCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM categories`).Scan(&categoryCount); err != nil {
		t.Fatalf("query categories: %v", err)
	}
	if categoryCount != 8 { // 7 spending categories + income
		t.Errorf("expected 8 seeded categories, got %d", categoryCount)
	}

	if got := db.Stats().MaxOpenConnections; got != 10 {
		t.Errorf("expected default MaxOpenConnections 10, got %d", got)
	}
}

func TestInitializeBackfillsIncomeCategoryOnReopen(t *testing.T) {
	withRepoRootCWD(t)
	dbPath := filepath.Join(t.TempDir(), "test.db")
	logger := testMigrationLogger()

	db, err := Initialize(context.Background(), Config{DBPath: dbPath}, logger)
	if err != nil {
		t.Fatalf("first Initialize failed: %v", err)
	}
	var incomeCategoryID int64
	if err := db.QueryRow(`SELECT id FROM categories WHERE name = 'income'`).Scan(&incomeCategoryID); err != nil {
		t.Fatalf("expected income category to exist: %v", err)
	}
	var supermercadoID int64
	if err := db.QueryRow(`SELECT id FROM categories WHERE name = 'supermercado'`).Scan(&supermercadoID); err != nil {
		t.Fatalf("expected supermercado category to exist: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO expenses (date, description, amount, balance, category_id, confidence_level, imported_at, fingerprint)
		VALUES ('2026-01-01', 'Refund', 25.0, 100, ?, 'low', '2026-01-01', 'fp-reopen-test')`, supermercadoID); err != nil {
		t.Fatalf("insert positive-amount expense: %v", err)
	}
	if err := Close(db, logger); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Reopening (simulating an app restart) must retroactively move the
	// positive-amount row into the income category.
	reopened, err := Initialize(context.Background(), Config{DBPath: dbPath}, logger)
	if err != nil {
		t.Fatalf("second Initialize failed: %v", err)
	}
	defer Close(reopened, logger)

	var categoryID int64
	if err := reopened.QueryRow(`SELECT category_id FROM expenses WHERE fingerprint = 'fp-reopen-test'`).Scan(&categoryID); err != nil {
		t.Fatalf("query backfilled expense: %v", err)
	}
	if categoryID != incomeCategoryID {
		t.Errorf("expected positive-amount expense moved to income (id=%d), got category_id=%d", incomeCategoryID, categoryID)
	}
}

func TestCloseHandlesNilDB(t *testing.T) {
	if err := Close(nil, testMigrationLogger()); err != nil {
		t.Fatalf("expected Close(nil, ...) to be a no-op, got: %v", err)
	}
}

func TestCloseClosesRealConnection(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := Close(db, testMigrationLogger()); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if err := db.Ping(); err == nil {
		t.Fatal("expected database to be closed after Close")
	}
}
