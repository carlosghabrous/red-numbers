package classification

import (
	"context"
	"database/sql"
	"fmt"
)

// Repository persists automatic classification decisions to the audit log.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a classification log repository backed by db.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Log records an automatic classification decision in the audit log.
func (r *Repository) Log(ctx context.Context, expenseID int64, classification Classification) error {
	const query = `
		INSERT INTO audit_log (action, expense_id, new_value, details)
		VALUES (?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, "classification", expenseID, classification.CategoryName,
		fmt.Sprintf("pattern=%s;confidence=%s", classification.Pattern, classification.Confidence))
	return err
}

// GetLogs returns recent automatic classification decisions.
func (r *Repository) GetLogs(ctx context.Context) ([]LogEntry, error) {
	const query = `
		SELECT timestamp, expense_id, new_value, details
		FROM audit_log
		WHERE action = ?
		ORDER BY timestamp DESC, id DESC`
	rows, err := r.db.QueryContext(ctx, query, "classification")
	if err != nil {
		return nil, fmt.Errorf("failed to query classification logs: %w", err)
	}
	defer rows.Close()

	logs := make([]LogEntry, 0)
	for rows.Next() {
		var entry LogEntry
		if err := rows.Scan(&entry.Timestamp, &entry.ExpenseID, &entry.CategoryName, &entry.Details); err != nil {
			return nil, fmt.Errorf("failed to scan classification log: %w", err)
		}
		logs = append(logs, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate classification logs: %w", err)
	}
	return logs, nil
}
