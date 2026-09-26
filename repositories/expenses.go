package repositories

import (
	"context"
	"database/sql"
	"fmt"

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
	const query = `
		SELECT id, date, description, amount, balance, COALESCE(category_id, 0),
		       confidence_level, imported_at, corrected_at
		FROM expenses
		ORDER BY date DESC, id DESC`

	rows, err := r.db.QueryContext(ctx, query)
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
