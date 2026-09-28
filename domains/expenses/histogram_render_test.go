package expenses

import (
	"fmt"
	"strings"
	"testing"
)

func TestRenderHistogramEmptyStateWhenNoData(t *testing.T) {
	got := renderHistogram("Weekly Spending", Histogram{})
	if !strings.Contains(got, "No spending data in range.") || !strings.Contains(got, "Weekly Spending") {
		t.Fatalf("expected an empty-state message naming the chart title, got: %s", got)
	}
	if strings.Contains(got, "<svg") {
		t.Fatalf("expected no SVG to be rendered for an empty histogram, got: %s", got)
	}
}

func TestRenderHistogramDrawsOneBarPerCategoryPerPeriod(t *testing.T) {
	histogram := Histogram{
		Periods: []HistogramPeriod{
			{Label: "05/01/2026", Total: 30, Bars: []HistogramBar{
				{CategoryID: 1, CategoryName: "Casa", Amount: 20, Color: "#4C6EF5"},
				{CategoryID: 2, CategoryName: "Ocio", Amount: 10, Color: "#f76707"},
			}},
		},
		MaxTotal: 33,
	}
	got := renderHistogram("Weekly Spending", histogram)

	if !strings.Contains(got, `<svg class="histogram-chart"`) {
		t.Fatalf("expected an SVG histogram chart, got: %s", got)
	}
	if strings.Count(got, `class="hist-bar"`) != 2 {
		t.Fatalf("expected exactly 2 bar segments (one per category), got: %s", got)
	}
	if !strings.Contains(got, "05/01/2026 · Casa: 20.00€") || !strings.Contains(got, "05/01/2026 · Ocio: 10.00€") {
		t.Fatalf("expected native tooltips naming the period, category, and amount, got: %s", got)
	}
	if !strings.Contains(got, `fill="#4C6EF5"`) || !strings.Contains(got, `fill="#f76707"`) {
		t.Fatalf("expected each bar to use its category's color, got: %s", got)
	}
	if !strings.Contains(got, `class="histogram-scroll"`) {
		t.Fatalf("expected the chart wrapped in a horizontally-scrolling container for mobile, got: %s", got)
	}
}

func TestRenderHistogramSkipsZeroAmountBars(t *testing.T) {
	histogram := Histogram{
		Periods: []HistogramPeriod{
			{Label: "05/01/2026", Total: 10, Bars: []HistogramBar{
				{CategoryID: 1, CategoryName: "Casa", Amount: 10, Color: "#4C6EF5"},
				{CategoryID: 2, CategoryName: "Ocio", Amount: 0, Color: "#f76707"},
			}},
		},
		MaxTotal: 11,
	}
	got := renderHistogram("Weekly Spending", histogram)
	if strings.Count(got, `class="hist-bar"`) != 1 {
		t.Fatalf("expected the zero-amount category to be skipped, got: %s", got)
	}
}

func TestRenderHistogramIncludesGridlinesAndAxisLabels(t *testing.T) {
	histogram := Histogram{
		Periods:  []HistogramPeriod{{Label: "Jan 2026", Total: 100, Bars: []HistogramBar{{CategoryID: 1, CategoryName: "Casa", Amount: 100, Color: "#4C6EF5"}}}},
		MaxTotal: 110,
	}
	got := renderHistogram("Monthly Spending", histogram)

	wantGridlines := histGridlines + 1 // histGridlines divisions means histGridlines+1 lines (0%..100% inclusive)
	if strings.Count(got, `class="hist-gridline"`) != wantGridlines {
		t.Fatalf("expected %d gridlines, got body: %s", wantGridlines, got)
	}
	if !strings.Contains(got, "Jan 2026") {
		t.Fatalf("expected the period label on the x-axis, got: %s", got)
	}
	if !strings.Contains(got, "100€") {
		t.Fatalf("expected a y-axis label at the raw (unpadded) max of 100€, got: %s", got)
	}
}

func TestRenderHistogramWidthGrowsWithPeriodCount(t *testing.T) {
	onePeriod := Histogram{
		Periods:  []HistogramPeriod{{Label: "P1", Total: 10, Bars: []HistogramBar{{CategoryID: 1, CategoryName: "Casa", Amount: 10, Color: "#4C6EF5"}}}},
		MaxTotal: 11,
	}
	fivePeriods := Histogram{MaxTotal: 11}
	for i := 0; i < 5; i++ {
		fivePeriods.Periods = append(fivePeriods.Periods, HistogramPeriod{Label: "P", Total: 10, Bars: []HistogramBar{{CategoryID: 1, CategoryName: "Casa", Amount: 10, Color: "#4C6EF5"}}})
	}

	oneWidth := extractViewBoxWidth(t, renderHistogram("t", onePeriod))
	fiveWidth := extractViewBoxWidth(t, renderHistogram("t", fivePeriods))

	if fiveWidth <= oneWidth {
		t.Fatalf("expected chart width to grow with more periods: one=%v five=%v", oneWidth, fiveWidth)
	}
	wantFiveWidth := histLeftPadding + 5*(histBarWidth+histBarGap) + histBarGap
	if fiveWidth != wantFiveWidth {
		t.Fatalf("expected width %v for 5 periods, got %v", wantFiveWidth, fiveWidth)
	}
}

func extractViewBoxWidth(t *testing.T, svg string) float64 {
	t.Helper()
	start := strings.Index(svg, `viewBox="0 0 `)
	if start == -1 {
		t.Fatalf("no viewBox found in: %s", svg)
	}
	rest := svg[start+len(`viewBox="0 0 `):]
	end := strings.Index(rest, " ")
	var width float64
	if _, err := fmt.Sscan(rest[:end], &width); err != nil {
		t.Fatalf("failed to parse viewBox width from %q: %v", rest[:end], err)
	}
	return width
}
