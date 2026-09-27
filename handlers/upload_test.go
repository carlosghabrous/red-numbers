package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ghab/red-numbers/repositories"
	_ "github.com/mattn/go-sqlite3"
)

func TestUploadEndpoint(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewUploadHandler(logger)

	tests := []struct {
		name           string
		csv            string
		expectedStatus int
		expectedText   string
	}{
		{
			name: "valid CSV displays parsed data",
			csv: "fecha de operación;concepto;fecha valor;importe;saldo\n" +
				"05/01/2026;Mercadona;05/01/2026;-18,00;2.121,64\n",
			expectedStatus: http.StatusOK,
			expectedText:   "Mercadona",
		},
		{
			name: "missing required column returns error",
			csv: "fecha de operación;concepto;fecha valor;saldo\n" +
				"05/01/2026;Mercadona;05/01/2026;2.121,64\n",
			expectedStatus: http.StatusBadRequest,
			expectedText:   "importe",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			part, err := writer.CreateFormFile("file", "expenses.csv")
			if err != nil {
				t.Fatalf("CreateFormFile failed: %v", err)
			}
			if _, err := part.Write([]byte(test.csv)); err != nil {
				t.Fatalf("Writing CSV failed: %v", err)
			}
			if err := writer.Close(); err != nil {
				t.Fatalf("Closing multipart writer failed: %v", err)
			}

			request := httptest.NewRequest(http.MethodPost, "/upload", &body)
			request.Header.Set("Content-Type", writer.FormDataContentType())
			response := httptest.NewRecorder()

			handler.HandlePostUpload(response, request)

			if response.Code != test.expectedStatus {
				t.Fatalf("Expected status %d, got %d", test.expectedStatus, response.Code)
			}
			if !bytes.Contains(response.Body.Bytes(), []byte(test.expectedText)) {
				t.Errorf("Expected response to contain %q", test.expectedText)
			}
			if test.expectedStatus == http.StatusOK {
				if !bytes.Contains(response.Body.Bytes(), []byte("Category")) ||
					!bytes.Contains(response.Body.Bytes(), []byte("Confidence")) {
					t.Error("Expected upload preview to show category and confidence columns")
				}
			}
		})
	}
}

func TestUploadPersistsExpensesAcrossDatabaseReopen(t *testing.T) {
	databasePath := t.TempDir() + "/expenses.db"
	db, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE expenses (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		date DATE NOT NULL,
		description TEXT NOT NULL,
		amount REAL NOT NULL,
		balance REAL NOT NULL,
		fingerprint TEXT,
		category_id INTEGER,
		confidence_level TEXT DEFAULT 'low',
		imported_at TIMESTAMP NOT NULL,
		corrected_at TIMESTAMP
	);
	CREATE TABLE categories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		display_name TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		action TEXT NOT NULL,
		expense_id INTEGER,
		new_value TEXT,
		details TEXT
	);
	INSERT INTO categories (name, display_name) VALUES
		('supermercado', 'Supermercado'),
		('medico', 'Médico'),
		('niños', 'Niños'),
		('ocio', 'Ocio'),
		('deporte', 'Deporte'),
		('suministros', 'Suministros'),
		('casa', 'Casa');
	`)
	if err != nil {
		db.Close()
		t.Fatalf("create expenses table: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewUploadHandler(logger, db)
	csv := "fecha de operación;concepto;fecha valor;importe;saldo\n" +
		"05/01/2026;Persistent expense;05/01/2026;-18,00;2.121,64\n"
	response := uploadRequest(t, handler, csv)
	if response.Code != http.StatusOK {
		t.Fatalf("expected upload status 200, got %d: %s", response.Code, response.Body.String())
	}
	if !bytes.Contains(response.Body.Bytes(), []byte("Expenses saved to database:")) ||
		!bytes.Contains(response.Body.Bytes(), []byte(">1</span>")) {
		t.Fatalf("expected saved count in response: %s", response.Body.String())
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}

	reopened, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	defer reopened.Close()
	stored, err := repositories.NewExpenseRepository(reopened).GetAll(context.Background())
	if err != nil {
		t.Fatalf("read persisted expense: %v", err)
	}
	if len(stored) != 1 || stored[0].Description != "Persistent expense" {
		t.Fatalf("expected persisted expense, got %+v", stored)
	}
}

func uploadRequest(t *testing.T, handler *UploadHandler, csv string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "expenses.csv")
	if err != nil {
		t.Fatalf("CreateFormFile failed: %v", err)
	}
	if _, err := part.Write([]byte(csv)); err != nil {
		t.Fatalf("writing CSV failed: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("closing multipart writer failed: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/upload", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	handler.HandlePostUpload(response, request)
	return response
}

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
	handler := NewUploadHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), db)

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
