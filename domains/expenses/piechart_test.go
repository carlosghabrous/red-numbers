package expenses

import (
	"math"
	"testing"
)

func TestBuildPieSlicesCalculatesPercentages(t *testing.T) {
	breakdown := []CategoryAmount{
		{CategoryID: 1, Amount: 75},
		{CategoryID: 2, Amount: 25},
	}
	categoryNames := map[int64]string{1: "Casa", 2: "Ocio"}

	slices := buildPieSlices(breakdown, categoryNames)
	if len(slices) != 2 {
		t.Fatalf("expected 2 slices, got %d", len(slices))
	}
	if slices[0].CategoryName != "Casa" || slices[0].Percentage != 75 {
		t.Errorf("expected Casa at 75%%, got %+v", slices[0])
	}
	if slices[1].CategoryName != "Ocio" || slices[1].Percentage != 25 {
		t.Errorf("expected Ocio at 25%%, got %+v", slices[1])
	}
}

func TestBuildPieSlicesFallsBackToUnclassifiedName(t *testing.T) {
	breakdown := []CategoryAmount{{CategoryID: 0, Amount: 10}}
	slices := buildPieSlices(breakdown, map[int64]string{})
	if len(slices) != 1 || slices[0].CategoryName != "Sin clasificar" {
		t.Fatalf("expected fallback category name, got %+v", slices)
	}
}

func TestBuildPieSlicesReturnsNilForNoSpending(t *testing.T) {
	if slices := buildPieSlices(nil, nil); slices != nil {
		t.Fatalf("expected nil slices for empty breakdown, got %+v", slices)
	}
	if slices := buildPieSlices([]CategoryAmount{{CategoryID: 1, Amount: 0}}, nil); slices != nil {
		t.Fatalf("expected nil slices when total spending is zero, got %+v", slices)
	}
}

func TestBuildPieSlicesAssignsStableColorsIndependentOfRank(t *testing.T) {
	ascending := buildPieSlices([]CategoryAmount{
		{CategoryID: 5, Amount: 10},
		{CategoryID: 3, Amount: 90},
	}, nil)
	descending := buildPieSlices([]CategoryAmount{
		{CategoryID: 3, Amount: 90},
		{CategoryID: 5, Amount: 10},
	}, nil)

	colorFor := func(slices []PieSlice, categoryID int64) string {
		for _, slice := range slices {
			if slice.CategoryID == categoryID {
				return slice.Color
			}
		}
		t.Fatalf("category %d not found", categoryID)
		return ""
	}
	if colorFor(ascending, 5) != colorFor(descending, 5) || colorFor(ascending, 3) != colorFor(descending, 3) {
		t.Fatal("expected category colors to stay stable regardless of ranking order")
	}
}

func TestBuildPieSlicesSingleCategoryDrawsFullCircle(t *testing.T) {
	slices := buildPieSlices([]CategoryAmount{{CategoryID: 1, Amount: 42}}, nil)
	if len(slices) != 1 || slices[0].Percentage != 100 {
		t.Fatalf("expected single 100%% slice, got %+v", slices)
	}
	if slices[0].PathData != fullCirclePath() {
		t.Errorf("expected full-circle path for a single category, got %q", slices[0].PathData)
	}
}

func TestArcPointMatchesClockPositions(t *testing.T) {
	tests := []struct {
		angle      float64
		wantX      float64
		wantY      float64
		clockLabel string
	}{
		{0, pieChartCenter, pieChartCenter - pieChartRadius, "12 o'clock"},
		{90, pieChartCenter + pieChartRadius, pieChartCenter, "3 o'clock"},
		{180, pieChartCenter, pieChartCenter + pieChartRadius, "6 o'clock"},
		{270, pieChartCenter - pieChartRadius, pieChartCenter, "9 o'clock"},
	}
	for _, test := range tests {
		x, y := arcPoint(test.angle)
		if math.Abs(x-test.wantX) > 0.01 || math.Abs(y-test.wantY) > 0.01 {
			t.Errorf("%s: expected (%.2f, %.2f), got (%.2f, %.2f)", test.clockLabel, test.wantX, test.wantY, x, y)
		}
	}
}
