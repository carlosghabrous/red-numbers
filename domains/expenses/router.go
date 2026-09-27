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

	h.renderDashboard(w, data.Expenses, data.CategoryNames, data.Categories, sortBy, sortDirection, filterState, page, data.TotalExpenses, r.URL.RawQuery)
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
	h.renderExpenseDetail(w, expense, categoryList, returnURL)
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
	if err := h.service.UpdateCategory(r.Context(), id, categoryID); err != nil {
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

func (h *Handler) renderDashboard(w http.ResponseWriter, expenseList []Expense, categoryNames map[int64]string, categoryList []categories.Category, sortBy, sortDirection string, filterState dashboardFilterState, page, totalExpenses int, listQuery string) {
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
		</html>`, dashboardMessage(listQuery), len(expenseList), selectedOption(sortBy, "date"), selectedOption(sortBy, "amount"), selectedOption(sortBy, "category"), selectedOption(sortBy, "description"), selectedOption(sortDirection, "asc"), selectedOption(sortDirection, "desc"),
		renderDashboardFilters(categoryList, filterState),
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

func (h *Handler) renderExpenseDetail(w http.ResponseWriter, expense *Expense, categoryList []categories.Category, returnURL string) {
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
<style>body{font-family:sans-serif;background:#f5f5f5;padding:20px}.container{max-width:620px;margin:auto;background:#fff;padding:32px}dt{font-weight:bold;margin-top:12px}dd{margin:4px 0}select,button{padding:8px;margin-top:8px}.actions{display:flex;gap:12px;margin-top:24px}.actions a{padding:8px 12px}</style>
</head><body><div class="container"><h1>Expense Detail</h1>
<dl><dt>Date</dt><dd>%s</dd><dt>Description</dt><dd>%s</dd><dt>Amount</dt><dd>%.2f€</dd><dt>Balance</dt><dd>%.2f€</dd><dt>Confidence</dt><dd>%s</dd></dl>
<form method="post"><input type="hidden" name="return" value="%s"><label for="category_id">Category</label><br><select id="category_id" name="category_id">%s</select><br><button type="submit">Save Changes</button></form>
<div class="actions"><a href="%s">Back to List</a></div></div></body></html>`,
		expense.Date.Format("02/01/2006"), html.EscapeString(expense.Description), expense.Amount, expense.Balance,
		html.EscapeString(expense.ConfidenceLevel), html.EscapeString(returnURL), options, html.EscapeString(returnURL))
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
