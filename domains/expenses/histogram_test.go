package expenses

import (
	"testing"
	"time"
)

func TestMondayOfHandlesEveryWeekday(t *testing.T) {
	// 2026-01-05 is a Monday.
	monday := time.Date(2026, 1, 5, 15, 30, 0, 0, time.UTC)
	tests := []struct {
		name     string
		date     time.Time
		wantDate string // YYYY-MM-DD
	}{
		{"Monday itself", monday, "2026-01-05"},
		{"Tuesday", monday.AddDate(0, 0, 1), "2026-01-05"},
		{"Wednesday", monday.AddDate(0, 0, 2), "2026-01-05"},
		{"Thursday", monday.AddDate(0, 0, 3), "2026-01-05"},
		{"Friday", monday.AddDate(0, 0, 4), "2026-01-05"},
		{"Saturday", monday.AddDate(0, 0, 5), "2026-01-05"},
		{"Sunday (last day of the same week)", monday.AddDate(0, 0, 6), "2026-01-05"},
		{"next Monday starts a new week", monday.AddDate(0, 0, 7), "2026-01-12"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := mondayOf(test.date).Format("2006-01-02")
			if got != test.wantDate {
				t.Errorf("mondayOf(%s) = %s, want %s", test.date.Format("2006-01-02 Mon"), got, test.wantDate)
			}
		})
	}
}

func TestMondayOfHandlesYearBoundary(t *testing.T) {
	// 2025-12-29 is a Monday; the week runs into January 2026.
	weekStart := time.Date(2025, 12, 29, 0, 0, 0, 0, time.UTC)
	newYearsDay := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	if got := mondayOf(newYearsDay); !got.Equal(weekStart) {
		t.Fatalf("expected New Year's Day to belong to the week starting %s, got %s", weekStart.Format("2006-01-02"), got.Format("2006-01-02"))
	}
}

func categoryNamesFixture() map[int64]string {
	return map[int64]string{1: "Casa", 2: "Ocio"}
}

func TestBuildWeeklyHistogramGroupsByWeekAndCategory(t *testing.T) {
	monday := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	entries := []SpendingEntry{
		{Date: monday, Amount: 10, CategoryID: 1},                  // week 1, Casa
		{Date: monday.AddDate(0, 0, 3), Amount: 5, CategoryID: 2},  // week 1 (Thursday), Ocio
		{Date: monday.AddDate(0, 0, 7), Amount: 20, CategoryID: 1}, // week 2, Casa
		{Date: monday.AddDate(0, 0, 13), Amount: 7, CategoryID: 1}, // week 2 (Sunday), Casa
	}

	histogram := BuildWeeklyHistogram(entries, categoryNamesFixture(), nil)

	if len(histogram.Periods) != 2 {
		t.Fatalf("expected 2 weekly periods, got %d: %+v", len(histogram.Periods), histogram.Periods)
	}
	week1, week2 := histogram.Periods[0], histogram.Periods[1]
	if week1.Label != "05/01/2026" || week2.Label != "12/01/2026" {
		t.Fatalf("expected chronological week labels 05/01/2026 then 12/01/2026, got %q then %q", week1.Label, week2.Label)
	}
	if week1.Total != 15 {
		t.Errorf("expected week 1 total 15 (10 Casa + 5 Ocio), got %v", week1.Total)
	}
	if len(week1.Bars) != 2 {
		t.Fatalf("expected 2 category bars in week 1, got %d", len(week1.Bars))
	}
	if week2.Total != 27 {
		t.Errorf("expected week 2 total 27 (20 + 7, same category merged), got %v", week2.Total)
	}
	if len(week2.Bars) != 1 || week2.Bars[0].CategoryName != "Casa" || week2.Bars[0].Amount != 27 {
		t.Fatalf("expected a single merged Casa bar of 27 in week 2, got %+v", week2.Bars)
	}
}

func TestBuildMonthlyHistogramGroupsByCalendarMonth(t *testing.T) {
	entries := []SpendingEntry{
		{Date: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC), Amount: 10, CategoryID: 1},
		{Date: time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC), Amount: 5, CategoryID: 1},
		{Date: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), Amount: 8, CategoryID: 2},
	}

	histogram := BuildMonthlyHistogram(entries, categoryNamesFixture(), nil)

	if len(histogram.Periods) != 2 {
		t.Fatalf("expected 2 monthly periods, got %d", len(histogram.Periods))
	}
	if histogram.Periods[0].Label != "Jan 2026" || histogram.Periods[1].Label != "Feb 2026" {
		t.Fatalf("expected Jan 2026 then Feb 2026, got %q then %q", histogram.Periods[0].Label, histogram.Periods[1].Label)
	}
	if histogram.Periods[0].Total != 15 {
		t.Errorf("expected January total 15 (10+5, same month despite spanning most of it), got %v", histogram.Periods[0].Total)
	}
}

func TestBuildHistogramHandlesYearBoundaryAcrossDecemberJanuary(t *testing.T) {
	entries := []SpendingEntry{
		{Date: time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC), Amount: 10, CategoryID: 1},
		{Date: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), Amount: 5, CategoryID: 1}, // same ISO week (Mon Dec 29 - Sun Jan 4)
		{Date: time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC), Amount: 3, CategoryID: 1}, // following week
	}

	weekly := BuildWeeklyHistogram(entries, categoryNamesFixture(), nil)
	if len(weekly.Periods) != 2 {
		t.Fatalf("expected the Dec 31 and Jan 2 entries merged into one week spanning the year boundary, got %d periods: %+v", len(weekly.Periods), weekly.Periods)
	}
	if weekly.Periods[0].Total != 15 {
		t.Errorf("expected the boundary-spanning week total to be 15, got %v", weekly.Periods[0].Total)
	}

	monthly := BuildMonthlyHistogram(entries, categoryNamesFixture(), nil)
	if len(monthly.Periods) != 2 {
		t.Fatalf("expected December and January to remain separate monthly periods, got %d", len(monthly.Periods))
	}
	if monthly.Periods[0].Label != "Dec 2025" || monthly.Periods[1].Label != "Jan 2026" {
		t.Fatalf("expected Dec 2025 then Jan 2026, got %q then %q", monthly.Periods[0].Label, monthly.Periods[1].Label)
	}
}

func TestBuildHistogramAppliesTenPercentHeadroomToMaxTotal(t *testing.T) {
	entries := []SpendingEntry{
		{Date: time.Now(), Amount: 100, CategoryID: 1},
	}
	histogram := BuildWeeklyHistogram(entries, categoryNamesFixture(), nil)
	if diff := histogram.MaxTotal - 110; diff > 0.001 || diff < -0.001 {
		t.Errorf("expected MaxTotal = 100 * 1.1 = 110, got %v", histogram.MaxTotal)
	}
}

func TestBuildHistogramEmptyInputProducesZeroPeriods(t *testing.T) {
	histogram := BuildWeeklyHistogram(nil, categoryNamesFixture(), nil)
	if len(histogram.Periods) != 0 || histogram.MaxTotal != 0 {
		t.Fatalf("expected an empty histogram for no entries, got %+v", histogram)
	}
}

func TestBuildHistogramFallsBackToUnclassifiedName(t *testing.T) {
	entries := []SpendingEntry{{Date: time.Now(), Amount: 10, CategoryID: 999}}
	histogram := BuildWeeklyHistogram(entries, categoryNamesFixture(), nil)
	if len(histogram.Periods) != 1 || histogram.Periods[0].Bars[0].CategoryName != "Sin clasificar" {
		t.Fatalf("expected unknown category to fall back to 'Sin clasificar', got %+v", histogram.Periods)
	}
}
