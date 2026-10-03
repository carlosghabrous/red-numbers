package upload

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/ghab/red-numbers/domains/categories"
	"github.com/ghab/red-numbers/domains/classification"
	"github.com/ghab/red-numbers/domains/expenses"
	_ "github.com/mattn/go-sqlite3"
)

func TestHasPersistence(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	classifyOnly := NewService(logger, NewCSVParser(), classification.NewClassifier(), nil, nil, nil)
	if classifyOnly.HasPersistence() {
		t.Error("expected HasPersistence to be false without an expense repository")
	}
}

func newClassifyTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	_, err = db.Exec(`
		CREATE TABLE categories (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT UNIQUE NOT NULL, display_name TEXT NOT NULL, created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP);
		CREATE TABLE expenses (id INTEGER PRIMARY KEY AUTOINCREMENT, date DATE NOT NULL, description TEXT NOT NULL, amount REAL NOT NULL, balance REAL NOT NULL, fingerprint TEXT, category_id INTEGER, confidence_level TEXT DEFAULT 'low', imported_at TIMESTAMP NOT NULL, corrected_at TIMESTAMP);
		CREATE TABLE audit_log (id INTEGER PRIMARY KEY AUTOINCREMENT, timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, action TEXT NOT NULL, expense_id INTEGER, old_value TEXT, new_value TEXT, details TEXT);
		INSERT INTO categories (name, display_name) VALUES ('supermercado', 'Supermercado'), ('ocio', 'Ocio'), ('income', 'Income');`)
	if err != nil {
		db.Close()
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// TestClassifyExpensesAppliesLearnedCorrections reproduces the reported bug:
// a previously-corrected expense's category should apply to a newly
// imported expense with the same description, instead of only relying on
// the keyword classifier every time a new file is uploaded.
func TestClassifyExpensesAppliesLearnedCorrections(t *testing.T) {
	db := newClassifyTestDB(t)
	expenseRepo := expenses.NewRepository(db)
	categoryRepo := categories.NewRepository(db)
	ctx := context.Background()

	// A past import classified this as supermercado (by keyword guess, say),
	// and the user corrected it to ocio.
	existing := expenses.Expense{Date: time.Now(), Description: "AMAZON PRIME VIDEO", Amount: -12, Balance: 1, CategoryID: 1, ImportedAt: time.Now()}
	if err := expenseRepo.Create(ctx, &existing); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if _, err := expenseRepo.UpdateCategory(ctx, existing.ID, 2); err != nil { // 2 = ocio
		t.Fatalf("UpdateCategory failed: %v", err)
	}

	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), NewCSVParser(), classification.NewClassifier(), expenseRepo, categoryRepo, classification.NewRepository(db))

	newExpenses := []expenses.Expense{
		{Date: time.Now(), Description: "Amazon Prime Video", Amount: -12, Balance: 1, ImportedAt: time.Now()}, // same description, different case/accents
		{Date: time.Now(), Description: "Totally Unrelated Merchant", Amount: -5, Balance: 1, ImportedAt: time.Now()},
	}
	results, err := service.ClassifyExpenses(ctx, newExpenses)
	if err != nil {
		t.Fatalf("ClassifyExpenses failed: %v", err)
	}

	if results[0].CategoryName != "ocio" || results[0].Confidence != "high" {
		t.Fatalf("expected the learned correction (ocio/high) applied to the matching new row, got %+v", results[0])
	}
	if newExpenses[0].CategoryID != 2 {
		t.Fatalf("expected the new expense's CategoryID set to ocio's id (2), got %d", newExpenses[0].CategoryID)
	}
	if results[1].CategoryName == "ocio" {
		t.Fatalf("expected the unrelated merchant to fall back to the keyword classifier, not inherit ocio: %+v", results[1])
	}
}

// TestClassifyExpensesIncomeRuleOverridesLearnedCategory confirms the
// unconditional positive-amount-is-income rule still wins even when a
// matching description was previously corrected to something else.
func TestClassifyExpensesIncomeRuleOverridesLearnedCategory(t *testing.T) {
	db := newClassifyTestDB(t)
	expenseRepo := expenses.NewRepository(db)
	categoryRepo := categories.NewRepository(db)
	ctx := context.Background()

	existing := expenses.Expense{Date: time.Now(), Description: "REFUND MERCHANT", Amount: -10, Balance: 1, CategoryID: 1, ImportedAt: time.Now()}
	if err := expenseRepo.Create(ctx, &existing); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if _, err := expenseRepo.UpdateCategory(ctx, existing.ID, 2); err != nil { // corrected to ocio
		t.Fatalf("UpdateCategory failed: %v", err)
	}

	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), NewCSVParser(), classification.NewClassifier(), expenseRepo, categoryRepo, classification.NewRepository(db))

	newExpenses := []expenses.Expense{
		{Date: time.Now(), Description: "Refund Merchant", Amount: 10, Balance: 1, ImportedAt: time.Now()}, // positive amount this time
	}
	results, err := service.ClassifyExpenses(ctx, newExpenses)
	if err != nil {
		t.Fatalf("ClassifyExpenses failed: %v", err)
	}
	if results[0].CategoryName != "income" {
		t.Fatalf("expected the income rule to override the learned ocio category, got %+v", results[0])
	}
}
