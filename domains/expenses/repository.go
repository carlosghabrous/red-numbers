package expenses

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Repository handles expense persistence.
type Repository struct {
	db *sql.DB
}

// NewRepository creates an expense repository backed by db.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Create stores one expense and assigns its database ID.
func (r *Repository) Create(ctx context.Context, expense *Expense) error {
	_, err := r.create(ctx, r.db, expense)
	return err
}

// CreateBatch stores all expenses in one transaction.
func (r *Repository) CreateBatch(ctx context.Context, expenses []Expense) error {
	_, err := r.CreateBatchWithCount(ctx, expenses)
	return err
}

// CreateBatchWithCount stores only new expenses and returns the inserted count.
func (r *Repository) CreateBatchWithCount(ctx context.Context, expenses []Expense) (int, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin expense transaction: %w", err)
	}
	defer tx.Rollback()

	inserted := 0
	for i := range expenses {
		wasInserted, err := r.create(ctx, tx, &expenses[i])
		if err != nil {
			return 0, fmt.Errorf("failed to create expense %d: %w", i, err)
		}
		if wasInserted {
			inserted++
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit expense transaction: %w", err)
	}
	return inserted, nil
}

// GetAll returns all expenses ordered from newest to oldest.
func (r *Repository) GetAll(ctx context.Context) ([]Expense, error) {
	return r.GetByFilters(ctx, "date", "desc")
}

// BackfillFingerprints assigns fingerprints to records created before deduplication.
func (r *Repository) BackfillFingerprints(ctx context.Context) error {
	rows, err := r.db.QueryContext(ctx, `SELECT id, date, description, amount, balance FROM expenses WHERE fingerprint IS NULL OR fingerprint = ''`)
	if err != nil {
		return fmt.Errorf("failed to find records without fingerprints: %w", err)
	}
	defer rows.Close()
	type record struct {
		id      int64
		expense Expense
	}
	var records []record
	for rows.Next() {
		var item record
		if err := rows.Scan(&item.id, &item.expense.Date, &item.expense.Description, &item.expense.Amount, &item.expense.Balance); err != nil {
			return err
		}
		records = append(records, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin fingerprint backfill: %w", err)
	}
	defer tx.Rollback()
	for _, item := range records {
		if _, err := tx.ExecContext(ctx, `UPDATE expenses SET fingerprint = ? WHERE id = ?`, fingerprintForExpense(item.expense), item.id); err != nil {
			return fmt.Errorf("failed to backfill expense fingerprint: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM audit_log
		WHERE expense_id IS NOT NULL
		  AND expense_id NOT IN (SELECT MIN(id) FROM expenses GROUP BY fingerprint)`); err != nil {
		return fmt.Errorf("failed to remove duplicate audit entries: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM expenses
		WHERE id NOT IN (SELECT MIN(id) FROM expenses GROUP BY fingerprint)`); err != nil {
		return fmt.Errorf("failed to remove duplicate expenses: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit fingerprint backfill: %w", err)
	}
	return nil
}

// DeleteAll removes all imported expenses and their audit entries.
func (r *Repository) DeleteAll(ctx context.Context) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM audit_log`); err != nil {
		return fmt.Errorf("failed to delete audit log: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM expenses`); err != nil {
		return fmt.Errorf("failed to delete expenses: %w", err)
	}
	return tx.Commit()
}

// GetByID returns one expense by database ID.
func (r *Repository) GetByID(ctx context.Context, id int64) (*Expense, error) {
	const query = `
		SELECT id, date, description, amount, balance, COALESCE(category_id, 0), fingerprint,
		       confidence_level, imported_at, corrected_at
		FROM expenses WHERE id = ?`
	var expense Expense
	var correctedAt sql.NullTime
	var fingerprint sql.NullString
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&expense.ID, &expense.Date, &expense.Description, &expense.Amount, &expense.Balance,
		&expense.CategoryID, &fingerprint, &expense.ConfidenceLevel, &expense.ImportedAt, &correctedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get expense: %w", err)
	}
	if correctedAt.Valid {
		expense.CorrectedAt = &correctedAt.Time
	}
	if fingerprint.Valid {
		expense.Fingerprint = fingerprint.String
	}
	return &expense, nil
}

// UpdateCategory changes an expense category and records the correction.
func (r *Repository) UpdateCategory(ctx context.Context, expenseID, categoryID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin category update: %w", err)
	}
	defer tx.Rollback()

	var oldCategory sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT category_id FROM expenses WHERE id = ?`, expenseID).Scan(&oldCategory); err != nil {
		return fmt.Errorf("failed to load current category: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE expenses SET category_id = ?, confidence_level = 'high', corrected_at = CURRENT_TIMESTAMP WHERE id = ?`, categoryID, expenseID); err != nil {
		return fmt.Errorf("failed to update expense category: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO audit_log (action, expense_id, old_value, new_value, details) VALUES (?, ?, ?, ?, ?)`,
		"category_correction", expenseID, nullableIntString(oldCategory), fmt.Sprint(categoryID), "manual category correction"); err != nil {
		return fmt.Errorf("failed to log category correction: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit category update: %w", err)
	}
	return nil
}

