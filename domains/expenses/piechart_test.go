package expenses

import (
	"math"
	"testing"

	"github.com/ghab/red-numbers/domains/categories"
)

func TestBuildPieSlicesCalculatesPercentages(t *testing.T) {
	breakdown := []CategoryAmount{
		{CategoryID: 1, Amount: 75},
		{CategoryID: 2, Amount: 25},
	}
	categoryNames := map[int64]string{1: "Casa", 2: "Ocio"}

	slices := buildPieSlices(breakdown, categoryNames, nil)
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
	slices := buildPieSlices(breakdown, map[int64]string{}, nil)
	if len(slices) != 1 || slices[0].CategoryName != "Sin clasificar" {
		t.Fatalf("expected fallback category name, got %+v", slices)
	}
	if slices[0].Color != unclassifiedColor {
		t.Errorf("expected the neutral unclassified color, got %q", slices[0].Color)
	}
}

func TestBuildPieSlicesReturnsNilForNoSpending(t *testing.T) {
	if slices := buildPieSlices(nil, nil, nil); slices != nil {
		t.Fatalf("expected nil slices for empty breakdown, got %+v", slices)
	}
	if slices := buildPieSlices([]CategoryAmount{{CategoryID: 1, Amount: 0}}, nil, nil); slices != nil {
		t.Fatalf("expected nil slices when total spending is zero, got %+v", slices)
	}
}

func TestBuildPieSlicesAssignsStableColorsIndependentOfRank(t *testing.T) {
	categoryColors := map[int64]string{5: "#111111", 3: "#222222"}
	ascending := buildPieSlices([]CategoryAmount{
		{CategoryID: 5, Amount: 10},
		{CategoryID: 3, Amount: 90},
	}, nil, categoryColors)
	descending := buildPieSlices([]CategoryAmount{
		{CategoryID: 3, Amount: 90},
		{CategoryID: 5, Amount: 10},
	}, nil, categoryColors)

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
	if colorFor(ascending, 5) != "#111111" || colorFor(ascending, 3) != "#222222" {
		t.Fatal("expected each category's color to come from the provided color index")
	}
}

func TestBuildPieSlicesSingleCategoryDrawsFullCircle(t *testing.T) {
	slices := buildPieSlices([]CategoryAmount{{CategoryID: 1, Amount: 42}}, nil, nil)
	if len(slices) != 1 || slices[0].Percentage != 100 {
		t.Fatalf("expected single 100%% slice, got %+v", slices)
	}
	if slices[0].PathData != fullCirclePath() {
		t.Errorf("expected full-circle path for a single category, got %q", slices[0].PathData)
	}
}

// TestBuildCategoryColorIndexRanksBySparseID reproduces the reported bug:
// category IDs can be large and sparse (e.g. 736, 737, 748 after repeated
// test/experimental inserts), and cycling the palette by raw ID wastes slots
// (none of the first 7 default IDs ever land on slot 0) and produces
// collisions that don't need to happen yet, since compacting by rank still
// fits comfortably inside the palette. Ranking first fixes both.
func TestBuildCategoryColorIndexRanksBySparseID(t *testing.T) {
	categoryList := []categories.Category{
		{ID: 1, Name: "supermercado"},
		{ID: 2, Name: "medico"},
		{ID: 736, Name: "viajes"},
		{ID: 737, Name: "autonomos"},
	}
	colors := BuildCategoryColorIndex(categoryList)

	if colors[1] != pieChartPalette[0] {
		t.Errorf("expected the first category (by ID) to land on slot 0 (blue), got %q", colors[1])
	}
	if colors[2] != pieChartPalette[1] {
		t.Errorf("expected the second category to land on slot 1, got %q", colors[2])
	}
	if colors[736] != pieChartPalette[2] {
		t.Errorf("expected a sparse-but-third-by-rank ID to land on slot 2 (not wrap around), got %q", colors[736])
	}
	if colors[737] != pieChartPalette[3] {
		t.Errorf("expected the fourth category to land on slot 3, got %q", colors[737])
	}
	// The actual reported symptom: with the old raw-ID-modulo scheme and an
	// 8-slot palette, supermercado (id=1) and autonomos (id=737, 737%8==1)
	// landed on the exact same slot. Ranked, they must now differ.
	if colors[1] == colors[737] {
		t.Fatal("expected supermercado and autonomos to no longer collide")
	}
}

func TestBuildCategoryColorIndexIsStableAsCategoriesAreAdded(t *testing.T) {
	before := BuildCategoryColorIndex([]categories.Category{
		{ID: 1, Name: "a"},
		{ID: 2, Name: "b"},
	})
	after := BuildCategoryColorIndex([]categories.Category{
		{ID: 1, Name: "a"},
		{ID: 2, Name: "b"},
		{ID: 3, Name: "c"}, // newly added, higher ID
	})
	if before[1] != after[1] || before[2] != after[2] {
		t.Fatal("expected existing categories to keep their color when a new one is added")
	}
}

func TestBuildCategoryColorIndexWrapsBeyondPaletteSize(t *testing.T) {
	var categoryList []categories.Category
	for i := 1; i <= len(pieChartPalette)+1; i++ {
		categoryList = append(categoryList, categories.Category{ID: i})
	}
	colors := BuildCategoryColorIndex(categoryList)
	firstID := int64(1)
	wrappedID := int64(len(pieChartPalette) + 1)
	if colors[firstID] != colors[wrappedID] {
		t.Fatalf("expected the (palette size + 1)th category to wrap back to slot 0, matching the first")
	}
}

func TestColorForCategoryFallsBackForUnknownID(t *testing.T) {
	if got := colorForCategory(999, map[int64]string{1: "#111111"}); got != unclassifiedColor {
		t.Errorf("expected the neutral unclassified color for an unrecognized ID, got %q", got)
	}
	if got := colorForCategory(1, map[int64]string{1: "#111111"}); got != "#111111" {
		t.Errorf("expected the indexed color for a known ID, got %q", got)
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
