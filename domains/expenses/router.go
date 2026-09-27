package expenses

import (
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ghab/red-numbers/domains/categories"
	"github.com/ghab/red-numbers/platform"
)

const dashboardPageSize = 50

// Handler serves the expense dashboard, detail view, and deletion endpoints.
type Handler struct {
	logger  *slog.Logger
	service *Service
}

// NewHandler creates an expenses handler using injected dependencies and service.
func NewHandler(deps platform.Dependencies, service *Service) *Handler {
	return &Handler{logger: deps.Logger, service: service}
}

// HandleGetDashboard renders all expenses stored in SQLite.
func (h *Handler) HandleGetDashboard(w http.ResponseWriter, r *http.Request) {
	sortBy, sortDirection := dashboardSortPreference(r)
	page, err := dashboardPage(r)
	if err != nil {
		h.renderError(w, err.Error())
		return
	}
	filterState, err := dashboardFilterStateFromRequest(r)
	if err != nil {
		h.renderError(w, err.Error())
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

	data, err := h.service.GetDashboard(r.Context(), filterState.options)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Failed to load dashboard", slog.String("error", err.Error()))
		h.renderError(w, "Failed to load expenses from the database")
		return
	}

	h.renderDashboard(w, data.Expenses, data.CategoryNames, data.Categories, data.Statistics, data.PieSlices, sortBy, sortDirection, filterState, page, data.TotalExpenses, r.URL.RawQuery)
}

// HandlePostDeleteAll removes every imported expense after explicit confirmation.
func (h *Handler) HandlePostDeleteAll(w http.ResponseWriter, r *http.Request) {
	if r.FormValue("confirm") != "delete-all" {
		http.Error(w, "Deletion was not confirmed", http.StatusBadRequest)
		return
	}
	if err := h.service.DeleteAll(r.Context()); err != nil {
		h.logger.ErrorContext(r.Context(), "Failed to delete all expenses", slog.String("error", err.Error()))
		h.renderError(w, "Failed to delete all expenses")
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// HandleGetExpenseDetail renders one expense and its category correction form.
func (h *Handler) HandleGetExpenseDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "Invalid expense ID", http.StatusBadRequest)
		return
	}
	expense, categoryList, err := h.service.GetExpenseDetail(r.Context(), id)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Failed to load expense detail", slog.String("error", err.Error()))
		http.Error(w, "Failed to load expense", http.StatusInternalServerError)
		return
	}
	if expense == nil {
		http.NotFound(w, r)
		return
	}
	returnURL := r.URL.Query().Get("return")
	if returnURL == "" || !strings.HasPrefix(returnURL, "/") {
		returnURL = "/"
	}
	selfURL := "/expenses/" + strconv.FormatInt(id, 10)
	if returnURL != "/" {
		selfURL += "?return=" + url.QueryEscape(returnURL)
	}
	h.renderExpenseDetail(w, expense, categoryList, returnURL, selfURL, r.URL.Query())
}

