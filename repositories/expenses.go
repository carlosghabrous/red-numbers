package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/ghab/red-numbers/models"
)

// ExpenseRepository handles expense persistence.
type ExpenseRepository struct {
	db *sql.DB
}

// NewExpenseRepository creates an expense repository backed by db.
func NewExpenseRepository(db *sql.DB) *ExpenseRepository {
	return &ExpenseRepository{db: db}
}

// Create stores one expense and assigns its database ID.
func (r *ExpenseRepository) Create(ctx context.Context, expense *models.Expense) error {
	return r.create(ctx, r.db, expense)
}

// CreateBatch stores all expenses in one transaction.
func (r *ExpenseRepository) CreateBatch(ctx context.Context, expenses []models.Expense) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin expense transaction: %w", err)
	}
	defer tx.Rollback()

	for i := range expenses {
		if err := r.create(ctx, tx, &expenses[i]); err != nil {
			return fmt.Errorf("failed to create expense %d: %w", i, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit expense transaction: %w", err)
	}
	return nil
}

// GetAll returns all expenses ordered from newest to oldest.
func (r *ExpenseRepository) GetAll(ctx context.Context) ([]models.Expense, error) {
	return r.GetByFilters(ctx, "date", "desc")
}

// GetByFilters returns expenses sorted by a supported field and direction.
func (r *ExpenseRepository) GetByFilters(ctx context.Context, sortBy, sortDirection string) ([]models.Expense, error) {
	return r.GetByFilterOptions(ctx, ExpenseFilterOptions{SortBy: sortBy, SortDirection: sortDirection})
}

// ExpenseFilterOptions contains the dashboard sorting and filtering options.
type ExpenseFilterOptions struct {
	SortBy        string
	SortDirection string
	CategoryIDs   []int64
	StartDate     *time.Time
	EndDate       *time.Time
}

// GetByFilterOptions returns expenses matching category/date filters and sorting.
func (r *ExpenseRepository) GetByFilterOptions(ctx context.Context, options ExpenseFilterOptions) ([]models.Expense, error) {
	sortBy := options.SortBy
	sortDirection := options.SortDirection
	sortExpressions := map[string]string{
		"date":        "e.date",
		"amount":      "e.amount",
		"description": "e.description COLLATE NOCASE",
		"category":    "COALESCE(c.display_name, '') COLLATE NOCASE",
	}
	sortExpression, exists := sortExpressions[sortBy]
	if !exists {
		sortBy = "date"
		sortExpression = sortExpressions[sortBy]
	}
	if sortDirection != "asc" && sortDirection != "desc" {
		sortDirection = "desc"
	}
	fromClause := "expenses e"
	if sortBy == "category" {
		fromClause += " LEFT JOIN categories c ON c.id = e.category_id"
	}
	conditions := make([]string, 0, 2)
	args := make([]any, 0, len(options.CategoryIDs)+2)
	if len(options.CategoryIDs) > 0 {
		placeholders := make([]string, len(options.CategoryIDs))
		for index, categoryID := range options.CategoryIDs {
			placeholders[index] = "?"
			args = append(args, categoryID)
		}
		conditions = append(conditions, "e.category_id IN ("+strings.Join(placeholders, ",")+")")
	}
	if options.StartDate != nil {
		conditions = append(conditions, "e.date >= ?")
		args = append(args, options.StartDate.Format("2006-01-02"))
	}
	if options.EndDate != nil {
		conditions = append(conditions, "e.date < ?")
		args = append(args, options.EndDate.Format("2006-01-02"))
	}
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT e.id, e.date, e.description, e.amount, e.balance, COALESCE(e.category_id, 0),
		       e.confidence_level, e.imported_at, e.corrected_at
		FROM %s
		%s
		ORDER BY %s %s, e.date DESC, e.id DESC`, fromClause, whereClause, sortExpression, sortDirection)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query expenses: %w", err)
	}
	defer rows.Close()

	expenses := make([]models.Expense, 0)
	for rows.Next() {
		var expense models.Expense
		if err := rows.Scan(
			&expense.ID,
			&expense.Date,
			&expense.Description,
			&expense.Amount,
			&expense.Balance,
			&expense.CategoryID,
			&expense.ConfidenceLevel,
			&expense.ImportedAt,
			&expense.CorrectedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan expense: %w", err)
		}
		expenses = append(expenses, expense)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate expenses: %w", err)
	}
	return expenses, nil
}

// LogClassification records an automatic classification decision in the audit log.
func (r *ExpenseRepository) LogClassification(ctx context.Context, expense models.Expense, classification ClassificationLog) error {
	const query = `
		INSERT INTO audit_log (action, expense_id, new_value, details)
		VALUES (?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, "classification", expense.ID, classification.CategoryName,
		fmt.Sprintf("pattern=%s;confidence=%s", classification.Pattern, classification.Confidence))
	return err
}

// ClassificationLog contains the audit fields for an automatic classification.
type ClassificationLog struct {
	CategoryName string
	Pattern      string
	Confidence   string
}

// ClassificationLogEntry represents one persisted classification decision.
type ClassificationLogEntry struct {
	Timestamp    time.Time
	ExpenseID    int64
	CategoryName string
	Details      string
}

// GetClassificationLogs returns recent automatic classification decisions.
func (r *ExpenseRepository) GetClassificationLogs(ctx context.Context) ([]ClassificationLogEntry, error) {
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

	logs := make([]ClassificationLogEntry, 0)
	for rows.Next() {
		var entry ClassificationLogEntry
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

type expenseWriter interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func (r *ExpenseRepository) create(ctx context.Context, writer expenseWriter, expense *models.Expense) error {
	const query = `
		INSERT INTO expenses
			(date, description, amount, balance, category_id, confidence_level, imported_at, corrected_at)
		VALUES (?, ?, ?, ?, NULLIF(?, 0), ?, ?, ?)`

	confidenceLevel := expense.ConfidenceLevel
	if confidenceLevel == "" {
		confidenceLevel = "low"
	}

	result, err := writer.ExecContext(ctx, query,
		expense.Date,
		expense.Description,
		expense.Amount,
		expense.Balance,
		expense.CategoryID,
		confidenceLevel,
		expense.ImportedAt,
		expense.CorrectedAt,
	)
	if err != nil {
		return err
	}

	expense.ID, err = result.LastInsertId()
	return err
}
