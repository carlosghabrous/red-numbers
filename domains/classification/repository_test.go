package classification

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func newClassificationTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE audit_log (
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
		t.Fatalf("create audit_log table: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestRepositoryLogAndGetLogs(t *testing.T) {
	db := newClassificationTestDB(t)
	repository := NewRepository(db)
	ctx := context.Background()

	if err := repository.Log(ctx, 1, Classification{CategoryName: "supermercado", Pattern: "mercadona", Confidence: "high"}); err != nil {
		t.Fatalf("Log failed: %v", err)
	}
	if err := repository.Log(ctx, 2, Classification{CategoryName: "ocio", Pattern: "netflix", Confidence: "medium"}); err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	logs, err := repository.GetLogs(ctx)
	if err != nil {
		t.Fatalf("GetLogs failed: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(logs))
	}
	// Most recent first (id DESC).
	if logs[0].ExpenseID != 2 || logs[0].CategoryName != "ocio" {
		t.Errorf("expected most recent entry first, got %+v", logs[0])
	}
	if logs[1].Details != "pattern=mercadona;confidence=high" {
		t.Errorf("unexpected details: %q", logs[1].Details)
	}
}

func TestRepositoryGetLogsIgnoresOtherAuditActions(t *testing.T) {
	db := newClassificationTestDB(t)
	repository := NewRepository(db)
	ctx := context.Background()

	if err := repository.Log(ctx, 1, Classification{CategoryName: "casa"}); err != nil {
		t.Fatalf("Log failed: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO audit_log (action, expense_id, new_value, details) VALUES ('category_correction', 1, '2', 'manual')`); err != nil {
		t.Fatalf("seed unrelated audit entry: %v", err)
	}

	logs, err := repository.GetLogs(ctx)
	if err != nil {
		t.Fatalf("GetLogs failed: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected only classification entries, got %d", len(logs))
	}
}
