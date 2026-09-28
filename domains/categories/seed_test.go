package categories

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestSeedDefaultCategoriesInsertsAllAndIsIdempotent(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE categories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		display_name TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatalf("create categories table: %v", err)
	}

	ctx := context.Background()
	if err := SeedDefaultCategories(ctx, db); err != nil {
		t.Fatalf("SeedDefaultCategories failed: %v", err)
	}

	repository := NewRepository(db)
	first, err := repository.GetAllCategories(ctx)
	if err != nil {
		t.Fatalf("GetAllCategories failed: %v", err)
	}
	if len(first) != 8 { // 7 spending categories + income
		t.Fatalf("expected 8 seeded categories, got %d", len(first))
	}
	income, err := repository.GetByName(ctx, "income")
	if err != nil || income == nil {
		t.Fatalf("expected income category to be seeded, got %+v err=%v", income, err)
	}

	// Running it again must not duplicate or error (INSERT OR IGNORE).
	if err := SeedDefaultCategories(ctx, db); err != nil {
		t.Fatalf("second SeedDefaultCategories failed: %v", err)
	}
	second, err := repository.GetAllCategories(ctx)
	if err != nil || len(second) != len(first) {
		t.Fatalf("expected idempotent seeding, first=%d second=%d err=%v", len(first), len(second), err)
	}
}
