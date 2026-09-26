package repositories

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/ghab/red-numbers/models"
	_ "github.com/mattn/go-sqlite3"
)

func newExpenseTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	_, err = db.Exec(`
		CREATE TABLE categories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL,
			display_name TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE expenses (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			date DATE NOT NULL,
			description TEXT NOT NULL CHECK (description <> ''),
			amount REAL NOT NULL,
			balance REAL NOT NULL,
			category_id INTEGER,
			confidence_level TEXT DEFAULT 'low',
			imported_at TIMESTAMP NOT NULL,
			corrected_at TIMESTAMP
		)`)
	if err != nil {
		db.Close()
		t.Fatalf("create expenses table: %v", err)
	}
	_, err = db.Exec(`INSERT INTO categories (name, display_name) VALUES ('casa', 'Casa'), ('ocio', 'Ocio')`)
	if err != nil {
		db.Close()
		t.Fatalf("seed categories: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestExpenseRepositoryGetByFiltersSorting(t *testing.T) {
	db := newExpenseTestDB(t)
	repository := NewExpenseRepository(db)
	baseDate := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	expenses := []models.Expense{
		{Date: baseDate.AddDate(0, 0, 2), Description: "Zeta", Amount: 20, Balance: 1, CategoryID: 1, ImportedAt: baseDate},
		{Date: baseDate.AddDate(0, 0, 1), Description: "Alpha", Amount: 5, Balance: 1, CategoryID: 2, ImportedAt: baseDate},
		{Date: baseDate, Description: "Beta", Amount: 10, Balance: 1, CategoryID: 1, ImportedAt: baseDate},
	}
	if err := repository.CreateBatch(context.Background(), expenses); err != nil {
		t.Fatalf("CreateBatch failed: %v", err)
	}

	tests := []struct {
		name      string
		sortBy    string
		direction string
		wantFirst string
	}{
		{"date ascending", "date", "asc", "Beta"},
		{"amount descending", "amount", "desc", "Zeta"},
		{"description ascending", "description", "asc", "Alpha"},
		{"category ascending", "category", "asc", "Zeta"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stored, err := repository.GetByFilters(context.Background(), test.sortBy, test.direction)
			if err != nil {
				t.Fatalf("GetByFilters failed: %v", err)
			}
			if stored[0].Description != test.wantFirst {
				t.Errorf("expected first expense %q, got %q", test.wantFirst, stored[0].Description)
			}
		})
	}
}

func TestExpenseRepositoryGetByFilterOptions(t *testing.T) {
	db := newExpenseTestDB(t)
	repository := NewExpenseRepository(db)
	baseDate := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	expenses := []models.Expense{
		{Date: baseDate, Description: "Casa", Amount: 30, Balance: 1, CategoryID: 1, ImportedAt: baseDate},
		{Date: baseDate.AddDate(0, 0, 1), Description: "Ocio", Amount: 10, Balance: 1, CategoryID: 2, ImportedAt: baseDate},
		{Date: baseDate.AddDate(0, 0, 2), Description: "Casa later", Amount: 20, Balance: 1, CategoryID: 1, ImportedAt: baseDate},
	}
	if err := repository.CreateBatch(context.Background(), expenses); err != nil {
		t.Fatalf("CreateBatch failed: %v", err)
	}

	start := baseDate
	end := baseDate.AddDate(0, 0, 2)
	filtered, err := repository.GetByFilterOptions(context.Background(), ExpenseFilterOptions{
		SortBy: "date", SortDirection: "asc", CategoryIDs: []int64{1, 2}, StartDate: &start, EndDate: &end,
	})
	if err != nil {
		t.Fatalf("GetByFilterOptions failed: %v", err)
	}
	if len(filtered) != 2 || filtered[0].Description != "Casa" || filtered[1].Description != "Ocio" {
		t.Fatalf("expected date-bounded results with OR category filter, got %+v", filtered)
	}

	filtered, err = repository.GetByFilterOptions(context.Background(), ExpenseFilterOptions{CategoryIDs: []int64{1}})
	if err != nil {
		t.Fatalf("single category filter failed: %v", err)
	}
	if len(filtered) != 2 {
		t.Fatalf("expected 2 casa expenses, got %d", len(filtered))
	}
}

func TestExpenseRepositoryCreateAndGetAll(t *testing.T) {
	db := newExpenseTestDB(t)
	repository := NewExpenseRepository(db)
	importedAt := time.Date(2026, time.January, 5, 12, 0, 0, 0, time.UTC)
	expenses := []models.Expense{
		{Date: importedAt, Description: "Older", Amount: 10, Balance: 100, ImportedAt: importedAt},
		{Date: importedAt.AddDate(0, 0, 1), Description: "Newer", Amount: -5, Balance: 95, ImportedAt: importedAt},
	}

	if err := repository.CreateBatch(context.Background(), expenses); err != nil {
		t.Fatalf("CreateBatch failed: %v", err)
	}
	if expenses[0].ID == 0 || expenses[1].ID == 0 {
		t.Fatal("expected CreateBatch to assign IDs")
	}

	stored, err := repository.GetAll(context.Background())
	if err != nil {
		t.Fatalf("GetAll failed: %v", err)
	}
	if len(stored) != 2 {
		t.Fatalf("expected 2 expenses, got %d", len(stored))
	}
	if stored[0].Description != "Newer" || stored[1].Description != "Older" {
		t.Errorf("expected newest-first ordering, got %q then %q", stored[0].Description, stored[1].Description)
	}
	if stored[0].ConfidenceLevel != "low" {
		t.Errorf("expected default confidence low, got %q", stored[0].ConfidenceLevel)
	}
}

func TestExpenseRepositoryCreateBatchRollsBackOnError(t *testing.T) {
	db := newExpenseTestDB(t)
	repository := NewExpenseRepository(db)
	expenses := []models.Expense{
		{Date: time.Now(), Description: "Valid", Amount: 10, Balance: 100, ImportedAt: time.Now()},
		{Date: time.Now(), Amount: 5, Balance: 105, ImportedAt: time.Now()},
	}

	if err := repository.CreateBatch(context.Background(), expenses); err == nil {
		t.Fatal("expected CreateBatch to fail for missing description")
	}

	stored, err := repository.GetAll(context.Background())
	if err != nil {
		t.Fatalf("GetAll failed: %v", err)
	}
	if len(stored) != 0 {
		t.Fatalf("expected rollback to leave no expenses, got %d", len(stored))
	}
}
