package handlers

import (
	"database/sql"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ghab/red-numbers/models"
	"github.com/ghab/red-numbers/repositories"
	"github.com/ghab/red-numbers/services"
)

// UploadHandler handles CSV file uploads
type UploadHandler struct {
	logger    *slog.Logger
	csvParser *services.CSVParser
	expenses  *repositories.ExpenseRepository
}

// NewUploadHandler creates a new upload handler
func NewUploadHandler(logger *slog.Logger, db ...*sql.DB) *UploadHandler {
	handler := &UploadHandler{
		logger:    logger,
		csvParser: services.NewCSVParser(),
	}
	if len(db) > 0 && db[0] != nil {
		handler.expenses = repositories.NewExpenseRepository(db[0])
	}
	return handler
}

// HandleGetUpload serves the CSV upload form
func (h *UploadHandler) HandleGetUpload(w http.ResponseWriter, r *http.Request) {
	h.logger.DebugContext(r.Context(), "GET /upload - serving upload form")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	html := `<!DOCTYPE html>
<html>
<head>
	<title>Upload CSV - Expense Tracking</title>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<style>
		* {
			margin: 0;
			padding: 0;
			box-sizing: border-box;
		}
		body {
			font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
			background: #f5f5f5;
			padding: 20px;
		}
		.container {
			max-width: 600px;
			margin: 0 auto;
			background: white;
			border-radius: 8px;
			box-shadow: 0 2px 4px rgba(0,0,0,0.1);
			padding: 40px;
		}
		h1 {
			color: #333;
			margin-bottom: 10px;
			font-size: 28px;
		}
		.subtitle {
			color: #666;
			margin-bottom: 30px;
			font-size: 14px;
		}
		.form-group {
			margin-bottom: 25px;
		}
		label {
			display: block;
			margin-bottom: 8px;
			color: #333;
			font-weight: 500;
			font-size: 14px;
		}
		input[type="file"] {
			display: block;
			padding: 10px;
			border: 2px solid #e0e0e0;
			border-radius: 4px;
			cursor: pointer;
			font-size: 14px;
			width: 100%;
		}
		input[type="file"]:hover {
			border-color: #999;
		}
		.file-info {
			font-size: 12px;
			color: #666;
			margin-top: 8px;
		}
		.instructions {
			background: #f9f9f9;
			border-left: 4px solid #007bff;
			padding: 15px;
			margin-bottom: 25px;
			border-radius: 4px;
			font-size: 13px;
			color: #555;
			line-height: 1.6;
		}
		.instructions h3 {
			color: #333;
			margin-bottom: 10px;
			font-size: 14px;
		}
		.instructions ul {
			margin-left: 20px;
		}
		.instructions li {
			margin-bottom: 5px;
		}
		.instructions code {
			background: #eee;
			padding: 2px 4px;
			border-radius: 2px;
			font-family: monospace;
			font-size: 12px;
		}
		button {
			background: #007bff;
			color: white;
			padding: 12px 30px;
			border: none;
			border-radius: 4px;
			font-size: 16px;
			font-weight: 500;
			cursor: pointer;
			width: 100%;
			transition: background 0.2s;
		}
		button:hover {
			background: #0056b3;
		}
		button:active {
			background: #004085;
		}
		.error {
			background: #f8d7da;
			border: 1px solid #f5c6cb;
			color: #721c24;
			padding: 12px;
			border-radius: 4px;
			margin-bottom: 20px;
			font-size: 14px;
		}
		.success {
			background: #d4edda;
			border: 1px solid #c3e6cb;
			color: #155724;
			padding: 12px;
			border-radius: 4px;
			margin-bottom: 20px;
			font-size: 14px;
		}
		.example-table {
			width: 100%;
			border-collapse: collapse;
			font-size: 12px;
			margin-top: 10px;
			background: #fff;
		}
		.example-table th,
		.example-table td {
			border: 1px solid #ddd;
			padding: 8px;
			text-align: left;
		}
		.example-table th {
			background: #f5f5f5;
			font-weight: 600;
		}
	</style>
</head>
<body>
	<div class="container">
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
			<h3 style="margin-top: 15px;">Example CSV Format</h3>
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

		<form method="POST" action="/upload" enctype="multipart/form-data">
			<div class="form-group">
				<label for="file">Select CSV File:</label>
				<input type="file" id="file" name="file" accept=".csv" required>
				<div class="file-info">Only .csv files are accepted</div>
			</div>
			<button type="submit">Upload CSV</button>
		</form>
	</div>
</body>
</html>`

	fmt.Fprint(w, html)
}

