package models

import "time"

// Expense represents a single transaction record
type Expense struct {
	ID              int64      `json:"id"`
	Date            time.Time  `json:"date"`
	Description     string     `json:"description"`
	Amount          float64    `json:"amount"`
	Balance         float64    `json:"balance"`
	Fingerprint     string     `json:"-"`
	CategoryID      int64      `json:"category_id"`
	ConfidenceLevel string     `json:"confidence_level"`
	ImportedAt      time.Time  `json:"imported_at"`
	CorrectedAt     *time.Time `json:"corrected_at"`
}
