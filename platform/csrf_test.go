package platform

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestCSRFAllowsGetAndIssuesToken(t *testing.T) {
	var seenToken string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenToken = CSRFToken(r)
		w.WriteHeader(http.StatusOK)
	})
	handler := CSRF(testLogger())(next)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected GET to succeed, got %d", response.Code)
	}
	if seenToken == "" || !isValidToken(seenToken) {
		t.Fatalf("expected a valid token in context, got %q", seenToken)
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "csrf_token" || cookies[0].Value != seenToken {
		t.Fatalf("expected csrf_token cookie matching context token, got %+v", cookies)
	}
}

func TestCSRFRejectsPostWithoutCookie(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	handler := CSRF(testLogger())(next)

	request := httptest.NewRequest(http.MethodPost, "/expenses/1", strings.NewReader("category_id=2"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for POST without a CSRF cookie, got %d", response.Code)
	}
}

func TestCSRFRejectsPostWithMismatchedToken(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	handler := CSRF(testLogger())(next)

	form := url.Values{"csrf_token": {"wrong-token-that-is-not-64-hex-characters-long-enough-000000"}, "category_id": {"2"}}
	request := httptest.NewRequest(http.MethodPost, "/expenses/1", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(&http.Cookie{Name: "csrf_token", Value: strings.Repeat("a", 64)})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for mismatched CSRF token, got %d", response.Code)
	}
}

func TestCSRFAcceptsPostWithMatchingToken(t *testing.T) {
	var reachedHandler bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reachedHandler = true
		w.WriteHeader(http.StatusOK)
	})
	handler := CSRF(testLogger())(next)
	token := strings.Repeat("a", 64)

	form := url.Values{"csrf_token": {token}, "category_id": {"2"}}
	request := httptest.NewRequest(http.MethodPost, "/expenses/1", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(&http.Cookie{Name: "csrf_token", Value: token})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK || !reachedHandler {
		t.Fatalf("expected matching token to reach the handler, status=%d reached=%v", response.Code, reachedHandler)
	}
}

func TestCSRFTokenOutsideMiddlewareReturnsEmpty(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	if token := CSRFToken(request); token != "" {
		t.Fatalf("expected empty token outside middleware, got %q", token)
	}
}

func TestIsValidToken(t *testing.T) {
	tests := map[string]bool{
		strings.Repeat("a", 64): true,
		strings.Repeat("A", 64): false, // uppercase hex not accepted
		strings.Repeat("a", 63): false,
		"":                      false,
		strings.Repeat("g", 64): false, // not a hex digit
	}
	for input, want := range tests {
		if got := isValidToken(input); got != want {
			t.Errorf("isValidToken(%q) = %v, want %v", input, got, want)
		}
	}
}