// HandlePostUpload processes a CSV file upload
func (h *UploadHandler) HandlePostUpload(w http.ResponseWriter, r *http.Request) {
	h.logger.DebugContext(r.Context(), "POST /upload - processing file upload")

	// Parse multipart form
	err := r.ParseMultipartForm(10 << 20) // 10MB max file size
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Failed to parse multipart form", slog.String("error", err.Error()))
		h.renderUploadError(w, "Failed to parse form data", nil)
		return
	}

	// Get file from form
	file, header, err := r.FormFile("file")
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Failed to get file from form", slog.String("error", err.Error()))
		h.renderUploadError(w, "No file provided", nil)
		return
	}
	defer file.Close()

	// Validate file extension
	ext := filepath.Ext(header.Filename)
	if strings.ToLower(ext) != ".csv" {
		h.logger.WarnContext(r.Context(), "Invalid file extension", slog.String("filename", header.Filename))
		h.renderUploadError(w, fmt.Sprintf("Invalid file extension: %s. Only .csv files are accepted.", ext), nil)
		return
	}

	// Save file to temporary location
	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, fmt.Sprintf("upload_%d.csv", time.Now().UnixNano()))

	outFile, err := os.Create(tempFile)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Failed to create temp file", slog.String("error", err.Error()))
		h.renderUploadError(w, "Failed to save uploaded file", nil)
		return
	}
	defer outFile.Close()

	// Copy uploaded file to temp location
	_, err = io.Copy(outFile, file)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Failed to write temp file", slog.String("error", err.Error()))
		h.renderUploadError(w, "Failed to process uploaded file", nil)
		return
	}

	// Parse CSV file
	expenses, skippedRows, err := h.csvParser.ParseCSVFileWithStats(tempFile)
	if err != nil {
		h.logger.WarnContext(r.Context(), "CSV parsing failed", slog.String("error", err.Error()))
		h.renderUploadError(w, err.Error(), nil)
		// Clean up temp file
		os.Remove(tempFile)
		return
	}

	// Clean up temp file
	defer os.Remove(tempFile)

	h.logger.InfoContext(r.Context(), "CSV parsed successfully", slog.Int("expense_count", len(expenses)))
	savedCount := 0
	if h.expenses != nil {
		if err := h.expenses.CreateBatch(r.Context(), expenses); err != nil {
			h.logger.ErrorContext(r.Context(), "Failed to save expenses", slog.String("error", err.Error()))
			h.renderUploadError(w, "Failed to save expenses to the database", nil)
			return
		}
		savedCount = len(expenses)
	}

	// Render success response with parsed expenses
	h.renderUploadSuccess(w, header.Filename, expenses, skippedRows, savedCount)
}

// HandleGetDashboard renders all expenses stored in SQLite.
func (h *UploadHandler) HandleGetDashboard(w http.ResponseWriter, r *http.Request) {
	if h.expenses == nil {
		h.renderUploadError(w, "Database is not configured", nil)
		return
	}

	expenses, err := h.expenses.GetAll(r.Context())
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Failed to load expenses", slog.String("error", err.Error()))
		h.renderUploadError(w, "Failed to load expenses from the database", nil)
		return
	}

	h.renderDashboard(w, expenses)
}

