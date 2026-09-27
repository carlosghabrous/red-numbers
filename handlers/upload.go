package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ghab/red-numbers/models"
	"github.com/ghab/red-numbers/repositories"
	"github.com/ghab/red-numbers/services"
)

// UploadHandler handles CSV file uploads
type UploadHandler struct {
	logger     *slog.Logger
	csvParser  *services.CSVParser
	classifier *services.FuzzyClassifierService
	expenses   *repositories.ExpenseRepository
	categories *repositories.CategoryRepository
}

const dashboardPageSize = 50

// NewUploadHandler creates a new upload handler
func NewUploadHandler(logger *slog.Logger, db ...*sql.DB) *UploadHandler {
	handler := &UploadHandler{
		logger:     logger,
		csvParser:  services.NewCSVParser(),
		classifier: services.NewFuzzyClassifierService(),
	}
	if len(db) > 0 && db[0] != nil {
		handler.expenses = repositories.NewExpenseRepository(db[0])
		handler.categories = repositories.NewCategoryRepository(db[0])
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
		.dashboard-link {
			display: block;
			margin-top: 15px;
			text-align: center;
			color: #007bff;
			text-decoration: none;
			font-size: 14px;
		}
		.dashboard-link:hover {
			text-decoration: underline;
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
		<a href="/" class="dashboard-link">← Return to Dashboard</a>
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
	classifications, err := h.classifyExpenses(r.Context(), expenses)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Expense classification failed", slog.String("error", err.Error()))
		h.renderUploadError(w, "Failed to classify expenses", nil)
		return
	}

	// Clean up temp file
	defer os.Remove(tempFile)

	h.logger.InfoContext(r.Context(), "CSV parsed successfully", slog.Int("expense_count", len(expenses)))
	savedCount := 0
	if h.expenses != nil {
		insertedCount, err := h.expenses.CreateBatchWithCount(r.Context(), expenses)
		if err != nil {
			h.logger.ErrorContext(r.Context(), "Failed to save expenses", slog.String("error", err.Error()))
			h.renderUploadError(w, "Failed to save expenses to the database", nil)
			return
		}
		savedCount = insertedCount
		for index, classification := range classifications {
			if err := h.expenses.LogClassification(r.Context(), expenses[index], repositories.ClassificationLog{
				CategoryName: classification.CategoryName,
				Pattern:      classification.Pattern,
				Confidence:   classification.Confidence,
			}); err != nil {
				h.logger.WarnContext(r.Context(), "Failed to log classification", slog.String("error", err.Error()))
			}
		}
	}

	// Render success response with parsed expenses
	h.renderUploadSuccess(w, header.Filename, expenses, classifications, skippedRows, savedCount)
}

func (h *UploadHandler) classifyExpenses(ctx context.Context, expenses []models.Expense) ([]services.Classification, error) {
	classifications := make([]services.Classification, len(expenses))
	if h.categories == nil {
		for index := range expenses {
			classifications[index] = h.classifier.Classify(expenses[index].Description)
		}
		return classifications, nil
	}

	categories, err := h.categories.GetAllCategories(ctx)
	if err != nil {
		return nil, err
	}
	categoryIDs := make(map[string]int64, len(categories))
	for _, category := range categories {
		categoryIDs[category.Name] = int64(category.ID)
	}

	for index := range expenses {
		classification := h.classifier.Classify(expenses[index].Description)
		categoryID, exists := categoryIDs[classification.CategoryName]
		if !exists {
			return nil, fmt.Errorf("category %q is not seeded", classification.CategoryName)
		}
		expenses[index].CategoryID = categoryID
		expenses[index].ConfidenceLevel = classification.Confidence
		classifications[index] = classification
	}
	return classifications, nil
}

// HandleGetDashboard renders all expenses stored in SQLite.
func (h *UploadHandler) HandleGetDashboard(w http.ResponseWriter, r *http.Request) {
	if h.expenses == nil {
		h.renderUploadError(w, "Database is not configured", nil)
		return
	}

	sortBy, sortDirection := dashboardSortPreference(r)
	page, err := dashboardPage(r)
	if err != nil {
		h.renderUploadError(w, err.Error(), nil)
		return
	}
	filterState, err := dashboardFilterStateFromRequest(r)
	if err != nil {
		h.renderUploadError(w, err.Error(), nil)
		return
	}
	if r.URL.Query().Get("sort") != "" || r.URL.Query().Get("direction") != "" {
		http.SetCookie(w, &http.Cookie{Name: "expense_sort", Value: sortBy + ":" + sortDirection, Path: "/", MaxAge: 60 * 60 * 24 * 365, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	}
	http.SetCookie(w, &http.Cookie{Name: "expense_filters", Value: filterState.cookieValue(), Path: "/", MaxAge: 60 * 60 * 24 * 365, HttpOnly: true, SameSite: http.SameSiteLaxMode})

	filterState.options.SortBy = sortBy
	filterState.options.SortDirection = sortDirection
	filterState.options.Limit = dashboardPageSize
	filterState.options.Offset = (page - 1) * dashboardPageSize
	expenses, err := h.expenses.GetByFilterOptions(r.Context(), filterState.options)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Failed to load expenses", slog.String("error", err.Error()))
		h.renderUploadError(w, "Failed to load expenses from the database", nil)
		return
	}
	totalExpenses, err := h.expenses.CountByFilterOptions(r.Context(), filterState.options)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Failed to count expenses", slog.String("error", err.Error()))
		h.renderUploadError(w, "Failed to count expenses", nil)
		return
	}

	categories, err := h.categories.GetAllCategories(r.Context())
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Failed to load categories", slog.String("error", err.Error()))
		h.renderUploadError(w, "Failed to load categories from the database", nil)
		return
	}
	categoryNames := make(map[int64]string, len(categories))
	for _, category := range categories {
		categoryNames[int64(category.ID)] = category.DisplayName
	}

	h.renderDashboard(w, expenses, categoryNames, categories, sortBy, sortDirection, filterState, page, totalExpenses, r.URL.RawQuery)
}

