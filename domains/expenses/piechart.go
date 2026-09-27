package expenses

import (
	"fmt"
	"math"
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

// pieChartPalette assigns a fixed color per category ID (via categoryColor), so a
// category's color stays the same across reloads regardless of its spending rank.
var pieChartPalette = []string{
	"#4C6EF5", "#f76707", "#37b24d", "#f03e3e",
	"#ae3ec9", "#f59f00", "#0ca678", "#868e96",
}

func categoryColor(categoryID int64) string {
	if categoryID < 0 {
		categoryID = -categoryID
	}
	return pieChartPalette[int(categoryID)%len(pieChartPalette)]
}

const (
	pieChartCenter = 100.0
	pieChartRadius = 90.0
)

// buildPieSlices converts a category breakdown (already sorted highest first) into
// renderable pie slices with percentages and SVG arc path data. Categories with no
// spending never appear in breakdown, so there's no need to filter zero-amount rows here.
func buildPieSlices(breakdown []CategoryAmount, categoryNames map[int64]string) []PieSlice {
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
			Color:        categoryColor(category.CategoryID),
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
