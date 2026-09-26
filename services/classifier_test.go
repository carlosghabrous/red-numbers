package services

import "testing"

func TestFuzzyClassifierMatchesCategories(t *testing.T) {
	classifier := NewFuzzyClassifierService()
	tests := []struct {
		description string
		category    string
		confidence  string
	}{
		{"MERCADONA AVDA BALEARES", "supermercado", "high"},
		{"Farmacia Central", "medico", "high"},
		{"R/ EDUCARTE PROYECTOS", "niños", "high"},
		{"Netflix mensual", "ocio", "high"},
		{"Club Natacion Delfin", "deporte", "high"},
		{"Telefónica Internet", "suministros", "high"},
		{"Volkswagen Renting", "casa", "high"},
		{"Unknown merchant", "supermercado", "low"},
	}

	for _, test := range tests {
		result := classifier.Classify(test.description)
		if result.CategoryName != test.category || result.Confidence != test.confidence {
			t.Errorf("Classify(%q) = %+v, want category %q confidence %q", test.description, result, test.category, test.confidence)
		}
	}
}

func TestFuzzyClassifierUsesPriorityForOverlappingMatches(t *testing.T) {
	classifier := NewFuzzyClassifierService()
	result := classifier.Classify("CINE CASA")
	if result.CategoryName != "ocio" {
		t.Fatalf("expected ocio to win by priority, got %+v", result)
	}
}

func TestFuzzyClassifierMatchesPartialKeywords(t *testing.T) {
	classifier := NewFuzzyClassifierService()
	result := classifier.Classify("supermercadona outlet")
	if result.CategoryName != "supermercado" || result.Confidence != "medium" {
		t.Fatalf("expected medium-confidence partial match, got %+v", result)
	}
}
