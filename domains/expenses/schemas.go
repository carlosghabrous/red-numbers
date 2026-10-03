package expenses

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// dashboardPage parses and validates the "page" query parameter.
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

// dashboardFilterState is the parsed and validated representation of the
// dashboard's category and date range filters, sourced from query params or cookies.
type dashboardFilterState struct {
	options   FilterOptions
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
		if state.dateMode == "all" && (state.startDate != "" || state.endInput != "") {
			state.dateMode = "custom"
		}
	}
	if state.dateMode == "" {
		state.dateMode = "all"
	}

	now := time.Now()
	switch state.dateMode {
	case "all":
		return state, nil
	case "current":
		monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		nextMonthStart := monthStart.AddDate(0, 1, 0)
		state.startDate = monthStart.Format("2006-01-02")
		state.endDate = nextMonthStart.Format("2006-01-02") // exclusive, used for the SQL range
		// endInput is explicitly overwritten (not left blank) so a stale
		// end_date carried over from whatever filter was active before this
		// one was selected can never leak into the displayed field.
		state.endInput = nextMonthStart.AddDate(0, 0, -1).Format("2006-01-02") // inclusive, for display
	case "previous":
		monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		previousMonthStart := monthStart.AddDate(0, -1, 0)
		state.startDate = previousMonthStart.Format("2006-01-02")
		state.endDate = monthStart.Format("2006-01-02")                    // exclusive
		state.endInput = monthStart.AddDate(0, 0, -1).Format("2006-01-02") // inclusive, for display
	case "custom":
		if state.startDate == "" || state.endInput == "" {
			return state, fmt.Errorf("both a start date and an end date are required to filter by date range")
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
