package categories

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/ghab/red-numbers/platform"
)

// Handler serves category management endpoints.
type Handler struct {
	logger  *slog.Logger
	service *Service
}

// NewHandler creates a categories handler using injected dependencies and service.
func NewHandler(deps platform.Dependencies, service *Service) *Handler {
	return &Handler{logger: deps.Logger, service: service}
}

// HandlePostCreate adds a new category from a display name, then redirects back
// to whichever page submitted the form (e.g. an expense detail page).
func (h *Handler) HandlePostCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}
	returnURL := r.FormValue("return")
	if returnURL == "" || !strings.HasPrefix(returnURL, "/") {
		returnURL = "/"
	}

	category, err := h.service.CreateCategory(r.Context(), r.FormValue("display_name"))
	if err != nil {
		reason := "failed"
		switch {
		case errors.Is(err, ErrEmptyDisplayName):
			reason = "empty_name"
		case errors.Is(err, ErrDuplicateCategory):
			reason = "duplicate"
		default:
			h.logger.ErrorContext(r.Context(), "Failed to create category", slog.String("error", err.Error()))
		}
		http.Redirect(w, r, addQueryParameter(returnURL, "category_error", reason), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, addQueryParameter(returnURL, "category_added", category.DisplayName), http.StatusSeeOther)
}

func addQueryParameter(rawURL, key, value string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "/"
	}
	query := parsed.Query()
	query.Set(key, value)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
