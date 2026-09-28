package classification

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ghab/red-numbers/platform"
	_ "github.com/mattn/go-sqlite3"
)

func TestHandleGetClassificationLogRendersEntries(t *testing.T) {
	db := newClassificationTestDB(t)
	repository := NewRepository(db)
	if err := repository.Log(context.Background(), 42, Classification{CategoryName: "supermercado", Pattern: "mercadona", Confidence: "high"}); err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(platform.Dependencies{Logger: logger}, NewService(repository))

	request := httptest.NewRequest(http.MethodGet, "/classification-log", nil)
	response := httptest.NewRecorder()
	handler.HandleGetClassificationLog(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", response.Code)
	}
	if !bytes.Contains(response.Body.Bytes(), []byte("supermercado")) || !bytes.Contains(response.Body.Bytes(), []byte("42")) {
		t.Fatalf("expected log entry in response body: %s", response.Body.String())
	}
}

func TestHandleGetClassificationLogFailsGracefullyWhenTableMissing(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(platform.Dependencies{Logger: logger}, NewService(NewRepository(db)))

	request := httptest.NewRequest(http.MethodGet, "/classification-log", nil)
	response := httptest.NewRecorder()
	handler.HandleGetClassificationLog(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when the audit_log table is missing, got %d", response.Code)
	}
}
