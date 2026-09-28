package platform

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"
)

const csrfCookieName = "csrf_token"

type csrfContextKey struct{}

// CSRFToken returns the token that should be embedded as a hidden form field
// on any page rendering a POST form. It returns "" when called outside the
// CSRF middleware (e.g. a handler test invoked directly), so templates can
// call it unconditionally.
func CSRFToken(r *http.Request) string {
	token, _ := r.Context().Value(csrfContextKey{}).(string)
	return token
}

// CSRF implements the double-submit-cookie pattern: a random token is stored
// in an HttpOnly cookie and must also be echoed back as a "csrf_token" form
// field on every state-changing request. A cross-site page can trigger a
// form submission but can't read the cookie to copy its value, so a mismatch
// (or a missing cookie) is rejected.
func CSRF(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			existingToken := ""
			if cookie, err := r.Cookie(csrfCookieName); err == nil && isValidToken(cookie.Value) {
				existingToken = cookie.Value
			}

			token := existingToken
			if token == "" {
				generated, err := generateCSRFToken()
				if err != nil {
					logger.ErrorContext(r.Context(), "failed to generate CSRF token", slog.String("error", err.Error()))
					http.Error(w, "Internal server error", http.StatusInternalServerError)
					return
				}
				token = generated
			}
			if token != existingToken {
				http.SetCookie(w, &http.Cookie{
					Name:     csrfCookieName,
					Value:    token,
					Path:     "/",
					HttpOnly: true,
					SameSite: http.SameSiteLaxMode,
					MaxAge:   60 * 60 * 24, // 24h; regenerated as needed
				})
			}

			if isStateChangingMethod(r.Method) {
				submittedToken := r.FormValue("csrf_token")
				validSubmission := existingToken != "" && isValidToken(submittedToken) &&
					subtle.ConstantTimeCompare([]byte(submittedToken), []byte(existingToken)) == 1
				if !validSubmission {
					logger.WarnContext(r.Context(), "rejected request with missing or invalid CSRF token",
						slog.String("method", r.Method), slog.String("path", r.URL.Path))
					http.Error(w, "Invalid or missing CSRF token", http.StatusForbidden)
					return
				}
			}

			r = r.WithContext(context.WithValue(r.Context(), csrfContextKey{}, token))
			next.ServeHTTP(w, r)
		})
	}
}

func isStateChangingMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func generateCSRFToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

// isValidToken performs a cheap shape check (64 lowercase hex characters)
// before comparing, so malformed cookies don't reach the constant-time compare.
func isValidToken(value string) bool {
	if len(value) != 64 {
		return false
	}
	return strings.IndexFunc(value, func(r rune) bool {
		return !(r >= '0' && r <= '9') && !(r >= 'a' && r <= 'f')
	}) == -1
}
