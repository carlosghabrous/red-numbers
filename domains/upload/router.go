package upload

import (
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ghab/red-numbers/domains/classification"
	"github.com/ghab/red-numbers/domains/expenses"
	"github.com/ghab/red-numbers/platform"
)

// Handler serves the CSV upload form and processes uploads.
type Handler struct {
	logger  *slog.Logger
	service *Service
}

// NewHandler creates an upload handler using injected dependencies and service.
func NewHandler(deps platform.Dependencies, service *Service) *Handler {
	return &Handler{logger: deps.Logger, service: service}
}

// HandleGetUpload serves the CSV upload form.
func (h *Handler) HandleGetUpload(w http.ResponseWriter, r *http.Request) {
	h.logger.DebugContext(r.Context(), "GET /upload - serving upload form")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	page := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
	<title>Upload CSV - Expense Tracking</title>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<link rel="stylesheet" href="/static/style.css">
</head>
<body>
	<div class="page">
	<div class="card card--narrow">
		<h1>Upload Bank Export CSV</h1>
		<p class="subtitle">Import your transaction history</p>

		<div class="instructions">
			<h3>CSV Format Requirements</h3>
			<ul>
				<li>File must be in <strong>.csv</strong> format</li>
				<li>First row must contain column headers (case-insensitive)</li>
				<li>Required columns: <code>fecha de operación</code>, <code>concepto</code>, <code>fecha valor</code>, <code>importe</code>, <code>saldo</code></li>
				<li>Date format: DD/MM/YYYY or DD-MM-YYYY</li>
				<li>Amount format: supports comma (1.234,56) or period (1234.56) as decimal separator</li>
			</ul>
			<h3>Example CSV Format</h3>
			<div class="table-scroll">
			<table class="example-table">
				<tr>
					<th>fecha de operación</th>
					<th>concepto</th>
					<th>fecha valor</th>
					<th>importe</th>
					<th>saldo</th>
				</tr>
				<tr>
					<td>01/01/2024</td>
					<td>Mercadona Compra Supermercado</td>
					<td>01/01/2024</td>
					<td>45,50</td>
					<td>1.234,56</td>
				</tr>
				<tr>
					<td>02/01/2024</td>
					<td>Telefonica Pago Factura</td>
					<td>02/01/2024</td>
					<td>65,00</td>
					<td>1.169,56</td>
				</tr>
			</table>
			</div>
		</div>

		<form method="POST" action="/upload" enctype="multipart/form-data">
			<input type="hidden" name="csrf_token" value="%s">
			<div class="form-group">
				<label for="file">Select CSV File:</label>
				<input type="file" id="file" name="file" accept=".csv" required>
				<div class="file-info">Only .csv files are accepted, up to 10MB</div>
			</div>
			<button class="btn" type="submit">Upload CSV</button>
		</form>
		<p><a href="/">← Return to Dashboard</a></p>
	</div>
	</div>
</body>
</html>`, html.EscapeString(platform.CSRFToken(r)))

	fmt.Fprint(w, page)
}

// HandlePostUpload processes a CSV file upload.
func (h *Handler) HandlePostUpload(w http.ResponseWriter, r *http.Request) {
	h.logger.DebugContext(r.Context(), "POST /upload - processing file upload")

	// Parse multipart form
	err := r.ParseMultipartForm(10 << 20) // 10MB max file size
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Failed to parse multipart form", slog.String("error", err.Error()))
		h.renderUploadError(w, "Failed to parse form data")
		return
	}

	// Get file from form
	file, header, err := r.FormFile("file")
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Failed to get file from form", slog.String("error", err.Error()))
		h.renderUploadError(w, "No file provided")
		return
	}
	defer file.Close()

	// Validate file extension
	ext := filepath.Ext(header.Filename)
	if strings.ToLower(ext) != ".csv" {
		h.logger.WarnContext(r.Context(), "Invalid file extension", slog.String("filename", header.Filename))
		h.renderUploadError(w, fmt.Sprintf("Invalid file extension: %s. Only .csv files are accepted.", ext))
		return
	}

	// Save file to temporary location
	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, fmt.Sprintf("upload_%d.csv", time.Now().UnixNano()))

	outFile, err := os.Create(tempFile)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Failed to create temp file", slog.String("error", err.Error()))
		h.renderUploadError(w, "Failed to save uploaded file")
		return
	}
	defer outFile.Close()

	// Copy uploaded file to temp location
	_, err = io.Copy(outFile, file)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Failed to write temp file", slog.String("error", err.Error()))
		h.renderUploadError(w, "Failed to process uploaded file")
		return
	}

	// Parse CSV file
	parsedExpenses, skippedRows, err := h.service.ParseFile(tempFile)
	if err != nil {
		h.logger.WarnContext(r.Context(), "CSV parsing failed", slog.String("error", err.Error()))
		h.renderUploadError(w, err.Error())
		// Clean up temp file
		os.Remove(tempFile)
		return
	}
	classifications, err := h.service.ClassifyExpenses(r.Context(), parsedExpenses)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Expense classification failed", slog.String("error", err.Error()))
		h.renderUploadError(w, "Failed to classify expenses")
		return
	}

	// Clean up temp file
	defer os.Remove(tempFile)

	h.logger.InfoContext(r.Context(), "CSV parsed successfully", slog.Int("expense_count", len(parsedExpenses)))
	savedCount, err := h.service.SaveExpenses(r.Context(), parsedExpenses, classifications)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Failed to save expenses", slog.String("error", err.Error()))
		h.renderUploadError(w, "Failed to save expenses to the database")
		return
	}

	// Render success response with parsed expenses
	h.renderUploadSuccess(w, header.Filename, parsedExpenses, classifications, skippedRows, savedCount)
}

// renderUploadError renders an error page for upload failures.
func (h *Handler) renderUploadError(w http.ResponseWriter, errMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)

	page := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
	<title>Upload Error - Expense Tracking</title>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<link rel="stylesheet" href="/static/style.css">
</head>
<body>
	<div class="page">
	<div class="card card--narrow">
		<h1>Upload Failed</h1>
		<p class="subtitle">There was an error processing your CSV file</p>

		<div class="alert alert-error">
			<strong>Error details:</strong> %s
		</div>

		<p>Please check your CSV file and try again.</p>
		<a class="btn" href="/upload">← Back to Upload</a>
	</div>
	</div>
</body>
</html>`, html.EscapeString(errMsg))

	fmt.Fprint(w, page)
}

