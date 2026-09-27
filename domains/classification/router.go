package classification

import (
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"time"

	"github.com/ghab/red-numbers/platform"
)

// Handler serves the classification log viewer.
type Handler struct {
	logger  *slog.Logger
	service *Service
}

// NewHandler creates a classification handler using injected dependencies and service.
func NewHandler(deps platform.Dependencies, service *Service) *Handler {
	return &Handler{logger: deps.Logger, service: service}
}

// HandleGetClassificationLog renders automatic classification decisions.
func (h *Handler) HandleGetClassificationLog(w http.ResponseWriter, r *http.Request) {
	logs, err := h.service.GetLogs(r.Context())
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Failed to load classification logs", slog.String("error", err.Error()))
		http.Error(w, "Failed to load classification logs", http.StatusInternalServerError)
		return
	}

	rows := ""
	for _, entry := range logs {
		rows += fmt.Sprintf(`<tr><td>%s</td><td>%d</td><td>%s</td><td>%s</td></tr>`,
			entry.Timestamp.Format(time.RFC3339), entry.ExpenseID, html.EscapeString(entry.CategoryName), html.EscapeString(entry.Details))
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><title>Classification Log</title><meta charset="UTF-8"></head>
<body><h1>Classification Log</h1>
<table><thead><tr><th>Timestamp</th><th>Expense</th><th>Category</th><th>Details</th></tr></thead>
<tbody>%s</tbody></table><p><a href="/">Back to dashboard</a></p>
</body></html>`, rows)
}