func nullableIntString(value sql.NullInt64) string {
	if !value.Valid {
		return ""
	}
	return fmt.Sprint(value.Int64)
}

// GetByFilters returns expenses sorted by a supported field and direction.
func (r *Repository) GetByFilters(ctx context.Context, sortBy, sortDirection string) ([]Expense, error) {
	return r.GetByFilterOptions(ctx, FilterOptions{SortBy: sortBy, SortDirection: sortDirection})
}

// FilterOptions contains the dashboard sorting and filtering options.
type FilterOptions struct {
	SortBy        string
	SortDirection string
	CategoryIDs   []int64
	StartDate     *time.Time
	EndDate       *time.Time
	Limit         int
	Offset        int
}

// GetByFilterOptions returns expenses matching category/date filters and sorting.
func (r *Repository) GetByFilterOptions(ctx context.Context, options FilterOptions) ([]Expense, error) {
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
		conditions = append(conditions, "date(e.date) >= date(?)")
		args = append(args, options.StartDate.Format("2006-01-02"))
	}
	if options.EndDate != nil {
		conditions = append(conditions, "date(e.date) < date(?)")
		args = append(args, options.EndDate.Format("2006-01-02"))
	}
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	limitClause := ""
	argsForQuery := append([]any(nil), args...)
	if options.Limit > 0 {
		limitClause = " LIMIT ? OFFSET ?"
		argsForQuery = append(argsForQuery, options.Limit, max(options.Offset, 0))
	}

	query := fmt.Sprintf(`
		SELECT e.id, e.date, e.description, e.amount, e.balance, COALESCE(e.category_id, 0), e.fingerprint,
		       e.confidence_level, e.imported_at, e.corrected_at
		FROM %s
		%s
		ORDER BY %s %s, e.date DESC, e.id DESC%s`, fromClause, whereClause, sortExpression, sortDirection, limitClause)

	rows, err := r.db.QueryContext(ctx, query, argsForQuery...)
	if err != nil {
		return nil, fmt.Errorf("failed to query expenses: %w", err)
	}
	defer rows.Close()

	expenses := make([]Expense, 0)
	for rows.Next() {
		var expense Expense
		if err := rows.Scan(
			&expense.ID,
			&expense.Date,
			&expense.Description,
			&expense.Amount,
			&expense.Balance,
			&expense.CategoryID,
			&expense.Fingerprint,
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

// CountByFilterOptions returns the number of expenses matching the filters.
func (r *Repository) CountByFilterOptions(ctx context.Context, options FilterOptions) (int, error) {
	fromClause := "expenses e"
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
		conditions = append(conditions, "date(e.date) >= date(?)")
		args = append(args, options.StartDate.Format("2006-01-02"))
	}
	if options.EndDate != nil {
		conditions = append(conditions, "date(e.date) < date(?)")
		args = append(args, options.EndDate.Format("2006-01-02"))
	}
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}
	var count int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+fromClause+whereClause, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count expenses: %w", err)
	}
	return count, nil
}

func max(first, second int) int {
	if first > second {
		return first
	}
	return second
}

type expenseWriter interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (r *Repository) create(ctx context.Context, writer expenseWriter, expense *Expense) (bool, error) {
	if expense.Fingerprint == "" {
		expense.Fingerprint = fingerprintForExpense(*expense)
	}
	var existingID int64
	if err := writer.QueryRowContext(ctx, `SELECT id FROM expenses WHERE fingerprint = ? LIMIT 1`, expense.Fingerprint).Scan(&existingID); err == nil {
		expense.ID = existingID
		return false, nil
	} else if err != sql.ErrNoRows {
		return false, err
	}
	const query = `
		INSERT INTO expenses
			(date, description, amount, balance, category_id, confidence_level, imported_at, corrected_at, fingerprint)
		VALUES (?, ?, ?, ?, NULLIF(?, 0), ?, ?, ?, ?)`

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
		expense.Fingerprint,
	)
	if err != nil {
		return false, err
	}

	expense.ID, err = result.LastInsertId()
	return true, err
}

func fingerprintForExpense(expense Expense) string {
	value := fmt.Sprintf("%s\x00%s\x00%.17g\x00%.17g", expense.Date.Format("2006-01-02"), expense.Description, expense.Amount, expense.Balance)
	hash := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", hash[:])
}
