package expenses

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

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
			fingerprint TEXT,
			category_id INTEGER,
			confidence_level TEXT DEFAULT 'low',
			imported_at TIMESTAMP NOT NULL,
			corrected_at TIMESTAMP
		);
		CREATE TABLE audit_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			action TEXT NOT NULL,
			expense_id INTEGER,
			old_value TEXT,
			new_value TEXT,
			details TEXT
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
	repository := NewRepository(db)
	baseDate := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	expenseList := []Expense{
		{Date: baseDate.AddDate(0, 0, 2), Description: "Zeta", Amount: 20, Balance: 1, CategoryID: 1, ImportedAt: baseDate},
		{Date: baseDate.AddDate(0, 0, 1), Description: "Alpha", Amount: 5, Balance: 1, CategoryID: 2, ImportedAt: baseDate},
		{Date: baseDate, Description: "Beta", Amount: 10, Balance: 1, CategoryID: 1, ImportedAt: baseDate},
	}
	if err := repository.CreateBatch(context.Background(), expenseList); err != nil {
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
	repository := NewRepository(db)
	baseDate := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	expenseList := []Expense{
		{Date: baseDate, Description: "Casa", Amount: 30, Balance: 1, CategoryID: 1, ImportedAt: baseDate},
		{Date: baseDate.AddDate(0, 0, 1), Description: "Ocio", Amount: 10, Balance: 1, CategoryID: 2, ImportedAt: baseDate},
		{Date: baseDate.AddDate(0, 0, 2), Description: "Casa later", Amount: 20, Balance: 1, CategoryID: 1, ImportedAt: baseDate},
	}
	if err := repository.CreateBatch(context.Background(), expenseList); err != nil {
		t.Fatalf("CreateBatch failed: %v", err)
	}

	start := baseDate
	end := baseDate.AddDate(0, 0, 2)
	filtered, err := repository.GetByFilterOptions(context.Background(), FilterOptions{
		SortBy: "date", SortDirection: "asc", CategoryIDs: []int64{1, 2}, StartDate: &start, EndDate: &end,
	})
	if err != nil {
		t.Fatalf("GetByFilterOptions failed: %v", err)
	}
	if len(filtered) != 2 || filtered[0].Description != "Casa" || filtered[1].Description != "Ocio" {
		t.Fatalf("expected date-bounded results with OR category filter, got %+v", filtered)
	}

	filtered, err = repository.GetByFilterOptions(context.Background(), FilterOptions{CategoryIDs: []int64{1}})
	if err != nil {
		t.Fatalf("single category filter failed: %v", err)
	}
	if len(filtered) != 2 {
		t.Fatalf("expected 2 casa expenses, got %d", len(filtered))
	}
}

func TestExpenseRepositoryPagination(t *testing.T) {
	db := newExpenseTestDB(t)
	repository := NewRepository(db)
	baseDate := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	for index := 0; index < 3; index++ {
		expense := Expense{Date: baseDate.AddDate(0, 0, index), Description: fmt.Sprintf("Expense %d", index), Amount: float64(index), Balance: 1, ImportedAt: baseDate}
		if err := repository.Create(context.Background(), &expense); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}
	page, err := repository.GetByFilterOptions(context.Background(), FilterOptions{SortBy: "date", SortDirection: "asc", Limit: 2, Offset: 1})
	if err != nil {
		t.Fatalf("GetByFilterOptions failed: %v", err)
	}
	if len(page) != 2 || page[0].Description != "Expense 1" || page[1].Description != "Expense 2" {
		t.Fatalf("unexpected page results: %+v", page)
	}
	count, err := repository.CountByFilterOptions(context.Background(), FilterOptions{})
	if err != nil || count != 3 {
		t.Fatalf("expected total count 3, got %d (err=%v)", count, err)
	}
}

func TestExpenseRepositorySkipsDuplicateRecords(t *testing.T) {
	db := newExpenseTestDB(t)
	repository := NewRepository(db)
	expense := Expense{Date: time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC), Description: "Repeated", Amount: 12.50, Balance: 100, ImportedAt: time.Now()}
	inserted, err := repository.CreateBatchWithCount(context.Background(), []Expense{expense})
	if err != nil || inserted != 1 {
		t.Fatalf("expected first insert, got count=%d err=%v", inserted, err)
	}
	duplicate := Expense{Date: expense.Date, Description: expense.Description, Amount: expense.Amount, Balance: expense.Balance, ImportedAt: time.Now()}
	inserted, err = repository.CreateBatchWithCount(context.Background(), []Expense{duplicate})
	if err != nil || inserted != 0 {
		t.Fatalf("expected duplicate to be skipped, got count=%d err=%v", inserted, err)
	}
	count, err := repository.CountByFilterOptions(context.Background(), FilterOptions{})
	if err != nil || count != 1 {
		t.Fatalf("expected one stored expense, got count=%d err=%v", count, err)
	}
}