func (h *UploadHandler) renderDashboard(w http.ResponseWriter, expenses []models.Expense) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	rows := ""
	for _, expense := range expenses {
		rows += fmt.Sprintf(`<tr><td>%s</td><td>%s</td><td>%.2f€</td><td>%.2f€</td></tr>`,
			expense.Date.Format("02/01/2006"), html.EscapeString(expense.Description), expense.Amount, expense.Balance)
	}

	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
	<title>Expense Dashboard</title>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<style>
		body { font-family: sans-serif; background: #f5f5f5; padding: 20px; }
		.container { max-width: 1000px; margin: 0 auto; background: #fff; padding: 32px; }
		table { width: 100%%; border-collapse: collapse; }
		th, td { padding: 10px 12px; border-bottom: 1px solid #ddd; text-align: left; }
		th { background: #f5f5f5; }
		.table-scroll { overflow-x: auto; }
		a { display: inline-block; margin-top: 20px; }
	</style>
</head>
<body>
	<div class="container">
		<h1>Expense Dashboard</h1>
		<p>%d expenses stored in the database.</p>
		<div class="table-scroll">
			<table>
				<thead><tr><th>Date</th><th>Description</th><th>Amount</th><th>Balance</th></tr></thead>
				<tbody>%s</tbody>
			</table>
		</div>
		<a href="/upload">Upload another CSV</a>
	</div>
</body>
</html>`, len(expenses), rows)
}

// renderUploadError renders an error page for upload failures
func (h *UploadHandler) renderUploadError(w http.ResponseWriter, errMsg string, missingColumns []string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)

	errorDetails := errMsg
	if len(missingColumns) > 0 {
		errorDetails = fmt.Sprintf("Missing required columns: %v", missingColumns)
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
	<title>Upload Error - Expense Tracking</title>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<style>
		* {
			margin: 0;
			padding: 0;
			box-sizing: border-box;
		}
		body {
			font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
			background: #f5f5f5;
			padding: 20px;
		}
		.container {
			max-width: 600px;
			margin: 0 auto;
			background: white;
			border-radius: 8px;
			box-shadow: 0 2px 4px rgba(0,0,0,0.1);
			padding: 40px;
		}
		.error-box {
			background: #f8d7da;
			border: 1px solid #f5c6cb;
			color: #721c24;
			padding: 15px;
			border-radius: 4px;
			margin-bottom: 25px;
		}
		.error-title {
			font-weight: 600;
			margin-bottom: 10px;
			font-size: 16px;
		}
		.error-message {
			font-size: 14px;
			line-height: 1.5;
		}
		h1 {
			color: #333;
			margin-bottom: 10px;
			font-size: 28px;
		}
		.subtitle {
			color: #666;
			margin-bottom: 25px;
			font-size: 14px;
		}
		a {
			color: #007bff;
			text-decoration: none;
		}
		a:hover {
			text-decoration: underline;
		}
		.back-link {
			display: inline-block;
			margin-top: 20px;
			padding: 10px 20px;
			background: #007bff;
			color: white;
			border-radius: 4px;
			text-decoration: none;
		}
		.back-link:hover {
			background: #0056b3;
			text-decoration: none;
		}
	</style>
</head>
<body>
	<div class="container">
		<h1>Upload Failed</h1>
		<p class="subtitle">There was an error processing your CSV file</p>

		<div class="error-box">
			<div class="error-title">Error Details:</div>
			<div class="error-message">%s</div>
		</div>

		<p>Please check your CSV file and try again.</p>
		<a href="/upload" class="back-link">← Back to Upload</a>
	</div>
</body>
</html>`, errorDetails)

	fmt.Fprint(w, html)
}

// renderUploadSuccess renders a success page with parsed expenses
func (h *UploadHandler) renderUploadSuccess(w http.ResponseWriter, filename string, expenses []models.Expense, skippedRows int, savedCount int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	// Build table rows
	tableRows := ""
	for _, expense := range expenses {
		tableRows += fmt.Sprintf(`
		<tr>
			<td>%s</td>
			<td>%s</td>
			<td>%.2f€</td>
			<td>%.2f€</td>
		</tr>`, expense.Date.Format("02/01/2006"), html.EscapeString(expense.Description), expense.Amount, expense.Balance)
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
	<title>Upload Successful - Expense Tracking</title>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<style>
		* {
			margin: 0;
			padding: 0;
			box-sizing: border-box;
		}
		body {
			font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
			background: #f5f5f5;
			padding: 20px;
		}
		.container {
			max-width: 800px;
			margin: 0 auto;
			background: white;
			border-radius: 8px;
			box-shadow: 0 2px 4px rgba(0,0,0,0.1);
			padding: 40px;
		}
		.success-box {
			background: #d4edda;
			border: 1px solid #c3e6cb;
			color: #155724;
			padding: 15px;
			border-radius: 4px;
			margin-bottom: 25px;
		}
		.success-title {
			font-weight: 600;
			margin-bottom: 5px;
			font-size: 16px;
		}
		h1 {
			color: #333;
			margin-bottom: 10px;
			font-size: 28px;
		}
		.subtitle {
			color: #666;
			margin-bottom: 20px;
			font-size: 14px;
		}
		.summary {
			background: #f9f9f9;
			padding: 15px;
			border-radius: 4px;
			margin-bottom: 25px;
			font-size: 14px;
		}
		.summary-row {
			display: flex;
			justify-content: space-between;
			margin-bottom: 8px;
		}
		.summary-label {
			font-weight: 500;
			color: #666;
		}
		.summary-value {
			color: #333;
			font-weight: 600;
		}
		table {
			width: 100%%;
			border-collapse: collapse;
			margin-bottom: 25px;
			font-size: 13px;
		}
		th {
			background: #f5f5f5;
			color: #333;
			font-weight: 600;
			text-align: left;
			padding: 12px;
			border-bottom: 2px solid #ddd;
		}
		td {
			padding: 10px 12px;
			border-bottom: 1px solid #eee;
		}
		tr:hover {
			background: #f9f9f9;
		}
		.text-right {
			text-align: right;
		}
		.actions {
			display: flex;
			gap: 10px;
		}
		a, button {
			display: inline-block;
			padding: 10px 20px;
			border-radius: 4px;
			text-decoration: none;
			border: none;
			cursor: pointer;
			font-size: 14px;
			transition: background 0.2s;
		}
		.btn-primary {
			background: #007bff;
			color: white;
		}
		.btn-primary:hover {
			background: #0056b3;
		}
		.btn-secondary {
			background: #6c757d;
			color: white;
		}
		.btn-secondary:hover {
			background: #545b62;
		}
		.table-scroll {
			overflow-x: auto;
			margin-bottom: 20px;
		}
		table {
			min-width: 640px;
		}
	</style>
</head>
<body>
	<div class="container">
		<h1>CSV Upload Successful</h1>
		<p class="subtitle">Your file has been parsed and is ready for review</p>

		<div class="success-box">
			<div class="success-title">✓ File processed successfully</div>
			File: <strong>%s</strong>
		</div>

		<div class="summary">
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

		<h2 style="font-size: 18px; margin-bottom: 15px; color: #333;">Parsed Expenses Preview</h2>
		<div class="table-scroll">
			<table>
				<thead>
					<tr>
						<th>Date</th>
						<th>Description</th>
						<th>Amount</th>
						<th class="text-right">Balance</th>
					</tr>
				</thead>
				<tbody>
					%s
				</tbody>
			</table>
		</div>

		<div class="actions">
			<a href="/upload" class="btn-secondary">← Upload Another File</a>
			<a href="/" class="btn-primary">Go to Dashboard →</a>
		</div>
	</div>
</body>
</html>`, html.EscapeString(filename), len(expenses), skippedRows, savedCount, tableRows)

	fmt.Fprint(w, html)
}
