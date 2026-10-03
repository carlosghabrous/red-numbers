package expenses

import (
	"sort"
	"time"
)

// HistogramBar is one category's stacked segment within a single period
// (week or month) of a histogram.
type HistogramBar struct {
	CategoryID   int64
	CategoryName string
	Amount       float64
	Color        string
}

// HistogramPeriod is one x-axis column (a week or a month) of a histogram,
// made up of one stacked bar per category that had spending in that period.
type HistogramPeriod struct {
	Label string
	Bars  []HistogramBar
	Total float64
}

// Histogram is a complete, chronologically-sorted set of periods ready to
// render as a stacked bar chart. MaxTotal is the tallest period's total plus
// 10% headroom, per the design doc, so the y-axis scale isn't touching the
// top of the chart.
type Histogram struct {
	Periods  []HistogramPeriod
	MaxTotal float64
}

// periodKeyFunc maps an expense date to the period it belongs to: a stable
// sort/group key, a display label, and a time used to order periods
// chronologically.
type periodKeyFunc func(time.Time) (key, label string, sortTime time.Time)

// BuildWeeklyHistogram aggregates spending entries into Monday-Sunday weekly
// periods. Using each week's actual Monday date (rather than a year+week-number
// pair) as the grouping key avoids ambiguity at year boundaries, where a
// single week can span two different years.
func BuildWeeklyHistogram(entries []SpendingEntry, categoryNames map[int64]string, categoryColors map[int64]string) Histogram {
	return buildHistogram(entries, categoryNames, categoryColors, func(date time.Time) (string, string, time.Time) {
		monday := mondayOf(date)
		return monday.Format("2006-01-02"), monday.Format("02/01/2006"), monday
	})
}

// BuildMonthlyHistogram aggregates spending entries into calendar-month periods.
func BuildMonthlyHistogram(entries []SpendingEntry, categoryNames map[int64]string, categoryColors map[int64]string) Histogram {
	return buildHistogram(entries, categoryNames, categoryColors, func(date time.Time) (string, string, time.Time) {
		firstOfMonth := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, date.Location())
		return firstOfMonth.Format("2006-01"), firstOfMonth.Format("Jan 2006"), firstOfMonth
	})
}

// mondayOf returns the Monday (midnight) of the week containing date.
func mondayOf(date time.Time) time.Time {
	date = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	// time.Weekday: Sunday=0, Monday=1, ..., Saturday=6. Shift so Monday=0.
	offset := (int(date.Weekday()) + 6) % 7
	return date.AddDate(0, 0, -offset)
}

func buildHistogram(entries []SpendingEntry, categoryNames map[int64]string, categoryColors map[int64]string, periodKey periodKeyFunc) Histogram {
	type periodAccumulator struct {
		label      string
		sortTime   time.Time
		byCategory map[int64]float64
	}

	periods := make(map[string]*periodAccumulator)
	for _, entry := range entries {
		key, label, sortTime := periodKey(entry.Date)
		period, exists := periods[key]
		if !exists {
			period = &periodAccumulator{label: label, sortTime: sortTime, byCategory: make(map[int64]float64)}
			periods[key] = period
		}
		period.byCategory[entry.CategoryID] += entry.Amount
	}

	keys := make([]string, 0, len(periods))
	for key := range periods {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return periods[keys[i]].sortTime.Before(periods[keys[j]].sortTime)
	})

	histogram := Histogram{Periods: make([]HistogramPeriod, 0, len(keys))}
	maxTotal := 0.0
	for _, key := range keys {
		period := periods[key]

		categoryIDs := make([]int64, 0, len(period.byCategory))
		for categoryID := range period.byCategory {
			categoryIDs = append(categoryIDs, categoryID)
		}
		sort.Slice(categoryIDs, func(i, j int) bool { return categoryIDs[i] < categoryIDs[j] })

		bars := make([]HistogramBar, 0, len(categoryIDs))
		total := 0.0
		for _, categoryID := range categoryIDs {
			amount := period.byCategory[categoryID]
			name := categoryNames[categoryID]
			if name == "" {
				name = "Sin clasificar"
			}
			bars = append(bars, HistogramBar{
				CategoryID:   categoryID,
				CategoryName: name,
				Amount:       amount,
				Color:        colorForCategory(categoryID, categoryColors),
			})
			total += amount
		}
		if total > maxTotal {
			maxTotal = total
		}
		histogram.Periods = append(histogram.Periods, HistogramPeriod{Label: period.label, Bars: bars, Total: total})
	}

	histogram.MaxTotal = maxTotal * 1.1 // 10% headroom, per design.md
	return histogram
}
