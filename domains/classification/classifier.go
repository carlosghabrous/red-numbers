package classification

import "strings"

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

// Classifier assigns categories using the built-in keyword rules.
type Classifier struct {
	rules []classificationRule
}

// NewClassifier creates a classifier with the default category rules.
func NewClassifier() *Classifier {
	return &Classifier{rules: classificationRules}
}

// NormalizeDescription exposes the classifier's description normalization
// (lowercasing, accent stripping, whitespace collapsing) so other domains can
// compare descriptions the same way the classifier does.
func NormalizeDescription(value string) string {
	return normalizeDescription(value)
}

// incomeCategory is assigned unconditionally to any expense with a positive
// amount, bypassing keyword matching entirely: a positive amount is money
// coming in, not a categorizable purchase.
const incomeCategory = "income"

// Classify returns the best matching category and confidence level. Expenses
// with a positive amount are always classified as income.
func (c *Classifier) Classify(description string, amount float64) Classification {
	if amount > 0 {
		return Classification{CategoryName: incomeCategory, Confidence: "high"}
	}
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
