package expenses

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDashboardFilterStateCurrentMonth(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/?date_filter=current", nil)
	state, err := dashboardFilterStateFromRequest(request)
	if err != nil {
		t.Fatalf("dashboardFilterStateFromRequest failed: %v", err)
	}

	// The production code formats dates as "YYYY-MM-DD" and re-parses them
	// with time.Parse, which defaults to UTC regardless of the server's
	// local timezone — so expectations here must be built in UTC too.
	now := time.Now()
	wantStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	wantEndExclusive := wantStart.AddDate(0, 1, 0)

	if state.options.StartDate == nil || !state.options.StartDate.Equal(wantStart) {
		t.Errorf("expected StartDate %v, got %v", wantStart, state.options.StartDate)
	}
	if state.options.EndDate == nil || !state.options.EndDate.Equal(wantEndExclusive) {
		t.Errorf("expected EndDate (exclusive) %v, got %v", wantEndExclusive, state.options.EndDate)
	}
	// endInput must be the inclusive last day of the month (not the exclusive
	// boundary used for the SQL range, and not left for a stale value carried
	// over from a previously-applied filter to leak through).
	wantEndInput := wantEndExclusive.AddDate(0, 0, -1).Format("2006-01-02")
	if state.endInput != wantEndInput {
		t.Errorf("expected endInput %q, got %q", wantEndInput, state.endInput)
	}
}

func TestDashboardFilterStatePreviousMonth(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/?date_filter=previous", nil)
	state, err := dashboardFilterStateFromRequest(request)
	if err != nil {
		t.Fatalf("dashboardFilterStateFromRequest failed: %v", err)
	}

	now := time.Now()
	thisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	wantStart := thisMonth.AddDate(0, -1, 0)

	if state.options.StartDate == nil || !state.options.StartDate.Equal(wantStart) {
		t.Errorf("expected StartDate %v, got %v", wantStart, state.options.StartDate)
	}
	if state.options.EndDate == nil || !state.options.EndDate.Equal(thisMonth) {
		t.Errorf("expected EndDate (exclusive) %v, got %v", thisMonth, state.options.EndDate)
	}
	wantEndInput := thisMonth.AddDate(0, 0, -1).Format("2006-01-02")
	if state.endInput != wantEndInput {
		t.Errorf("expected endInput %q, got %q", wantEndInput, state.endInput)
	}
}

// TestDashboardFilterStateDoesNotLeakStaleEndDateBetweenPresets reproduces the
// follow-up bug report: switching directly from one preset (whose end_date
// input is now populated) to another must not let the previous preset's
// end_date value leak into the new one. A browser auto-submitting the form
// on preset change will include whatever is currently in the end_date field
// alongside the new date_filter value, so the handler must not trust it.
func TestDashboardFilterStateDoesNotLeakStaleEndDateBetweenPresets(t *testing.T) {
	now := time.Now()
	thisMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	wantPreviousEndInput := thisMonthStart.AddDate(0, 0, -1).Format("2006-01-02")

	// Simulate a browser submitting "previous" while the form still carries
	// the start/end dates that were on screen from a prior "current" selection.
	staleStart := thisMonthStart.Format("2006-01-02")
	staleEnd := thisMonthStart.AddDate(0, 1, -1).Format("2006-01-02")
	request := httptest.NewRequest(http.MethodGet, "/?date_filter=previous&start_date="+staleStart+"&end_date="+staleEnd, nil)

	state, err := dashboardFilterStateFromRequest(request)
	if err != nil {
		t.Fatalf("dashboardFilterStateFromRequest failed: %v", err)
	}
	if state.endInput != wantPreviousEndInput {
		t.Fatalf("expected the stale end_date to be overridden with %q, got %q", wantPreviousEndInput, state.endInput)
	}
	wantEndExclusive := thisMonthStart
	if state.options.EndDate == nil || !state.options.EndDate.Equal(wantEndExclusive) {
		t.Fatalf("expected the query's EndDate to be the previous month's exclusive boundary %v, got %v", wantEndExclusive, state.options.EndDate)
	}
}

// TestDashboardFiltersRenderInclusiveEndDate verifies the actual bug report:
// the "Current Month"/"Previous Month" presets must populate the custom date
// fields with the real (inclusive) last day of the month, not the exclusive
// boundary used internally for the SQL range.
func TestDashboardFiltersRenderInclusiveEndDate(t *testing.T) {
	now := time.Now()
	thisMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	lastDayOfThisMonth := thisMonthStart.AddDate(0, 1, 0).AddDate(0, 0, -1).Format("2006-01-02")
	lastDayOfPrevMonth := thisMonthStart.AddDate(0, 0, -1).Format("2006-01-02")

	tests := []struct {
		dateFilter  string
		wantEndDate string
	}{
		{"current", lastDayOfThisMonth},
		{"previous", lastDayOfPrevMonth},
	}
	for _, test := range tests {
		t.Run(test.dateFilter, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/?date_filter="+test.dateFilter, nil)
			state, err := dashboardFilterStateFromRequest(request)
			if err != nil {
				t.Fatalf("dashboardFilterStateFromRequest failed: %v", err)
			}
			rendered := renderDashboardFilters(nil, state)
			wantAttr := `name="end_date" value="` + test.wantEndDate + `"`
			if !strings.Contains(rendered, wantAttr) {
				t.Fatalf("expected end_date input to show the inclusive last day %q, got: %s", test.wantEndDate, rendered)
			}
		})
	}
}
