package expenses

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ghab/red-numbers/domains/categories"
	"github.com/ghab/red-numbers/platform"
	_ "github.com/mattn/go-sqlite3"
)

func newDashboardTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	_, err = db.Exec(`
		CREATE TABLE categories (id INTEGER PRIMARY KEY, name TEXT, display_name TEXT, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
		CREATE TABLE expenses (id INTEGER PRIMARY KEY, date DATE, description TEXT, amount REAL, balance REAL, fingerprint TEXT, category_id INTEGER, confidence_level TEXT, imported_at TIMESTAMP, corrected_at TIMESTAMP);
		CREATE TABLE audit_log (id INTEGER PRIMARY KEY, timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP, action TEXT, expense_id INTEGER, old_value TEXT, new_value TEXT, details TEXT);
		INSERT INTO categories (id, name, display_name) VALUES (1, 'casa', 'Casa');`)
	if err != nil {
		db.Close()
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newTestHandler(db *sql.DB) *Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewHandler(platform.Dependencies{Logger: logger}, NewService(NewRepository(db), categories.NewRepository(db)))
}

func TestHandlePostDeleteAllRequiresConfirmation(t *testing.T) {
	handler := newTestHandler(newDashboardTestDB(t))

	request := httptest.NewRequest(http.MethodPost, "/expenses/delete-all", strings.NewReader("confirm=nope"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.HandlePostDeleteAll(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 without correct confirmation, got %d", response.Code)
	}
}

func TestHandlePostDeleteAllRemovesExpenses(t *testing.T) {
	db := newDashboardTestDB(t)
	if _, err := db.Exec(`INSERT INTO expenses (date, description, amount, balance, category_id, confidence_level, imported_at) VALUES ('2026-01-01','Test',1,1,1,'low','2026-01-01')`); err != nil {
		t.Fatalf("seed expense: %v", err)
	}
	handler := newTestHandler(db)

	request := httptest.NewRequest(http.MethodPost, "/expenses/delete-all", strings.NewReader("confirm=delete-all"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.HandlePostDeleteAll(response, request)

	if response.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect after deletion, got %d", response.Code)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM expenses`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("expected all expenses deleted, count=%d err=%v", count, err)
	}
}

func TestHandlePostDeleteAllReportsDatabaseError(t *testing.T) {
	db := newDashboardTestDB(t)
	handler := newTestHandler(db)
	db.Close() // force any subsequent query to fail

	request := httptest.NewRequest(http.MethodPost, "/expenses/delete-all", strings.NewReader("confirm=delete-all"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.HandlePostDeleteAll(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected the generic error page for a DB failure, got %d", response.Code)
	}
}

func TestHandleGetExpenseDetailRejectsInvalidID(t *testing.T) {
	handler := newTestHandler(newDashboardTestDB(t))

	tests := []string{"abc", "-1", "0"}
	for _, id := range tests {
		request := httptest.NewRequest(http.MethodGet, "/expenses/"+id, nil)
		request.SetPathValue("id", id)
		response := httptest.NewRecorder()
		handler.HandleGetExpenseDetail(response, request)
		if response.Code != http.StatusBadRequest {
			t.Errorf("id=%q: expected 400, got %d", id, response.Code)
		}
	}
}

func TestHandleGetExpenseDetailReturnsNotFoundForMissingExpense(t *testing.T) {
	handler := newTestHandler(newDashboardTestDB(t))

	request := httptest.NewRequest(http.MethodGet, "/expenses/999", nil)
	request.SetPathValue("id", "999")
	response := httptest.NewRecorder()
	handler.HandleGetExpenseDetail(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing expense, got %d", response.Code)
	}
}

func TestHandleGetExpenseDetailReportsDatabaseError(t *testing.T) {
	db := newDashboardTestDB(t)
	handler := newTestHandler(db)
	db.Close()

	request := httptest.NewRequest(http.MethodGet, "/expenses/1", nil)
	request.SetPathValue("id", "1")
	response := httptest.NewRecorder()
	handler.HandleGetExpenseDetail(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for a DB failure, got %d", response.Code)
	}
}

func TestHandleGetDashboardReportsDatabaseError(t *testing.T) {
	db := newDashboardTestDB(t)
	handler := newTestHandler(db)
	db.Close()

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	handler.HandleGetDashboard(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected the generic error page for a DB failure, got %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "Something went wrong") {
		t.Fatalf("expected renderError body, got %s", response.Body.String())
	}
}

func TestHandleGetDashboardPaginatesResults(t *testing.T) {
	db := newDashboardTestDB(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < dashboardPageSize+5; i++ {
		if _, err := db.Exec(`INSERT INTO expenses (date, description, amount, balance, category_id, confidence_level, imported_at) VALUES (?,?,?,?,?,?,?)`,
			base.AddDate(0, 0, i), "Expense "+strconv.Itoa(i), -1.0, 1.0, 1, "low", base); err != nil {
			t.Fatalf("seed expense %d: %v", i, err)
		}
	}
	handler := newTestHandler(db)

	request := httptest.NewRequest(http.MethodGet, "/?page=2", nil)
	response := httptest.NewRecorder()
	handler.HandleGetDashboard(response, request)

	body := response.Body.String()
	if !strings.Contains(body, `class="pagination"`) || !strings.Contains(body, "Previous") {
		t.Fatalf("expected pagination controls with a Previous link on page 2, got %s", body)
	}
}

func TestCategoryFormMessageBranches(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  string
	}{
		{"added", "category_added=Mascotas", "Mascotas"},
		{"empty name error", "category_error=empty_name", "cannot be empty"},
		{"duplicate error", "category_error=duplicate", "already exists"},
		{"too long error", "category_error=too_long", "40 characters"},
		{"failed error", "category_error=failed", "Failed to add category"},
		{"no message", "", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			query, _ := url.ParseQuery(test.query)
			got := categoryFormMessage(query)
			if test.want == "" {
				if got != "" {
					t.Errorf("expected empty message, got %q", got)
				}
				return
			}
			if !strings.Contains(got, test.want) {
				t.Errorf("expected message to contain %q, got %q", test.want, got)
			}
		})
	}
}

func TestServiceDeleteAllDelegatesToRepository(t *testing.T) {
	db := newDashboardTestDB(t)
	if _, err := db.Exec(`INSERT INTO expenses (date, description, amount, balance, category_id, confidence_level, imported_at) VALUES ('2026-01-01','Test',1,1,1,'low','2026-01-01')`); err != nil {
		t.Fatalf("seed expense: %v", err)
	}
	service := NewService(NewRepository(db), categories.NewRepository(db))
	if err := service.DeleteAll(context.Background()); err != nil {
		t.Fatalf("DeleteAll failed: %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM expenses`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("expected 0 expenses after DeleteAll, got %d (err=%v)", count, err)
	}
}