// HandlePostExpenseDetail saves a corrected category.
func (h *Handler) HandlePostExpenseDetail(w http.ResponseWriter, r *http.Request) {
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
	result, err := h.service.UpdateCategory(r.Context(), id, categoryID)
	if err != nil {
		if IsInvalidCategory(err) {
			http.Error(w, "Invalid category", http.StatusBadRequest)
			return
		}
		h.logger.ErrorContext(r.Context(), "Failed to update expense category", slog.String("error", err.Error()))
		http.Error(w, "Failed to update category", http.StatusInternalServerError)
		return
	}
	returnURL := r.FormValue("return")
	if returnURL == "" || !strings.HasPrefix(returnURL, "/") {
		returnURL = "/"
	}
	returnURL = addQueryParameter(returnURL, "updated", "1")
	if result.ChangedCount > 0 {
		returnURL = addQueryParameter(returnURL, "reclassified", strconv.Itoa(result.ChangedCount))
	}
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

// renderError renders a generic error page for dashboard/detail failures.
func (h *Handler) renderError(w http.ResponseWriter, errMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)
	fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><title>Error - Expense Tracking</title><meta charset="UTF-8"></head>
<body><h1>Something went wrong</h1><p>%s</p><a href="/">Back to dashboard</a></body></html>`, html.EscapeString(errMsg))
}

func (h *Handler) renderDashboard(w http.ResponseWriter, expenseList []Expense, categoryNames map[int64]string, categoryList []categories.Category, stats Statistics, pieSlices []PieSlice, sortBy, sortDirection string, filterState dashboardFilterState, page, totalExpenses int, listQuery string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	rows := ""
	for _, expense := range expenseList {
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
		.sort-header { display: inline-flex; align-items: center; gap: 6px; }
		.sort-arrows { display: inline-flex; flex-direction: column; line-height: 0.7; }
		.sort-arrow { color: #b6b6b6; text-decoration: none; font-size: 10px; }
		.sort-arrow:hover { color: #555; }
		.sort-arrow.active-arrow { color: #007bff; }
		.actions { display: flex; gap: 10px; margin: 20px 0; flex-wrap: wrap; }
		.btn { margin-top: 0; padding: 8px 12px; background: #007bff; color: #fff; text-decoration: none; border-radius: 4px; border: none; font: inherit; cursor: pointer; display: inline-block; }
		.btn:hover { background: #0056b3; }
		.btn-danger { background: #e2685f; }
		.btn-danger:hover { background: #c9564d; }
		.table-actions { display: flex; justify-content: flex-end; margin: 0 0 10px; }
		.success-message { padding: 10px 12px; background: #e7f6ec; color: #176b36; border: 1px solid #a7d8b5; }
		.filter-controls { border: 1px solid #e3e3e3; border-radius: 8px; padding: 20px 24px; margin: 16px 0 24px; background: #fafafa; }
		.filter-row { display: flex; gap: 40px; flex-wrap: wrap; margin-bottom: 16px; }
		.filter-group { display: flex; flex-direction: column; gap: 8px; }
		.filter-label { font-size: 12px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; color: #767676; }
		.multiselect { position: relative; display: inline-block; }
		.multiselect summary { list-style: none; cursor: pointer; padding: 8px 12px; border: 1px solid #d5d5d5; border-radius: 6px; background: #fff; font-size: 13px; }
		.multiselect summary::-webkit-details-marker { display: none; }
		.multiselect summary::after { content: " \25BE"; color: #888; }
		.multiselect[open] summary::after { content: " \25B4"; }
		.multiselect-panel { position: absolute; top: calc(100%% + 6px); left: 0; z-index: 10; background: #fff; border: 1px solid #d5d5d5; border-radius: 8px; padding: 6px; min-width: 240px; box-shadow: 0 8px 24px rgba(0,0,0,0.12); display: flex; flex-direction: column; gap: 1px; max-height: 260px; overflow-y: auto; }
		.multiselect-option { display: flex; align-items: center; gap: 10px; padding: 8px 10px; border-radius: 6px; font-size: 13px; color: #333; cursor: pointer; }
		.multiselect-option:hover { background: #f2f6ff; }
		.multiselect-option input[type="checkbox"] { width: 15px; height: 15px; accent-color: #007bff; cursor: pointer; flex-shrink: 0; }
		.segmented { display: inline-flex; border: 1px solid #d5d5d5; border-radius: 6px; overflow: hidden; background: #fff; width: fit-content; }
		.segmented label { margin: 0; position: relative; }
		.segmented input { position: absolute; opacity: 0; }
		.segmented span { display: block; padding: 7px 12px; font-size: 13px; color: #444; border-right: 1px solid #d5d5d5; cursor: pointer; white-space: nowrap; }
		.segmented label:last-child span { border-right: none; }
		.segmented label:has(input:checked) span { background: #007bff; color: #fff; }
		.segmented input:focus-visible ~ span { outline: 2px solid #007bff; outline-offset: -2px; }
		.date-range-inputs { display: flex; align-items: center; gap: 8px; }
		.date-range-inputs input[type="date"] { padding: 6px 8px; border: 1px solid #d5d5d5; border-radius: 6px; }
		.date-sep { color: #999; font-size: 13px; }
		.filter-footer { display: flex; justify-content: space-between; align-items: center; gap: 12px; flex-wrap: wrap; padding-top: 14px; border-top: 1px solid #e6e6e6; }
		.active-filters { display: flex; gap: 8px; flex-wrap: wrap; align-items: center; font-size: 13px; color: #767676; }
		.filter-chip { background: #e8f0fe; color: #1a56c4; padding: 4px 10px; border-radius: 999px; font-size: 12px; font-weight: 500; }
		.filter-buttons { display: flex; gap: 10px; }
		.btn-ghost { background: transparent; color: #444; border: 1px solid #ccc; }
		.btn-ghost:hover { background: #eee; }
		.stats-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap: 16px; margin: 0 0 20px; }
		.stat-card { border: 1px solid #e3e3e3; border-radius: 8px; padding: 14px 16px; background: #fff; }
		.stat-label { font-size: 12px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; color: #767676; display: block; margin-bottom: 6px; }
		.stat-value { font-size: 22px; font-weight: 700; color: #1a1a1a; }
		.stat-sub { font-size: 13px; color: #767676; margin-top: 2px; }
		.stats-empty { padding: 14px 16px; margin: 0 0 20px; border: 1px solid #e3e3e3; border-radius: 8px; background: #fafafa; color: #767676; font-size: 13px; }
		.chart-card { border: 1px solid #e3e3e3; border-radius: 8px; padding: 20px 24px; margin: 0 0 20px; background: #fff; }
		.chart-title { font-size: 12px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; color: #767676; margin: 0 0 16px; }
		.chart-body { display: flex; gap: 32px; align-items: center; flex-wrap: wrap; }
		.pie-chart { width: 180px; height: 180px; flex-shrink: 0; }
		.pie-slice { stroke: #fff; stroke-width: 1; }
		.chart-legend { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 10px; font-size: 13px; color: #333; min-width: 200px; flex: 1; }
		.chart-legend li { display: flex; align-items: center; gap: 8px; }
		.legend-swatch { width: 12px; height: 12px; border-radius: 3px; flex-shrink: 0; }
		.legend-amount { margin-left: auto; color: #767676; white-space: nowrap; padding-left: 12px; }
		@media (max-width: 600px) {
			.chart-body { flex-direction: column; align-items: stretch; }
			.pie-chart { width: 100%%; max-width: 220px; height: auto; margin: 0 auto; }
		}
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
			<a class="btn" href="/upload">Upload another CSV</a>
			<a class="btn" href="/classification-log">Classification log</a>
		</div>
		%s
		%s
		%s
		<div class="table-actions">
			<form method="post" action="/expenses/delete-all" onsubmit="return confirm('Delete all expenses and classification logs?');">
				<input type="hidden" name="confirm" value="delete-all">
				<button class="btn btn-danger" type="submit">Delete all records</button>
			</form>
		</div>
		<div class="table-scroll">
			<table>
				<thead><tr>%s%s%s<th>Balance</th>%s<th>Confidence</th></tr></thead>
				<tbody>%s</tbody>
			</table>
		</div>
		%s
</div>
</body>
		</html>`, dashboardMessage(listQuery), len(expenseList),
		renderDashboardFilters(categoryList, filterState),
		renderSummaryStatistics(stats, categoryNames),
		renderPieChart(pieSlices),
		renderSortableHeader("date", "Date", listQuery, sortBy, sortDirection),
		renderSortableHeader("description", "Description", listQuery, sortBy, sortDirection),
		renderSortableHeader("amount", "Amount", listQuery, sortBy, sortDirection),
		renderSortableHeader("category", "Category", listQuery, sortBy, sortDirection),
		rows,
		renderPagination(listQuery, page, totalExpenses))
}

