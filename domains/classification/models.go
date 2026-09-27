package classification

import "time"

// Classification is the result of classifying an expense description.
type Classification struct {
	CategoryName string
	Pattern      string
	Confidence   string
}

// LogEntry represents one persisted classification decision.
type LogEntry struct {
	Timestamp    time.Time
	ExpenseID    int64
	CategoryName string
	Details      string
}
