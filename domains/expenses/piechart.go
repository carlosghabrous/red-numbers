package expenses

import (
	"fmt"
	"math"
	"sort"

	"github.com/ghab/red-numbers/domains/categories"
)

// PieSlice is one renderable wedge of the category breakdown pie chart.
type PieSlice struct {
	CategoryID   int64
	CategoryName string
	Amount       float64
	Percentage   float64
	Color        string
	PathData     string
}

// pieChartPalette is a 16-hue categorical palette (blue, brown, violet, forest
// green, orchid, olive, magenta, steel blue, red, green, yellow, rose, cyan,
// orange, aqua, plum), covering up to 16 categories without repeating a color -
// this app's original 8 categories plus a realistic number of user-added ones.
//
// The ordering is deliberately not grouped by hue family: it's the result of a
// local search (scripts in this change's history) that maximizes the worst
// adjacent-pair separation, since the dataviz skill treats slot order as the
// CVD-safety mechanism, not cosmetics. Validated with
// scripts/validate_palette.js (light mode): all hard gates PASS; worst
// adjacent CVD ΔE 7.2 and worst adjacent normal-vision ΔE 26.5 (both in the
// legal range given this app's existing secondary encoding - every slice/bar
// already carries a plain-text legend label, never color alone).
var pieChartPalette = []string{
	"#2a78d6", "#96531f", "#4a3aa7", "#5a8f3a",
	"#a6309c", "#7a9a1e", "#e87ba4", "#3f6aa8",
	"#e34948", "#008300", "#eda100", "#c23558",
	"#0e9bc4", "#eb6834", "#1baf7a", "#6b4c9a",
}

// unclassifiedColor marks an expense with no category (or an unrecognized
// one), kept outside the categorical palette so it never collides with - or
// consumes a slot meant for - a real category.
const unclassifiedColor = "#9ca3af"

// BuildCategoryColorIndex assigns each category a stable color by its rank in
// ascending-ID (creation) order, not by its raw ID. Category IDs can be large
// and sparse (e.g. after repeated test/experimental inserts), and cycling the
// palette by raw ID wastes slots and produces avoidable collisions; ranking
// first packs every category into consecutive slots, using the full palette.
// Because IDs only ever increase, each existing category keeps its color
// forever - only a newly added category can ever change slots (into one not
// yet used by anything else).
func BuildCategoryColorIndex(categoryList []categories.Category) map[int64]string {
	sorted := make([]categories.Category, len(categoryList))
	copy(sorted, categoryList)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	colors := make(map[int64]string, len(sorted))
	for index, category := range sorted {
		colors[int64(category.ID)] = pieChartPalette[index%len(pieChartPalette)]
	}
	return colors
}

// colorForCategory looks up a category's assigned color, falling back to the
// neutral unclassified color for category 0 (uncategorized) or any ID the
// index doesn't recognize.
func colorForCategory(categoryID int64, categoryColors map[int64]string) string {
	if color, ok := categoryColors[categoryID]; ok {
		return color
	}
	return unclassifiedColor
}

const (
	pieChartCenter = 100.0
	pieChartRadius = 90.0
)

// buildPieSlices converts a category breakdown (already sorted highest first) into
// renderable pie slices with percentages and SVG arc path data. Categories with no
// spending never appear in breakdown, so there's no need to filter zero-amount rows here.
func buildPieSlices(breakdown []CategoryAmount, categoryNames map[int64]string, categoryColors map[int64]string) []PieSlice {
	total := 0.0
	for _, category := range breakdown {
		total += category.Amount
	}
	if total <= 0 {
		return nil
	}

	slices := make([]PieSlice, 0, len(breakdown))
	startAngle := 0.0
	for _, category := range breakdown {
		sweep := category.Amount / total * 360

		name := categoryNames[category.CategoryID]
		if name == "" {
			name = "Sin clasificar"
		}

		var pathData string
		if len(breakdown) == 1 {
			pathData = fullCirclePath()
		} else {
			pathData = arcSlicePath(startAngle, startAngle+sweep)
		}

		slices = append(slices, PieSlice{
			CategoryID:   category.CategoryID,
			CategoryName: name,
			Amount:       category.Amount,
			Percentage:   category.Amount / total * 100,
			Color:        colorForCategory(category.CategoryID, categoryColors),
			PathData:     pathData,
		})
		startAngle += sweep
	}
	return slices
}

// arcSlicePath draws one wedge from startAngle to endAngle (degrees, 0 = 12 o'clock,
// increasing clockwise) as an SVG path centered on the chart.
func arcSlicePath(startAngle, endAngle float64) string {
	x1, y1 := arcPoint(startAngle)
	x2, y2 := arcPoint(endAngle)
	largeArc := 0
	if endAngle-startAngle > 180 {
		largeArc = 1
	}
	return fmt.Sprintf("M %.2f,%.2f L %.2f,%.2f A %.2f,%.2f 0 %d,1 %.2f,%.2f Z",
		pieChartCenter, pieChartCenter, x1, y1, pieChartRadius, pieChartRadius, largeArc, x2, y2)
}

// fullCirclePath draws a complete circle as two semicircular arcs; a single 360-degree
// arc collapses to a point since its start and end coordinates are identical.
func fullCirclePath() string {
	top := pieChartCenter - pieChartRadius
	bottom := pieChartCenter + pieChartRadius
	return fmt.Sprintf("M %.2f,%.2f A %.2f,%.2f 0 1,1 %.2f,%.2f A %.2f,%.2f 0 1,1 %.2f,%.2f Z",
		pieChartCenter, top, pieChartRadius, pieChartRadius, pieChartCenter, bottom, pieChartRadius, pieChartRadius, pieChartCenter, top)
}

// arcPoint returns the point on the chart's circle at angleDegrees, measured
// clockwise from 12 o'clock.
func arcPoint(angleDegrees float64) (float64, float64) {
	angleRadians := angleDegrees * math.Pi / 180
	x := pieChartCenter + pieChartRadius*math.Sin(angleRadians)
	y := pieChartCenter - pieChartRadius*math.Cos(angleRadians)
	return x, y
}