// renderSortableHeader renders a table header with clickable ascending/descending
// arrows next to the title, replacing the old separate sort dropdown/button.
func renderSortableHeader(column, label, listQuery, sortBy, sortDirection string) string {
	ascQuery, _ := url.ParseQuery(listQuery)
	ascQuery.Del("page")
	ascQuery.Set("sort", column)
	ascQuery.Set("direction", "asc")

	descQuery, _ := url.ParseQuery(listQuery)
	descQuery.Del("page")
	descQuery.Set("sort", column)
	descQuery.Set("direction", "desc")

	ascClass, descClass := "", ""
	if sortBy == column {
		if sortDirection == "asc" {
			ascClass = " active-arrow"
		} else {
			descClass = " active-arrow"
		}
	}

	return fmt.Sprintf(`<th><span class="sort-header">%s<span class="sort-arrows"><a class="sort-arrow%s" href="/?%s" aria-label="Sort %s ascending">&#9650;</a><a class="sort-arrow%s" href="/?%s" aria-label="Sort %s descending">&#9660;</a></span></span></th>`,
		html.EscapeString(label), ascClass, ascQuery.Encode(), html.EscapeString(label), descClass, descQuery.Encode(), html.EscapeString(label))
}

func dashboardMessage(listQuery string) string {
	query, _ := url.ParseQuery(listQuery)
	if query.Get("updated") != "1" {
		return ""
	}
	message := "Category updated successfully."
	if reclassified := query.Get("reclassified"); reclassified != "" {
		message += fmt.Sprintf(" %s similar expense(s) were also re-classified.", html.EscapeString(reclassified))
	}
	return fmt.Sprintf(`<p class="success-message">%s</p>`, message)
}

