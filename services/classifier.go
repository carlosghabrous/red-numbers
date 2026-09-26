package services

import "strings"

// Classification is the result of classifying an expense description.
type Classification struct {
	CategoryName string
	Pattern      string
	Confidence   string
}

type classificationRule struct {
	category string
	priority int
	keywords []string
}

var classificationRules = []classificationRule{
	{category: "supermercado", priority: 1, keywords: []string{"mercadona", "consum", "carrefour", "aldi", "hiperchina", "dia retail"}},
	{category: "medico", priority: 2, keywords: []string{"farmacia", "clinica", "dental", "medico"}},
	{category: "niños", priority: 3, keywords: []string{"colegio", "educarte", "crececonmusica"}},
	{category: "ocio", priority: 4, keywords: []string{"cinema", "cine", "ocine", "enjoy", "amazon prime", "netflix", "downdog", "pilates"}},
	{category: "deporte", priority: 5, keywords: []string{"club natacion", "delfin", "mirador", "altitud", "bergueda", "extreme"}},
	{category: "suministros", priority: 6, keywords: []string{"iberdrola", "pepe mobile", "telefonica", "agua", "gas"}},
	{category: "casa", priority: 7, keywords: []string{"alquiler", "renta", "casa", "volkswagen renting"}},
}

// FuzzyClassifierService assigns categories using the built-in keyword rules.
type FuzzyClassifierService struct {
	rules []classificationRule
}

// NewFuzzyClassifierService creates a classifier with the default category rules.
func NewFuzzyClassifierService() *FuzzyClassifierService {
	return &FuzzyClassifierService{rules: classificationRules}
}

// Classify returns the best matching category and confidence level.
func (c *FuzzyClassifierService) Classify(description string) Classification {
	normalized := normalizeDescription(description)
	best := Classification{CategoryName: "supermercado", Confidence: "low"}
	bestScore := 0.0
	bestPriority := 0

	for _, rule := range c.rules {
		for _, keyword := range rule.keywords {
			score, confidence := matchScore(normalized, normalizeDescription(keyword))
			if score == 0 || score < bestScore || (score == bestScore && bestPriority != 0 && rule.priority > bestPriority) {
				continue
			}
			best = Classification{
				CategoryName: rule.category,
				Pattern:      keyword,
				Confidence:   confidence,
			}
			bestScore = score
			bestPriority = rule.priority
		}
	}

	return best
}

func normalizeDescription(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer(
		"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u",
	).Replace(value)
	return strings.Join(strings.Fields(value), " ")
}

func matchScore(description, keyword string) (float64, string) {
	if description == keyword {
		return 1.0, "high"
	}
	if containsWord(description, keyword) {
		return 0.9, "high"
	}
	if strings.Contains(description, keyword) {
		return 0.7, "medium"
	}
	return 0, "low"
}


func containsWord(description, keyword string) bool {
	words := strings.FieldsFunc(description, func(r rune) bool {
		return r < 'a' || r > 'z'
	})
	keywordWords := strings.FieldsFunc(keyword, func(r rune) bool {
		return r < 'a' || r > 'z'
	})
	if len(keywordWords) == 0 || len(keywordWords) > len(words) {
		return false
	}
	for start := 0; start <= len(words)-len(keywordWords); start++ {
		matched := true
		for offset := range keywordWords {
			if words[start+offset] != keywordWords[offset] {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}