func TestExpenseRepositoryDateFilterNormalizesStoredTimestamps(t *testing.T) {
	db := newExpenseTestDB(t)
	repository := NewRepository(db)
	expenseList := []Expense{
		{Date: time.Date(2026, 1, 31, 12, 0, 0, 0, time.UTC), Description: "January", Amount: 1, Balance: 1, ImportedAt: time.Now()},
		{Date: time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC), Description: "February", Amount: 1, Balance: 1, ImportedAt: time.Now()},
	}
	if err := repository.CreateBatch(context.Background(), expenseList); err != nil {
		t.Fatalf("CreateBatch failed: %v", err)
	}
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	filtered, err := repository.GetByFilterOptions(context.Background(), FilterOptions{StartDate: &start, EndDate: &end})
	if err != nil || len(filtered) != 1 || filtered[0].Description != "January" {
		t.Fatalf("expected January only, got %+v err=%v", filtered, err)
	}
}

func TestExpenseRepositoryDeleteAll(t *testing.T) {
	db := newExpenseTestDB(t)
	repository := NewRepository(db)
	expense := Expense{Date: time.Now(), Description: "To delete", Amount: 1, Balance: 1, ImportedAt: time.Now()}
	if err := repository.Create(context.Background(), &expense); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if err := repository.DeleteAll(context.Background()); err != nil {
		t.Fatalf("DeleteAll failed: %v", err)
	}
	count, err := repository.CountByFilterOptions(context.Background(), FilterOptions{})
	if err != nil || count != 0 {
		t.Fatalf("expected empty database, got count=%d err=%v", count, err)
	}
}

func TestExpenseRepositoryUpdateCategory(t *testing.T) {
	db := newExpenseTestDB(t)
	repository := NewRepository(db)
	expense := Expense{Date: time.Now(), Description: "Needs correction", Amount: 1, Balance: 1, ImportedAt: time.Now(), CategoryID: 1}
	if err := repository.Create(context.Background(), &expense); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if err := repository.UpdateCategory(context.Background(), expense.ID, 2); err != nil {
		t.Fatalf("UpdateCategory failed: %v", err)
	}
	updated, err := repository.GetByID(context.Background(), expense.ID)
	if err != nil || updated == nil || updated.CategoryID != 2 || updated.ConfidenceLevel != "high" {
		t.Fatalf("unexpected updated expense: %+v err=%v", updated, err)
	}
	var action string
	if err := db.QueryRow(`SELECT action FROM audit_log WHERE expense_id = ?`, expense.ID).Scan(&action); err != nil || action != "category_correction" {
		t.Fatalf("expected correction audit entry, action=%q err=%v", action, err)
	}
}

func TestExpenseRepositoryBackfillRemovesExistingDuplicates(t *testing.T) {
	db := newExpenseTestDB(t)
	date := time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC)
	_, err := db.Exec(`INSERT INTO expenses (date, description, amount, balance, imported_at) VALUES (?, ?, ?, ?, ?), (?, ?, ?, ?, ?)`,
		date, "Legacy duplicate", 5, 10, date, date, "Legacy duplicate", 5, 10, date)
	if err != nil {
		t.Fatalf("insert legacy duplicates: %v", err)
	}
	repository := NewRepository(db)
	if err := repository.BackfillFingerprints(context.Background()); err != nil {
		t.Fatalf("BackfillFingerprints failed: %v", err)
	}
	count, err := repository.CountByFilterOptions(context.Background(), FilterOptions{})
	if err != nil || count != 1 {
		t.Fatalf("expected one deduplicated record, got count=%d err=%v", count, err)
	}
}

func TestExpenseRepositoryCreateAndGetAll(t *testing.T) {
	db := newExpenseTestDB(t)
	repository := NewRepository(db)
	importedAt := time.Date(2026, time.January, 5, 12, 0, 0, 0, time.UTC)
	expenseList := []Expense{
		{Date: importedAt, Description: "Older", Amount: 10, Balance: 100, ImportedAt: importedAt},
		{Date: importedAt.AddDate(0, 0, 1), Description: "Newer", Amount: -5, Balance: 95, ImportedAt: importedAt},
	}

	if err := repository.CreateBatch(context.Background(), expenseList); err != nil {
		t.Fatalf("CreateBatch failed: %v", err)
	}
	if expenseList[0].ID == 0 || expenseList[1].ID == 0 {
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
	repository := NewRepository(db)
	expenseList := []Expense{
		{Date: time.Now(), Description: "Valid", Amount: 10, Balance: 100, ImportedAt: time.Now()},
		{Date: time.Now(), Amount: 5, Balance: 105, ImportedAt: time.Now()},
	}

	if err := repository.CreateBatch(context.Background(), expenseList); err == nil {
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