func selectedOption(current, option string) string {
	if current == option {
		return " selected"
	}
	return ""
}

func checkedAttr(checked bool) string {
	if checked {
		return " checked"
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

func (h *Handler) renderExpenseDetail(w http.ResponseWriter, expense *Expense, categoryList []categories.Category, returnURL, selfURL string, query url.Values) {
	options := ""
	for _, category := range categoryList {
		selected := ""
		if int64(category.ID) == expense.CategoryID {
			selected = " selected"
		}
		options += fmt.Sprintf(`<option value="%d"%s>%s</option>`, category.ID, selected, html.EscapeString(category.DisplayName))
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><title>Expense Detail</title><meta charset="UTF-8">
<style>body{font-family:sans-serif;background:#f5f5f5;padding:20px}.container{max-width:620px;margin:auto;background:#fff;padding:32px}dt{font-weight:bold;margin-top:12px}dd{margin:4px 0}select,button,input[type=text]{padding:8px;margin-top:8px}.actions{display:flex;gap:12px;margin-top:24px}.actions a{padding:8px 12px}.add-category{margin-top:20px;padding-top:16px;border-top:1px solid #ddd}.add-category label{font-size:13px;color:#555}.success-message{padding:10px 12px;background:#e7f6ec;color:#176b36;border:1px solid #a7d8b5}.error-message{padding:10px 12px;background:#fdecea;color:#a33a31;border:1px solid #f3b7b0}</style>
</head><body><div class="container"><h1>Expense Detail</h1>
%s
<dl><dt>Date</dt><dd>%s</dd><dt>Description</dt><dd>%s</dd><dt>Amount</dt><dd>%.2f€</dd><dt>Balance</dt><dd>%.2f€</dd><dt>Confidence</dt><dd>%s</dd></dl>
<form method="post"><input type="hidden" name="return" value="%s"><label for="category_id">Category</label><br><select id="category_id" name="category_id">%s</select><br><button type="submit">Save Changes</button></form>
<form method="post" action="/categories" class="add-category"><input type="hidden" name="return" value="%s"><label for="display_name">Add a new category</label><br><input type="text" id="display_name" name="display_name" placeholder="e.g. Mascotas" required><button type="submit">Add category</button></form>
<div class="actions"><a href="%s">Back to List</a></div></div></body></html>`,
		categoryFormMessage(query),
		expense.Date.Format("02/01/2006"), html.EscapeString(expense.Description), expense.Amount, expense.Balance,
		html.EscapeString(expense.ConfidenceLevel), html.EscapeString(returnURL), options,
		html.EscapeString(selfURL), html.EscapeString(returnURL))
}

// categoryFormMessage renders a success/error banner after an "Add a new
// category" submission redirects back to the expense detail page.
func categoryFormMessage(query url.Values) string {
	if added := query.Get("category_added"); added != "" {
		return fmt.Sprintf(`<p class="success-message">Category %q added. Select it above and save to apply it.</p>`, added)
	}
	switch query.Get("category_error") {
	case "empty_name":
		return `<p class="error-message">Category name cannot be empty.</p>`
	case "duplicate":
		return `<p class="error-message">A category with that name already exists.</p>`
	case "failed":
		return `<p class="error-message">Failed to add category.</p>`
	default:
		return ""
	}
}

// renderSummaryStatistics renders the total/count/average/top-category widget shown
// above the expense table. It reflects whatever filters are currently applied.
func renderSummaryStatistics(stats Statistics, categoryNames map[int64]string) string {
	if stats.TransactionCount == 0 {
		return `<div class="stats-empty">No expenses match the current filters.</div>`
	}
	topCategoryCard := `<div class="stat-card"><span class="stat-label">Top Category</span><span class="stat-value">—</span></div>`
	if stats.TopCategoryAmount > 0 {
		topCategoryName := categoryNames[stats.TopCategoryID]
		if topCategoryName == "" {
			topCategoryName = "Sin clasificar"
		}
		topCategoryCard = fmt.Sprintf(`<div class="stat-card"><span class="stat-label">Top Category</span><span class="stat-value">%s</span><div class="stat-sub">%.2f€</div></div>`,
			html.EscapeString(topCategoryName), stats.TopCategoryAmount)
	}
	return fmt.Sprintf(`<div class="stats-grid">
		<div class="stat-card"><span class="stat-label">Total Spending</span><span class="stat-value">%.2f€</span></div>
		<div class="stat-card"><span class="stat-label">Transactions</span><span class="stat-value">%d</span></div>
		<div class="stat-card"><span class="stat-label">Average Spend</span><span class="stat-value">%.2f€</span></div>
		%s
	</div>`, stats.TotalSpending, stats.TransactionCount, stats.AverageSpending, topCategoryCard)
}

// renderPieChart renders the category breakdown pie chart and its legend. Each
// slice carries a native SVG <title> tooltip, so hover details work without JS.
func renderPieChart(slices []PieSlice) string {
	if len(slices) == 0 {
		return `<div class="stats-empty">No spending data to chart.</div>`
	}
	paths := ""
	legend := ""
	for _, slice := range slices {
		paths += fmt.Sprintf(`<path class="pie-slice" d="%s" fill="%s"><title>%s: %.2f€ (%.1f%%)</title></path>`,
			slice.PathData, slice.Color, html.EscapeString(slice.CategoryName), slice.Amount, slice.Percentage)
		legend += fmt.Sprintf(`<li><span class="legend-swatch" style="background:%s"></span>%s<span class="legend-amount">%.2f€ · %.1f%%</span></li>`,
			slice.Color, html.EscapeString(slice.CategoryName), slice.Amount, slice.Percentage)
	}
	return fmt.Sprintf(`<div class="chart-card">
		<h2 class="chart-title">Spending by Category</h2>
		<div class="chart-body">
			<svg class="pie-chart" viewBox="0 0 200 200" role="img" aria-label="Spending by category">%s</svg>
			<ul class="chart-legend">%s</ul>
		</div>
	</div>`, paths, legend)
}

func renderDashboardFilters(categoryList []categories.Category, state dashboardFilterState) string {
	selected := make(map[int64]bool, len(state.options.CategoryIDs))
	for _, id := range state.options.CategoryIDs {
		selected[id] = true
	}
	categoryOptions := ""
	for _, category := range categoryList {
		checked := ""
		if selected[int64(category.ID)] {
			checked = " checked"
		}
		categoryOptions += fmt.Sprintf(`<label class="multiselect-option"><input type="checkbox" name="category" value="%d"%s> %s</label>`, category.ID, checked, html.EscapeString(category.DisplayName))
	}
	categorySummary := "All categories"
	switch len(state.options.CategoryIDs) {
	case 0:
	case 1:
		categorySummary = "1 category selected"
	default:
		categorySummary = fmt.Sprintf("%d categories selected", len(state.options.CategoryIDs))
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
		<div class="filter-row">
			<div class="filter-group">
				<span class="filter-label">Categories</span>
				<details class="multiselect">
					<summary>%s</summary>
					<div class="multiselect-panel">%s</div>
				</details>
			</div>
			<div class="filter-group">
				<span class="filter-label">Date range</span>
				<div class="segmented">
					<label><input type="radio" name="date_filter" value="all"%s><span>All Dates</span></label>
					<label><input type="radio" name="date_filter" value="current"%s><span>Current Month</span></label>
					<label><input type="radio" name="date_filter" value="previous"%s><span>Previous Month</span></label>
				</div>
				<div class="date-range-inputs">
					<input type="date" name="start_date" value="%s">
					<span class="date-sep">to</span>
					<input type="date" name="end_date" value="%s">
				</div>
			</div>
		</div>
		<div class="filter-footer">
			<div class="active-filters">%s</div>
			<div class="filter-buttons">
				<a class="btn btn-ghost" href="/?clear_filters=1">Clear filters</a>
				<button class="btn" type="submit">Apply filters</button>
			</div>
		</div>
	</form>`, html.EscapeString(state.options.SortBy), html.EscapeString(state.options.SortDirection), categorySummary, categoryOptions,
		checkedAttr(state.dateMode == "all" || state.dateMode == "custom"), checkedAttr(state.dateMode == "current"), checkedAttr(state.dateMode == "previous"),
		html.EscapeString(state.startDate), html.EscapeString(endInput),
		renderActiveFilterChips(len(state.options.CategoryIDs), state.dateMode, dateLabel))
}

func renderActiveFilterChips(categoryCount int, dateMode, dateLabel string) string {
	chips := ""
	if categoryCount == 1 {
		chips += `<span class="filter-chip">1 category</span>`
	} else if categoryCount > 1 {
		chips += fmt.Sprintf(`<span class="filter-chip">%d categories</span>`, categoryCount)
	}
	if dateMode != "all" {
		chips += fmt.Sprintf(`<span class="filter-chip">%s</span>`, html.EscapeString(dateLabel))
	}
	if chips == "" {
		return "No filters applied"
	}
	return chips
}