// HandlePostDeleteAll removes every imported expense after explicit confirmation.
func (h *UploadHandler) HandlePostDeleteAll(w http.ResponseWriter, r *http.Request) {
	if r.FormValue("confirm") != "delete-all" {
		http.Error(w, "Deletion was not confirmed", http.StatusBadRequest)
		return
	}
	if err := h.expenses.DeleteAll(r.Context()); err != nil {
		h.logger.ErrorContext(r.Context(), "Failed to delete all expenses", slog.String("error", err.Error()))
		h.renderUploadError(w, "Failed to delete all expenses", nil)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func dashboardPage(r *http.Request) (int, error) {
	value := r.URL.Query().Get("page")
	if value == "" {
		return 1, nil
	}
	page, err := strconv.Atoi(value)
	if err != nil || page < 1 {
		return 0, fmt.Errorf("invalid page number")
	}
	return page, nil
}

type dashboardFilterState struct {
	options   repositories.ExpenseFilterOptions
	dateMode  string
	startDate string
	endDate   string
	endInput  string
}

func (state dashboardFilterState) cookieValue() string {
	ids := make([]string, len(state.options.CategoryIDs))
	for index, id := range state.options.CategoryIDs {
		ids[index] = strconv.FormatInt(id, 10)
	}
	endValue := state.endInput
	if endValue == "" {
		endValue = state.endDate
	}
	return strings.Join([]string{strings.Join(ids, ","), state.dateMode, state.startDate, endValue}, "|")
}

func dashboardFilterStateFromRequest(r *http.Request) (dashboardFilterState, error) {
	state := dashboardFilterState{dateMode: "all"}
	query := r.URL.Query()
	if query.Get("clear_filters") == "1" {
		return state, nil
	}
	categoryValues, categoryProvided := query["category"]
	if !categoryProvided {
		if cookie, err := r.Cookie("expense_filters"); err == nil {
			parts := strings.Split(cookie.Value, "|")
			if len(parts) == 4 {
				categoryValues = strings.Split(parts[0], ",")
				if parts[0] == "" {
					categoryValues = nil
				}
				state.dateMode, state.startDate, state.endInput = parts[1], parts[2], parts[3]
			}
		}
	}
	for _, value := range categoryValues {
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id <= 0 {
			return state, fmt.Errorf("invalid category filter")
		}
		state.options.CategoryIDs = append(state.options.CategoryIDs, id)
	}
	if value := query.Get("date_filter"); value != "" {
		state.dateMode = value
		state.startDate = query.Get("start_date")
		state.endInput = query.Get("end_date")
	}
	if state.dateMode == "" {
		state.dateMode = "all"
	}

	now := time.Now()
	switch state.dateMode {
	case "all":
		return state, nil
	case "current":
		state.startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
		state.endDate = time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
		state.endInput = state.endDate
	case "previous":
		first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		state.startDate = first.AddDate(0, -1, 0).Format("2006-01-02")
		state.endDate = first.Format("2006-01-02")
		state.endInput = state.endDate
	case "custom":
		if state.startDate == "" || state.endInput == "" {
			return state, fmt.Errorf("custom date filters require both start and end dates")
		}
		start, startErr := time.Parse("2006-01-02", state.startDate)
		end, endErr := time.Parse("2006-01-02", state.endInput)
		if startErr != nil || endErr != nil || start.After(end) {
			return state, fmt.Errorf("start date must be on or before end date")
		}
		state.endDate = end.AddDate(0, 0, 1).Format("2006-01-02")
	default:
		return state, fmt.Errorf("invalid date filter")
	}
	if state.startDate != "" {
		start, err := time.Parse("2006-01-02", state.startDate)
		if err != nil {
			return state, fmt.Errorf("invalid start date")
		}
		state.options.StartDate = &start
	}
	if state.endDate != "" {
		end, err := time.Parse("2006-01-02", state.endDate)
		if err != nil {
			return state, fmt.Errorf("invalid end date")
		}
		state.options.EndDate = &end
	}
	return state, nil
}

func dashboardSortPreference(r *http.Request) (string, string) {
	sortBy := "date"
	sortDirection := "desc"
	if cookie, err := r.Cookie("expense_sort"); err == nil {
		parts := strings.Split(cookie.Value, ":")
		if len(parts) == 2 {
			sortBy, sortDirection = parts[0], parts[1]
		}
	}
	if value := r.URL.Query().Get("sort"); value != "" {
		sortBy = value
	}
	if value := r.URL.Query().Get("direction"); value != "" {
		sortDirection = value
	}
	return sortBy, sortDirection
}

func (h *UploadHandler) renderDashboard(w http.ResponseWriter, expenses []models.Expense, categoryNames map[int64]string, categories []models.Category, sortBy, sortDirection string, filterState dashboardFilterState, page, totalExpenses int, listQuery string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	rows := ""
	for _, expense := range expenses {
		categoryName := categoryNames[expense.CategoryID]
		if categoryName == "" {
			categoryName = "Sin clasificar"
		}
		detailURL := "/expenses/" + strconv.FormatInt(expense.ID, 10)
		if listQuery != "" {
			detailURL += "?return=" + url.QueryEscape("/?"+listQuery)
		}
		rows += fmt.Sprintf(`<tr class="expense-row"><td><a href="%s">%s</a></td><td><a href="%s">%s</a></td><td>%.2f€</td><td>%.2f€</td><td>%s</td><td>%s</td></tr>`,
			html.EscapeString(detailURL), expense.Date.Format("02/01/2006"), html.EscapeString(detailURL), html.EscapeString(expense.Description), expense.Amount, expense.Balance,
			html.EscapeString(categoryName), html.EscapeString(expense.ConfidenceLevel))
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
		.expense-row { cursor: pointer; }
		.expense-row:hover { background: #f5f9ff; }
		.expense-row a { color: inherit; text-decoration: none; }
		.expense-row a:hover { text-decoration: underline; }
		.table-scroll { overflow-x: auto; }
		.sort-controls { display: flex; gap: 10px; align-items: end; margin: 20px 0; flex-wrap: wrap; }
		.sort-controls label { display: flex; flex-direction: column; gap: 4px; font-size: 13px; }
		.sort-controls select, .sort-controls button { padding: 7px 10px; }
		.actions { display: flex; gap: 10px; margin: 20px 0; flex-wrap: wrap; }
		.actions a { margin-top: 0; padding: 8px 12px; background: #007bff; color: white; text-decoration: none; border-radius: 4px; }
		.actions a:hover { background: #0056b3; }
		.actions .danger { background: #b42318; }
		.actions .danger:hover { background: #8f1d14; }
		.active-sort { background: #e8f0fe; }
		.success-message { padding: 10px 12px; background: #e7f6ec; color: #176b36; border: 1px solid #a7d8b5; }
		.filter-controls { border: 1px solid #ddd; padding: 14px; margin: 12px 0 20px; }
		.category-options { display: flex; gap: 12px; flex-wrap: wrap; margin: 8px 0; }
		.filter-actions { display: flex; gap: 10px; align-items: center; flex-wrap: wrap; }
		.pagination { display: flex; gap: 8px; align-items: center; margin-top: 20px; flex-wrap: wrap; }
		.pagination a { margin-top: 0; padding: 6px 10px; border: 1px solid #ccc; text-decoration: none; }
		.pagination .current { font-weight: bold; background: #e8f0fe; padding: 6px 10px; }
		a { display: inline-block; margin-top: 20px; }
	</style>
</head>
<body>
	<div class="container">
		<h1>Expense Dashboard</h1>
		%s
		<p>%d expenses stored in the database.</p>
		<div class="actions">
			<a href="/upload">Upload another CSV</a>
			<a href="/classification-log">Classification log</a>
			<form method="post" action="/expenses/delete-all" onsubmit="return confirm('Delete all expenses and classification logs?');">
				<input type="hidden" name="confirm" value="delete-all">
				<button class="danger" type="submit">Delete all records</button>
			</form>
		</div>
		<form class="sort-controls" method="get" action="/">
			<label>Sort by
				<select name="sort">
					<option value="date"%s>Date</option>
					<option value="amount"%s>Amount</option>
					<option value="category"%s>Category</option>
					<option value="description"%s>Description</option>
				</select>
			</label>
			<label>Direction
				<select name="direction">
					<option value="asc"%s>Ascending</option>
					<option value="desc"%s>Descending</option>
				</select>
			</label>
			<button type="submit">Apply sort</button>
		</form>
		%s
		<div class="table-scroll">
			<table>
				<thead><tr><th class="%s">Date</th><th class="%s">Description</th><th class="%s">Amount</th><th>Balance</th><th class="%s">Category</th><th>Confidence</th></tr></thead>
				<tbody>%s</tbody>
			</table>
		</div>
		%s
</div>
</body>
		</html>`, dashboardMessage(listQuery), len(expenses), selectedOption(sortBy, "date"), selectedOption(sortBy, "amount"), selectedOption(sortBy, "category"), selectedOption(sortBy, "description"), selectedOption(sortDirection, "asc"), selectedOption(sortDirection, "desc"),
		renderDashboardFilters(categories, filterState),
		activeSortClass(sortBy, "date"), activeSortClass(sortBy, "description"), activeSortClass(sortBy, "amount"), activeSortClass(sortBy, "category"), rows,
		renderPagination(listQuery, page, totalExpenses))
}

func dashboardMessage(listQuery string) string {
	query, _ := url.ParseQuery(listQuery)
	if query.Get("updated") != "1" {
		return ""
	}
	return `<p class="success-message">Category updated successfully.</p>`
}

func selectedOption(current, option string) string {
	if current == option {
		return " selected"
	}
	return ""
}

func activeSortClass(current, option string) string {
	if current == option {
		return "active-sort"
	}
	return ""
}

func renderPagination(listQuery string, page, totalExpenses int) string {
	totalPages := (totalExpenses + dashboardPageSize - 1) / dashboardPageSize
	if totalPages <= 1 {
		return ""
	}
	query, _ := url.ParseQuery(listQuery)
	query.Del("page")
	link := func(targetPage int) string {
		query.Set("page", strconv.Itoa(targetPage))
		return "/?" + query.Encode()
	}
	markup := `<nav class="pagination" aria-label="Expense pages">`
	if page > 1 {
		markup += fmt.Sprintf(`<a href="%s">Previous</a>`, html.EscapeString(link(page-1)))
	}
	start := page - 2
	if start < 1 {
		start = 1
	}
	end := page + 2
	if end > totalPages {
		end = totalPages
	}
	for current := start; current <= end; current++ {
		if current == page {
			markup += fmt.Sprintf(`<span class="current">%d</span>`, current)
		} else {
			markup += fmt.Sprintf(`<a href="%s">%d</a>`, html.EscapeString(link(current)), current)
		}
	}
	if page < totalPages {
		markup += fmt.Sprintf(`<a href="%s">Next</a>`, html.EscapeString(link(page+1)))
	}
	return markup + "</nav>"
}

// HandleGetExpenseDetail renders one expense and its category correction form.
func (h *UploadHandler) HandleGetExpenseDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "Invalid expense ID", http.StatusBadRequest)
		return
	}
	expense, err := h.expenses.GetByID(r.Context(), id)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Failed to load expense", slog.String("error", err.Error()))
		http.Error(w, "Failed to load expense", http.StatusInternalServerError)
		return
	}
	if expense == nil {
		http.NotFound(w, r)
		return
	}
	categories, err := h.categories.GetAllCategories(r.Context())
	if err != nil {
		http.Error(w, "Failed to load categories", http.StatusInternalServerError)
		return
	}
	returnURL := r.URL.Query().Get("return")
	if returnURL == "" || !strings.HasPrefix(returnURL, "/") {
		returnURL = "/"
	}
	h.renderExpenseDetail(w, expense, categories, returnURL, "")
}

// HandlePostExpenseDetail saves a corrected category.
func (h *UploadHandler) HandlePostExpenseDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "Invalid expense ID", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}
	categoryID, err := strconv.ParseInt(r.FormValue("category_id"), 10, 64)
	if err != nil || categoryID <= 0 {
		http.Error(w, "Invalid category", http.StatusBadRequest)
		return
	}
	category, err := h.categories.GetByID(r.Context(), int(categoryID))
	if err != nil {
		http.Error(w, "Failed to validate category", http.StatusInternalServerError)
		return
	}
	if category == nil {
		http.Error(w, "Invalid category", http.StatusBadRequest)
		return
	}
	if err := h.expenses.UpdateCategory(r.Context(), id, categoryID); err != nil {
		h.logger.ErrorContext(r.Context(), "Failed to update expense category", slog.String("error", err.Error()))
		http.Error(w, "Failed to update category", http.StatusInternalServerError)
		return
	}
	returnURL := r.FormValue("return")
	if returnURL == "" || !strings.HasPrefix(returnURL, "/") {
		returnURL = "/"
	}
	returnURL = addQueryParameter(returnURL, "updated", "1")
	http.Redirect(w, r, returnURL, http.StatusSeeOther)
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

func (h *UploadHandler) renderExpenseDetail(w http.ResponseWriter, expense *models.Expense, categories []models.Category, returnURL, message string) {
	options := ""
	for _, category := range categories {
		selected := ""
		if int64(category.ID) == expense.CategoryID {
			selected = " selected"
		}
		options += fmt.Sprintf(`<option value="%d"%s>%s</option>`, category.ID, selected, html.EscapeString(category.DisplayName))
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><title>Expense Detail</title><meta charset="UTF-8">
<style>body{font-family:sans-serif;background:#f5f5f5;padding:20px}.container{max-width:620px;margin:auto;background:#fff;padding:32px}dt{font-weight:bold;margin-top:12px}dd{margin:4px 0}select,button{padding:8px;margin-top:8px}.actions{display:flex;gap:12px;margin-top:24px}.actions a{padding:8px 12px}</style>
</head><body><div class="container"><h1>Expense Detail</h1>
<dl><dt>Date</dt><dd>%s</dd><dt>Description</dt><dd>%s</dd><dt>Amount</dt><dd>%.2f€</dd><dt>Balance</dt><dd>%.2f€</dd><dt>Confidence</dt><dd>%s</dd></dl>
<form method="post"><input type="hidden" name="return" value="%s"><label for="category_id">Category</label><br><select id="category_id" name="category_id">%s</select><br><button type="submit">Save Changes</button></form>
<div class="actions"><a href="%s">Back to List</a></div></div></body></html>`,
		expense.Date.Format("02/01/2006"), html.EscapeString(expense.Description), expense.Amount, expense.Balance,
		html.EscapeString(expense.ConfidenceLevel), html.EscapeString(returnURL), options, html.EscapeString(returnURL))
}

func renderDashboardFilters(categories []models.Category, state dashboardFilterState) string {
	selected := make(map[int64]bool, len(state.options.CategoryIDs))
	for _, id := range state.options.CategoryIDs {
		selected[id] = true
	}
	categoryOptions := ""
	for _, category := range categories {
		checked := ""
		if selected[int64(category.ID)] {
			checked = " checked"
		}
		categoryOptions += fmt.Sprintf(`<label><input type="checkbox" name="category" value="%d"%s> %s</label>`, category.ID, checked, html.EscapeString(category.DisplayName))
	}
	endInput := state.endInput
	if endInput == "" && state.dateMode != "all" {
		endInput = state.endDate
		if endInput != "" {
			if end, err := time.Parse("2006-01-02", endInput); err == nil {
				endInput = end.AddDate(0, 0, -1).Format("2006-01-02")
			}
		}
	}
	dateLabel := "All Dates"
	switch state.dateMode {
	case "current":
		dateLabel = "Current Month"
	case "previous":
		dateLabel = "Previous Month"
	case "custom":
		dateLabel = "Custom Dates"
	}
	return fmt.Sprintf(`<form class="filter-controls" method="get" action="/">
		<input type="hidden" name="sort" value="%s">
		<input type="hidden" name="direction" value="%s">
		<strong>Categories</strong>
		<div class="category-options">%s</div>
		<div class="filter-actions">
			<strong>Date range</strong>
			<label><input type="radio" name="date_filter" value="all"%s> All Dates</label>
			<label><input type="radio" name="date_filter" value="current"%s> Current Month</label>
			<label><input type="radio" name="date_filter" value="previous"%s> Previous Month</label>
			<label><input type="radio" name="date_filter" value="custom"%s> Custom</label>
			<input type="date" name="start_date" value="%s">
			<input type="date" name="end_date" value="%s">
			<button type="submit">Apply filters</button>
			<a href="/?clear_filters=1">Clear filters</a>
		</div>
		<div>Active filters: %d categories, %s</div>
	</form>`, html.EscapeString(state.options.SortBy), html.EscapeString(state.options.SortDirection), categoryOptions,
		selectedOption(state.dateMode, "all"), selectedOption(state.dateMode, "current"), selectedOption(state.dateMode, "previous"), selectedOption(state.dateMode, "custom"),
		html.EscapeString(state.startDate), html.EscapeString(endInput), len(state.options.CategoryIDs), dateLabel)
}

// HandleGetClassificationLog renders automatic classification decisions.
func (h *UploadHandler) HandleGetClassificationLog(w http.ResponseWriter, r *http.Request) {
	if h.expenses == nil {
		h.renderUploadError(w, "Database is not configured", nil)
		return
	}

	logs, err := h.expenses.GetClassificationLogs(r.Context())
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Failed to load classification logs", slog.String("error", err.Error()))
		h.renderUploadError(w, "Failed to load classification logs", nil)
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
func (h *UploadHandler) renderUploadSuccess(w http.ResponseWriter, filename string, expenses []models.Expense, classifications []services.Classification, skippedRows int, savedCount int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	// Build table rows
	tableRows := ""
	for index, expense := range expenses {
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

		<div class="actions">
			<a href="/upload" class="btn-secondary">← Upload Another File</a>
			<a href="/" class="btn-primary">Go to Dashboard →</a>
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
</body>
</html>`, html.EscapeString(filename), len(expenses), skippedRows, savedCount, tableRows)

	fmt.Fprint(w, html)
}
