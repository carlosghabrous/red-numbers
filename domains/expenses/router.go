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

	h.renderDashboard(w, data.Expenses, data.CategoryNames, data.Categories, data.Statistics, data.PieSlices, data.WeeklyHistogram, data.MonthlyHistogram, sortBy, sortDirection, filterState, page, data.TotalExpenses, r.URL.RawQuery, platform.CSRFToken(r))
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
	h.renderExpenseDetail(w, expense, categoryList, returnURL, selfURL, r.URL.Query(), platform.CSRFToken(r))
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

func (h *Handler) renderDashboard(w http.ResponseWriter, expenseList []Expense, categoryNames map[int64]string, categoryList []categories.Category, stats Statistics, pieSlices []PieSlice, weeklyHistogram, monthlyHistogram Histogram, sortBy, sortDirection string, filterState dashboardFilterState, page, totalExpenses int, listQuery, csrfToken string) {
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
	<link rel="stylesheet" href="/static/style.css">
</head>
<body>
	<div class="page">
		<div class="card">
			<h1>Expense Dashboard</h1>
			%s
			<p>%d expenses stored in the database.</p>
			<div class="actions">
				<a class="btn" href="/upload">Upload another CSV</a>
				<a class="btn btn-secondary" href="/classification-log">Classification log</a>
			</div>
			%s
			%s
			%s
			%s
			%s
			<div class="table-actions">
				<form method="post" action="/expenses/delete-all" onsubmit="return confirm('Delete all expenses and classification logs?');">
					<input type="hidden" name="csrf_token" value="%s">
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
	</div>
</body>
</html>`, dashboardMessage(listQuery), len(expenseList),
		renderDashboardFilters(categoryList, filterState),
		renderSummaryStatistics(stats, categoryNames),
		renderPieChart(pieSlices),
		renderHistogram("Weekly Spending", weeklyHistogram),
		renderHistogram("Monthly Spending", monthlyHistogram),
		html.EscapeString(csrfToken),
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

func (h *Handler) renderExpenseDetail(w http.ResponseWriter, expense *Expense, categoryList []categories.Category, returnURL, selfURL string, query url.Values, csrfToken string) {
	options := ""
	for _, category := range categoryList {
		selected := ""
		if int64(category.ID) == expense.CategoryID {
			selected = " selected"
		}
		options += fmt.Sprintf(`<option value="%d"%s>%s</option>`, category.ID, selected, html.EscapeString(category.DisplayName))
	}
	token := html.EscapeString(csrfToken)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><title>Expense Detail</title><meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<link rel="stylesheet" href="/static/style.css">
</head><body><div class="page"><div class="card card--narrow"><h1>Expense Detail</h1>
%s
<dl><dt>Date</dt><dd>%s</dd><dt>Description</dt><dd>%s</dd><dt>Amount</dt><dd>%.2f€</dd><dt>Balance</dt><dd>%.2f€</dd><dt>Confidence</dt><dd>%s</dd></dl>
<form method="post"><input type="hidden" name="csrf_token" value="%s"><input type="hidden" name="return" value="%s"><label for="category_id">Category</label><select id="category_id" name="category_id">%s</select><button class="btn" type="submit">Save Changes</button></form>
<form method="post" action="/categories" class="add-category"><input type="hidden" name="csrf_token" value="%s"><input type="hidden" name="return" value="%s"><label for="display_name">Add a new category</label><input type="text" id="display_name" name="display_name" placeholder="e.g. Mascotas" maxlength="40" required><button class="btn btn-secondary" type="submit">Add category</button></form>
<div class="actions"><a href="%s">Back to List</a></div></div></div></body></html>`,
		categoryFormMessage(query),
		expense.Date.Format("02/01/2006"), html.EscapeString(expense.Description), expense.Amount, expense.Balance,
		html.EscapeString(expense.ConfidenceLevel), token, html.EscapeString(returnURL), options,
		token, html.EscapeString(selfURL), html.EscapeString(returnURL))
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
	case "too_long":
		return `<p class="error-message">Category name must be 40 characters or fewer.</p>`
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

// Histogram bar geometry, shared by the weekly and monthly charts.
const (
	histBarWidth     = 36.0
	histBarGap       = 24.0
	histChartHeight  = 200.0
	histTopPadding   = 10.0
	histLeftPadding  = 60.0
	histBottomLabels = 40.0
	histGridlines    = 4
)

// renderHistogram draws title as a stacked-bar SVG chart: one column per
// period, one colored segment per category. Bars are wrapped in a
// horizontally scrolling container (like the expense table) so many periods
// stay readable on narrow screens instead of shrinking to illegible widths,
// per requirement 15.4's explicit "horizontal scroll or reduce bar width"
// allowance.
func renderHistogram(title string, histogram Histogram) string {
	if len(histogram.Periods) == 0 || histogram.MaxTotal <= 0 {
		return fmt.Sprintf(`<div class="chart-card"><h2 class="chart-title">%s</h2><div class="stats-empty">No spending data in range.</div></div>`, html.EscapeString(title))
	}

	chartWidth := histLeftPadding + float64(len(histogram.Periods))*(histBarWidth+histBarGap) + histBarGap
	chartHeight := histTopPadding + histChartHeight + histBottomLabels
	rawMax := histogram.MaxTotal / 1.1

	gridlines := ""
	for step := 0; step <= histGridlines; step++ {
		fraction := float64(step) / float64(histGridlines)
		y := histTopPadding + histChartHeight*(1-fraction)
		amount := rawMax * fraction
		gridlines += fmt.Sprintf(`<line class="hist-gridline" x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f"/><text class="hist-axis-label" x="%.2f" y="%.2f" text-anchor="end">%.0f€</text>`,
			histLeftPadding, y, chartWidth, y, histLeftPadding-8, y+4, amount)
	}

	bars := ""
	xLabels := ""
	for index, period := range histogram.Periods {
		x := histLeftPadding + histBarGap + float64(index)*(histBarWidth+histBarGap)
		cumulative := 0.0
		for _, bar := range period.Bars {
			if bar.Amount <= 0 {
				continue
			}
			barHeight := bar.Amount / histogram.MaxTotal * histChartHeight
			y := histTopPadding + histChartHeight - cumulative - barHeight
			bars += fmt.Sprintf(`<rect class="hist-bar" x="%.2f" y="%.2f" width="%.2f" height="%.2f" fill="%s"><title>%s · %s: %.2f€</title></rect>`,
				x, y, histBarWidth, barHeight, bar.Color, html.EscapeString(period.Label), html.EscapeString(bar.CategoryName), bar.Amount)
			cumulative += barHeight
		}
		xLabels += fmt.Sprintf(`<text class="hist-axis-label" x="%.2f" y="%.2f" text-anchor="middle">%s</text>`,
			x+histBarWidth/2, histTopPadding+histChartHeight+18, html.EscapeString(period.Label))
	}

	return fmt.Sprintf(`<div class="chart-card">
		<h2 class="chart-title">%s</h2>
		<div class="histogram-scroll">
			<svg class="histogram-chart" viewBox="0 0 %.2f %.2f" width="%.2f" role="img" aria-label="%s">
				<line class="hist-axis" x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f"/>
				%s%s%s
			</svg>
		</div>
	</div>`, html.EscapeString(title), chartWidth, chartHeight, chartWidth, html.EscapeString(title),
		histLeftPadding, histTopPadding, histLeftPadding, histTopPadding+histChartHeight,
		gridlines, bars, xLabels)
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
