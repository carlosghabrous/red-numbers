package expenses

import (
	"bytes"
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ghab/red-numbers/domains/categories"
	"github.com/ghab/red-numbers/platform"
	_ "github.com/mattn/go-sqlite3"
)

func TestDashboardSortPreferenceUsesQueryAndCookie(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/?sort=amount&direction=asc", nil)
	sortBy, direction := dashboardSortPreference(request)
	if sortBy != "amount" || direction != "asc" {
		t.Fatalf("expected query sort preference amount/asc, got %s/%s", sortBy, direction)
	}

	request = httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(&http.Cookie{Name: "expense_sort", Value: "description:desc"})
	sortBy, direction = dashboardSortPreference(request)
	if sortBy != "description" || direction != "desc" {
		t.Fatalf("expected cookie sort preference description/desc, got %s/%s", sortBy, direction)
	}
}

func TestDashboardFilterStateRejectsInvalidCustomRange(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/?date_filter=custom&start_date=2026-02-01&end_date=2026-01-31", nil)
	_, err := dashboardFilterStateFromRequest(request)
	if err == nil {
		t.Fatal("expected invalid custom date range to be rejected")
	}
}

func TestExpenseDetailCorrectionWorkflow(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	_, err = db.Exec(`
		CREATE TABLE categories (id INTEGER PRIMARY KEY, name TEXT, display_name TEXT, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
		CREATE TABLE expenses (id INTEGER PRIMARY KEY, date DATE, description TEXT, amount REAL, balance REAL, fingerprint TEXT, category_id INTEGER, confidence_level TEXT, imported_at TIMESTAMP, corrected_at TIMESTAMP);
		CREATE TABLE audit_log (id INTEGER PRIMARY KEY, timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP, action TEXT, expense_id INTEGER, old_value TEXT, new_value TEXT, details TEXT);
		INSERT INTO categories (id, name, display_name) VALUES (1, 'casa', 'Casa'), (2, 'ocio', 'Ocio');
		INSERT INTO expenses (id, date, description, amount, balance, category_id, confidence_level, imported_at) VALUES (1, '2026-01-05 00:00:00+00:00', 'Test expense', 10, 20, 1, 'low', '2026-01-05 00:00:00+00:00');`)
	if err != nil {
		t.Fatalf("create detail schema: %v", err)
	}
	deps := platform.Dependencies{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	service := NewService(NewRepository(db), categories.NewRepository(db))
	handler := NewHandler(deps, service)

	get := httptest.NewRequest(http.MethodGet, "/expenses/1", nil)
	get.SetPathValue("id", "1")
	getResponse := httptest.NewRecorder()
	handler.HandleGetExpenseDetail(getResponse, get)
	if getResponse.Code != http.StatusOK || !bytes.Contains(getResponse.Body.Bytes(), []byte("Test expense")) || !bytes.Contains(getResponse.Body.Bytes(), []byte("Save Changes")) {
		t.Fatalf("unexpected detail response: status=%d body=%s", getResponse.Code, getResponse.Body.String())
	}

	invalid := httptest.NewRequest(http.MethodPost, "/expenses/1", strings.NewReader("category_id=999"))
	invalid.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	invalid.SetPathValue("id", "1")
	invalidResponse := httptest.NewRecorder()
	handler.HandlePostExpenseDetail(invalidResponse, invalid)
	if invalidResponse.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid category status 400, got %d", invalidResponse.Code)
	}

	valid := httptest.NewRequest(http.MethodPost, "/expenses/1", strings.NewReader("category_id=2&return=/?sort=date"))
	valid.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	valid.SetPathValue("id", "1")
	validResponse := httptest.NewRecorder()
	handler.HandlePostExpenseDetail(validResponse, valid)
	if validResponse.Code != http.StatusSeeOther || !strings.Contains(validResponse.Header().Get("Location"), "updated=1") {
		t.Fatalf("expected successful correction redirect, status=%d location=%q", validResponse.Code, validResponse.Header().Get("Location"))
	}
}
