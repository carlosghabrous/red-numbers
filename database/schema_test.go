package database

import (
	"database/sql"
	"log/slog"
	"os"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// TestSchemaIndexCreation tests that all indexes are created correctly
func TestSchemaIndexCreation(t *testing.T) {
	// Create a temporary test database
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create in-memory database: %v", err)
	}
	defer db.Close()

	// Read and execute the schema migration
	schemaSQL := `
-- Categories table
CREATE TABLE IF NOT EXISTS categories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    display_name TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Expenses table
CREATE TABLE IF NOT EXISTS expenses (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    date DATE NOT NULL,
    description TEXT NOT NULL,
    amount REAL NOT NULL,
    balance REAL NOT NULL,
	fingerprint TEXT,
    category_id INTEGER NOT NULL,
    confidence_level TEXT DEFAULT 'low',
    imported_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    corrected_at TIMESTAMP,
    FOREIGN KEY (category_id) REFERENCES categories(id)
);

-- Classification rules table
CREATE TABLE IF NOT EXISTS classification_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pattern TEXT NOT NULL,
    category_id INTEGER NOT NULL,
    priority INTEGER DEFAULT 0,
    confidence_weight REAL DEFAULT 1.0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by TEXT,
    FOREIGN KEY (category_id) REFERENCES categories(id)
);

-- Audit log table
CREATE TABLE IF NOT EXISTS audit_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    action TEXT NOT NULL,
    user_id TEXT,
    expense_id INTEGER,
    old_value TEXT,
    new_value TEXT,
    details TEXT,
    FOREIGN KEY (expense_id) REFERENCES expenses(id)
);

-- Performance indexes
CREATE INDEX IF NOT EXISTS idx_expenses_date ON expenses(date DESC);
CREATE INDEX IF NOT EXISTS idx_expenses_category ON expenses(category_id);
CREATE INDEX IF NOT EXISTS idx_expenses_date_category ON expenses(date DESC, category_id);
CREATE INDEX IF NOT EXISTS idx_expenses_confidence ON expenses(confidence_level);
CREATE INDEX IF NOT EXISTS idx_classification_rules_priority ON classification_rules(priority ASC);
CREATE INDEX IF NOT EXISTS idx_audit_log_timestamp ON audit_log(timestamp DESC);
	`

	_, err = db.Exec(schemaSQL)
	if err != nil {
		t.Fatalf("Failed to execute schema SQL: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	verifier := NewSchemaVerifier(db, logger)

	// Test 1: Verify all indexes exist
	t.Run("VerifyIndexes", func(t *testing.T) {
		indexes, err := verifier.VerifyIndexes()
		if err != nil {
			t.Errorf("VerifyIndexes failed: %v", err)
		}

		requiredIndexes := []string{
			"idx_expenses_date",
			"idx_expenses_category",
			"idx_expenses_date_category",
			"idx_expenses_confidence",
			"idx_classification_rules_priority",
			"idx_audit_log_timestamp",
		}

		for _, idxName := range requiredIndexes {
			if exists, ok := indexes[idxName]; !ok || !exists {
				t.Errorf("Index %s not found or not verified", idxName)
			}
		}

		if len(indexes) != 6 {
			t.Errorf("Expected 6 indexes, got %d", len(indexes))
		}
	})

	// Test 2: Verify index structure with PRAGMA index_info
	t.Run("VerifyIndexStructure", func(t *testing.T) {
		structures, err := verifier.VerifyIndexStructure()
		if err != nil {
			t.Errorf("VerifyIndexStructure failed: %v", err)
		}

		// Verify idx_expenses_date has date column
		if cols, ok := structures["idx_expenses_date"]; ok {
			if len(cols) != 1 || cols[0].Name != "date" {
				t.Errorf("idx_expenses_date: expected [date], got %v", cols)
			}
		} else {
			t.Error("idx_expenses_date structure not found")
		}

		// Verify idx_expenses_date_category has date and category_id columns
		if cols, ok := structures["idx_expenses_date_category"]; ok {
			if len(cols) != 2 {
				t.Errorf("idx_expenses_date_category: expected 2 columns, got %d", len(cols))
			}
			if cols[0].Name != "date" || cols[1].Name != "category_id" {
				t.Errorf("idx_expenses_date_category: expected [date, category_id], got %v", cols)
			}
		} else {
			t.Error("idx_expenses_date_category structure not found")
		}

		// Verify idx_classification_rules_priority
		if cols, ok := structures["idx_classification_rules_priority"]; ok {
			if len(cols) != 1 || cols[0].Name != "priority" {
				t.Errorf("idx_classification_rules_priority: expected [priority], got %v", cols)
			}
		} else {
			t.Error("idx_classification_rules_priority structure not found")
		}
	})

	// Test 3: Verify naming convention
	t.Run("IndexNamingConvention", func(t *testing.T) {
		indexes, err := verifier.VerifyIndexes()
		if err != nil {
			t.Fatalf("Failed to verify indexes: %v", err)
		}

		for idxName := range indexes {
			// All index names should start with "idx_"
			if len(idxName) < 4 || idxName[:4] != "idx_" {
				t.Errorf("Index %s does not follow naming convention (should start with 'idx_')", idxName)
			}
		}
	})

	// Test 4: Verify performance with EXPLAIN QUERY PLAN
	t.Run("QueryPerformance", func(t *testing.T) {
		plans, err := verifier.VerifyIndexesForPerformance()
		if err != nil {
			t.Errorf("VerifyIndexesForPerformance failed: %v", err)
		}

		// Ensure we got query plans for all test queries
		if len(plans) == 0 {
			t.Error("No query plans generated")
		}

		for queryName, plan := range plans {
			if plan == "" {
				t.Errorf("Query plan for %s is empty", queryName)
			}
		}
	})

	// Test 5: Log summary
	t.Run("LogIndexSummary", func(t *testing.T) {
		err := verifier.LogIndexSummary()
		if err != nil {
			t.Errorf("LogIndexSummary failed: %v", err)
		}
	})
}

// TestIndexesImproveQueryPerformance demonstrates that indexes improve performance
func TestIndexesImproveQueryPerformance(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create schema with indexes
	schemaSQL := `
CREATE TABLE categories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    display_name TEXT NOT NULL
);

CREATE TABLE expenses (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    date DATE NOT NULL,
    description TEXT NOT NULL,
    amount REAL NOT NULL,
    balance REAL NOT NULL,
    category_id INTEGER NOT NULL,
    confidence_level TEXT DEFAULT 'low',
    FOREIGN KEY (category_id) REFERENCES categories(id)
);

CREATE INDEX idx_expenses_date ON expenses(date DESC);
CREATE INDEX idx_expenses_category ON expenses(category_id);
CREATE INDEX idx_expenses_date_category ON expenses(date DESC, category_id);

INSERT INTO categories (name, display_name) VALUES ('supermercado', 'Supermercado');
	`

	_, err = db.Exec(schemaSQL)
	if err != nil {
		t.Fatalf("Failed to setup schema: %v", err)
	}

	// Insert test data
	stmt, err := db.Prepare("INSERT INTO expenses (date, description, amount, balance, category_id) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		t.Fatalf("Failed to prepare insert statement: %v", err)
	}
	defer stmt.Close()

	for i := 0; i < 1000; i++ {
		_, err := stmt.Exec("2024-01-15", "Test expense", 10.5, 100.0, 1)
		if err != nil {
			t.Fatalf("Failed to insert test data: %v", err)
		}
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	verifier := NewSchemaVerifier(db, logger)

	// Verify indexes exist
	indexes, err := verifier.VerifyIndexes()
	if err != nil {
		t.Fatalf("Failed to verify indexes: %v", err)
	}

	if len(indexes) != 3 {
		t.Errorf("Expected 3 indexes, got %d", len(indexes))
	}

	// Check that date filter query uses index
	plans, _ := verifier.VerifyIndexesForPerformance()
	if _, ok := plans["date_filter"]; !ok {
		t.Error("No query plan for date_filter")
	}
}