// renderUploadSuccess renders a success page with parsed expenses.
func (h *Handler) renderUploadSuccess(w http.ResponseWriter, filename string, expenseList []expenses.Expense, classifications []classification.Classification, skippedRows int, savedCount int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	// Build table rows
	tableRows := ""
	for index, expense := range expenseList {
		category := "Sin clasificar"
		confidence := expense.ConfidenceLevel
		if index < len(classifications) {
			category = classifications[index].CategoryName
			confidence = classifications[index].Confidence
		}
		tableRows += fmt.Sprintf(`
		<tr>
			<td>%s</td>
			<td>%s</td>
			<td>%.2f€</td>
			<td>%.2f€</td>
			<td>%s</td>
			<td>%s</td>
		</tr>`, expense.Date.Format("02/01/2006"), html.EscapeString(expense.Description), expense.Amount, expense.Balance,
			html.EscapeString(category), html.EscapeString(confidence))
	}

	page := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
	<title>Upload Successful - Expense Tracking</title>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<link rel="stylesheet" href="/static/style.css">
</head>
<body>
	<div class="page">
	<div class="card">
		<h1>CSV Upload Successful</h1>
		<p class="subtitle">Your file has been parsed and is ready for review</p>

		<div class="alert alert-success">
			<strong>&#10003; File processed successfully.</strong> File: %s
		</div>

		<div class="summary-box">
			<div class="summary-row">
				<span class="summary-label">Expenses parsed:</span>
				<span class="summary-value">%d</span>
			</div>
			<div class="summary-row">
				<span class="summary-label">Rows skipped:</span>
				<span class="summary-value">%d</span>
			</div>
			<div class="summary-row">
				<span class="summary-label">Expenses saved to database:</span>
				<span class="summary-value">%d</span>
			</div>
		</div>

		<div class="actions">
			<a href="/upload" class="btn btn-secondary">← Upload Another File</a>
			<a href="/" class="btn">Go to Dashboard →</a>
		</div>

		<h2>Parsed Expenses Preview</h2>
		<div class="table-scroll">
			<table>
				<thead>
					<tr>
						<th>Date</th>
						<th>Description</th>
						<th>Amount</th>
						<th>Balance</th>
						<th>Category</th>
						<th>Confidence</th>
					</tr>
				</thead>
				<tbody>
					%s
				</tbody>
			</table>
		</div>
	</div>
	</div>
</body>
</html>`, html.EscapeString(filename), len(expenseList), skippedRows, savedCount, tableRows)

	fmt.Fprint(w, page)
}
