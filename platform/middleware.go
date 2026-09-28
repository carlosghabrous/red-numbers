package platform

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

// Middleware wraps an http.Handler with cross-cutting behavior.
type Middleware func(http.Handler) http.Handler

// Chain applies middleware in order, so the first one listed runs first
// (outermost) and wraps every one that follows.
func Chain(handler http.Handler, middleware ...Middleware) http.Handler {
	for index := len(middleware) - 1; index >= 0; index-- {
		handler = middleware[index](handler)
	}
	return handler
}

// statusRecorder captures the status code written by the wrapped handler, so
// logging middleware can report it (http.ResponseWriter doesn't expose it).
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(status int) {
	if !r.wroteHeader {
		r.status = status
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(body []byte) (int, error) {
	if !r.wroteHeader {
		r.status = http.StatusOK
		r.wroteHeader = true
	}
	return r.ResponseWriter.Write(body)
}

// RequestLogging logs the method, path, status code, and duration of every request.
func RequestLogging(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(recorder, r)
			logger.InfoContext(r.Context(), "request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", recorder.status),
				slog.Duration("duration", time.Since(started)),
			)
		})
	}
}

// Recover catches panics from downstream handlers, logs them with a stack
// trace, and returns a generic 500 instead of crashing the server.
func Recover(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.ErrorContext(r.Context(), "panic recovered",
						slog.Any("error", recovered),
						slog.String("stack", string(debug.Stack())),
					)
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprint(w, `<!DOCTYPE html><html><head><title>Error</title><meta charset="UTF-8"></head>`+
						`<body style="font-family:sans-serif;padding:40px"><h1>Something went wrong</h1>`+
						`<p>An unexpected error occurred. Please try again.</p><a href="/">Back to dashboard</a></body></html>`)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeaders sets baseline defensive headers on every response. HSTS is
// only sent over an already-TLS connection, since advertising it over plain
// HTTP would be misleading (and could lock out an http-only deployment).
func SecurityHeaders() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := w.Header()
			header.Set("X-Content-Type-Options", "nosniff")
			header.Set("X-Frame-Options", "DENY")
			header.Set("X-XSS-Protection", "1; mode=block")
			header.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			header.Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; base-uri 'none'; frame-ancestors 'none'")
			if r.TLS != nil {
				header.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}
