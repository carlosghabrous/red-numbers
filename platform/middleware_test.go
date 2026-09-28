package platform

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestLoggingRecordsStatusAndPath(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	handler := RequestLogging(logger)(next)

	request := httptest.NewRequest(http.MethodGet, "/expenses/1", nil)
	handler.ServeHTTP(httptest.NewRecorder(), request)

	output := buf.String()
	if !strings.Contains(output, "path=/expenses/1") || !strings.Contains(output, "status=418") {
		t.Fatalf("expected log to record path and status, got: %s", output)
	}
}

func TestRequestLoggingDefaultsStatusToOKWhenHandlerOnlyWrites(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok")) //nolint:errcheck
	})
	handler := RequestLogging(logger)(next)
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if !strings.Contains(buf.String(), "status=200") {
		t.Fatalf("expected default status 200 when handler never calls WriteHeader, got: %s", buf.String())
	}
}

func TestRecoverCatchesPanicAndReturns500(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})
	handler := Recover(logger)(next)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 after recovering from panic, got %d", response.Code)
	}
	if !strings.Contains(buf.String(), "panic recovered") {
		t.Fatalf("expected panic to be logged, got: %s", buf.String())
	}
}

func TestSecurityHeadersSetsBaselineHeaders(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	handler := SecurityHeaders()(next)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	header := response.Header()
	checks := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
	}
	for key, want := range checks {
		if got := header.Get(key); got != want {
			t.Errorf("expected header %s=%q, got %q", key, want, got)
		}
	}
	if header.Get("Strict-Transport-Security") != "" {
		t.Error("expected no HSTS header over plain HTTP")
	}
}

func TestChainAppliesMiddlewareInOrder(t *testing.T) {
	var order []string
	trace := func(name string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next.ServeHTTP(w, r)
			})
		}
	}
	final := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { order = append(order, "handler") })
	handler := Chain(final, trace("outer"), trace("inner"))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	want := []string{"outer", "inner", "handler"}
	if len(order) != len(want) {
		t.Fatalf("expected order %v, got %v", want, order)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("expected order %v, got %v", want, order)
		}
	}
}
