package categories

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ghab/red-numbers/platform"
)

func newCategoriesTestHandler(t *testing.T) *Handler {
	t.Helper()
	db := setupCategoriesTestDB(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewHandler(platform.Dependencies{Logger: logger}, NewService(NewRepository(db)))
}

func TestHandlePostCreateRedirectsWithAddedCategory(t *testing.T) {
	handler := newCategoriesTestHandler(t)

	request := httptest.NewRequest(http.MethodPost, "/categories", strings.NewReader("display_name=Mascotas&return=/expenses/1"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.HandlePostCreate(response, request)

	if response.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect, got %d", response.Code)
	}
	location := response.Header().Get("Location")
	if !strings.HasPrefix(location, "/expenses/1?") || !strings.Contains(location, "category_added=Mascotas") {
		t.Fatalf("unexpected redirect location: %q", location)
	}
}

func TestHandlePostCreateRedirectsWithErrorReason(t *testing.T) {
	handler := newCategoriesTestHandler(t)

	tests := []struct {
		name        string
		displayName string
		wantReason  string
	}{
		{"empty name", "   ", "empty_name"},
		{"duplicate name", "Casa", "duplicate"},
		{"too long", strings.Repeat("a", 41), "too_long"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			form := "display_name=" + test.displayName + "&return=/expenses/1"
			request := httptest.NewRequest(http.MethodPost, "/categories", strings.NewReader(form))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			response := httptest.NewRecorder()
			handler.HandlePostCreate(response, request)

			if response.Code != http.StatusSeeOther {
				t.Fatalf("expected redirect, got %d", response.Code)
			}
			location := response.Header().Get("Location")
			if !strings.Contains(location, "category_error="+test.wantReason) {
				t.Fatalf("expected reason %q in redirect, got %q", test.wantReason, location)
			}
		})
	}
}

func TestHandlePostCreateDefaultsReturnURLToRootWhenMissingOrUnsafe(t *testing.T) {
	handler := newCategoriesTestHandler(t)

	tests := []string{"", "https://evil.example.com/steal"}
	for _, returnURL := range tests {
		request := httptest.NewRequest(http.MethodPost, "/categories", strings.NewReader("display_name=Mascotas&return="+returnURL))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response := httptest.NewRecorder()
		handler.HandlePostCreate(response, request)

		location := response.Header().Get("Location")
		if !strings.HasPrefix(location, "/?") {
			t.Fatalf("expected redirect to default to root for return=%q, got %q", returnURL, location)
		}
	}
}

func TestAddQueryParameterOnInvalidURLReturnsRoot(t *testing.T) {
	if got := addQueryParameter("://not a url", "k", "v"); got != "/" {
		t.Fatalf("expected fallback to root for unparsable URL, got %q", got)
	}
}
