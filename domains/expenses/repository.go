package expenses

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/ghab/red-numbers/domains/classification"
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

// BackfillIncomeCategory assigns incomeCategoryID to every stored expense with
// a positive amount that isn't already in that category. It runs on every
// startup (like the other backfills) so the income rule applies retroactively
// to data imported before the rule existed.
func (r *Repository) BackfillIncomeCategory(ctx context.Context, incomeCategoryID int64) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`UPDATE expenses SET category_id = ?, confidence_level = 'high' WHERE amount > 0 AND (category_id IS NULL OR category_id != ?)`,
		incomeCategoryID, incomeCategoryID)
	if err != nil {
		return 0, fmt.Errorf("failed to backfill income category: %w", err)
	}
	return result.RowsAffected()
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

// ReclassifyResult summarizes the effect of re-classifying expenses that share
// a description pattern with a just-corrected expense.
type ReclassifyResult struct {
	MatchedCount int // other expenses sharing the corrected expense's description pattern
	ChangedCount int // of those, how many actually had their category changed
}

// UpdateCategory changes an expense's category, then re-classifies every other
// stored expense that shares the same (normalized) description, so a single
// correction fixes every occurrence of a recurring merchant/description.
func (r *Repository) UpdateCategory(ctx context.Context, expenseID, categoryID int64) (ReclassifyResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ReclassifyResult{}, fmt.Errorf("failed to begin category update: %w", err)
	}
	defer tx.Rollback()

	var description string
	var oldCategory sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT description, category_id FROM expenses WHERE id = ?`, expenseID).Scan(&description, &oldCategory); err != nil {
		return ReclassifyResult{}, fmt.Errorf("failed to load current category: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE expenses SET category_id = ?, confidence_level = 'high', corrected_at = CURRENT_TIMESTAMP WHERE id = ?`, categoryID, expenseID); err != nil {
		return ReclassifyResult{}, fmt.Errorf("failed to update expense category: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO audit_log (action, expense_id, old_value, new_value, details) VALUES (?, ?, ?, ?, ?)`,
		"category_correction", expenseID, nullableIntString(oldCategory), fmt.Sprint(categoryID), "manual category correction"); err != nil {
		return ReclassifyResult{}, fmt.Errorf("failed to log category correction: %w", err)
	}

	result, err := reclassifySimilarExpenses(ctx, tx, expenseID, description, categoryID)
	if err != nil {
		return ReclassifyResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return ReclassifyResult{}, fmt.Errorf("failed to commit category update: %w", err)
	}
	return result, nil
}

// reclassifySimilarExpenses finds every other expense whose description
// normalizes the same way as excludeID's, and updates any that don't already
// have categoryID. Matching happens in Go (not SQL) because normalization
// strips accents and collapses whitespace.
func reclassifySimilarExpenses(ctx context.Context, tx *sql.Tx, excludeID int64, description string, categoryID int64) (ReclassifyResult, error) {
	normalizedTarget := classification.NormalizeDescription(description)

	rows, err := tx.QueryContext(ctx, `SELECT id, description, amount, category_id FROM expenses WHERE id != ?`, excludeID)
	if err != nil {
		return ReclassifyResult{}, fmt.Errorf("failed to find similar expenses: %w", err)
	}
	type candidate struct {
		id         int64
		categoryID sql.NullInt64
	}
	var matches []candidate
	for rows.Next() {
		var item candidate
		var otherDescription string
		var amount float64
		if err := rows.Scan(&item.id, &otherDescription, &amount, &item.categoryID); err != nil {
			rows.Close()
			return ReclassifyResult{}, fmt.Errorf("failed to scan candidate expense: %w", err)
		}
		// Positive-amount expenses are always income, regardless of description
		// pattern, so a description-based correction must never move them out.
		if amount > 0 {
			continue
		}
		if classification.NormalizeDescription(otherDescription) == normalizedTarget {
			matches = append(matches, item)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return ReclassifyResult{}, fmt.Errorf("failed to iterate candidate expenses: %w", err)
	}
	rows.Close()

	result := ReclassifyResult{MatchedCount: len(matches)}
	for _, match := range matches {
		if match.categoryID.Valid && match.categoryID.Int64 == categoryID {
			continue
		}
		if _, err := tx.ExecContext(ctx, `UPDATE expenses SET category_id = ?, confidence_level = 'high', corrected_at = CURRENT_TIMESTAMP WHERE id = ?`, categoryID, match.id); err != nil {
			return ReclassifyResult{}, fmt.Errorf("failed to re-classify similar expense: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO audit_log (action, expense_id, old_value, new_value, details) VALUES (?, ?, ?, ?, ?)`,
			"re_classification", match.id, nullableIntString(match.categoryID), fmt.Sprint(categoryID), "auto re-classified: matches corrected expense description"); err != nil {
			return ReclassifyResult{}, fmt.Errorf("failed to log re-classification: %w", err)
		}
		result.ChangedCount++
	}
	return result, nil
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

// filterConditions builds the shared WHERE conditions/args for category and date filters.
func filterConditions(options FilterOptions) ([]string, []any) {
	conditions := make([]string, 0, 3)
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
	return conditions, args
}

func whereClauseFrom(conditions []string) string {
	if len(conditions) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(conditions, " AND ")
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
	conditions, args := filterConditions(options)
	whereClause := whereClauseFrom(conditions)

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
		var fingerprint sql.NullString
		if err := rows.Scan(
			&expense.ID,
			&expense.Date,
			&expense.Description,
			&expense.Amount,
			&expense.Balance,
			&expense.CategoryID,
			&fingerprint,
			&expense.ConfidenceLevel,
			&expense.ImportedAt,
			&expense.CorrectedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan expense: %w", err)
		}
		if fingerprint.Valid {
			expense.Fingerprint = fingerprint.String
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
	conditions, args := filterConditions(options)
	whereClause := whereClauseFrom(conditions)
	var count int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM expenses e"+whereClause, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count expenses: %w", err)
	}
	return count, nil
}

// Statistics summarizes spending totals for a filtered set of expenses.
// "Spending" only counts outgoing (negative-amount) transactions, so incoming
// transfers/refunds don't inflate totals or dilute the average.
type Statistics struct {
	TransactionCount  int
	SpendingCount     int
	TotalSpending     float64
	TotalIncome       float64
	AverageSpending   float64
	TopCategoryID     int64
	TopCategoryAmount float64
}

// GetStatistics computes spending/income totals and the top category for the given filters.
func (r *Repository) GetStatistics(ctx context.Context, options FilterOptions) (Statistics, error) {
	conditions, args := filterConditions(options)
	whereClause := whereClauseFrom(conditions)

	var stats Statistics
	overallQuery := `
		SELECT
			COUNT(*),
			COUNT(CASE WHEN e.amount < 0 THEN 1 END),
			COALESCE(SUM(CASE WHEN e.amount < 0 THEN -e.amount ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN e.amount > 0 THEN e.amount ELSE 0 END), 0)
		FROM expenses e` + whereClause
	if err := r.db.QueryRowContext(ctx, overallQuery, args...).Scan(
		&stats.TransactionCount, &stats.SpendingCount, &stats.TotalSpending, &stats.TotalIncome,
	); err != nil {
		return Statistics{}, fmt.Errorf("failed to calculate expense statistics: %w", err)
	}
	if stats.SpendingCount > 0 {
		stats.AverageSpending = stats.TotalSpending / float64(stats.SpendingCount)
	}

	topConditions := append(append([]string{}, conditions...), "e.amount < 0")
	topQuery := `
		SELECT COALESCE(e.category_id, 0), SUM(-e.amount) AS spent
		FROM expenses e` + whereClauseFrom(topConditions) + `
		GROUP BY COALESCE(e.category_id, 0)
		ORDER BY spent DESC
		LIMIT 1`
	err := r.db.QueryRowContext(ctx, topQuery, args...).Scan(&stats.TopCategoryID, &stats.TopCategoryAmount)
	if err != nil && err != sql.ErrNoRows {
		return Statistics{}, fmt.Errorf("failed to calculate top category: %w", err)
	}
	return stats, nil
}

// CategoryAmount is one category's total spending within a filtered set.
type CategoryAmount struct {
	CategoryID int64
	Amount     float64
}

// GetCategoryBreakdown returns spending per category (highest first), for the pie
// chart. Like Statistics, only outgoing (negative-amount) transactions count.
func (r *Repository) GetCategoryBreakdown(ctx context.Context, options FilterOptions) ([]CategoryAmount, error) {
	conditions, args := filterConditions(options)
	conditions = append(conditions, "e.amount < 0")
	query := `
		SELECT COALESCE(e.category_id, 0), SUM(-e.amount) AS spent
		FROM expenses e` + whereClauseFrom(conditions) + `
		GROUP BY COALESCE(e.category_id, 0)
		ORDER BY spent DESC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate category breakdown: %w", err)
	}
	defer rows.Close()

	breakdown := make([]CategoryAmount, 0)
	for rows.Next() {
		var item CategoryAmount
		if err := rows.Scan(&item.CategoryID, &item.Amount); err != nil {
			return nil, fmt.Errorf("failed to scan category breakdown: %w", err)
		}
		breakdown = append(breakdown, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate category breakdown: %w", err)
	}
	return breakdown, nil
}

// SpendingEntry is one outgoing transaction's date, amount, and category,
// used as the raw input for the weekly/monthly histograms. Week/month
// bucketing happens in Go (see histogram.go) rather than in SQL, since
// SQLite's week-of-year functions don't cleanly express a Monday-start week
// across year boundaries.
type SpendingEntry struct {
	Date       time.Time
	Amount     float64
	CategoryID int64
}

// GetSpendingEntries returns every outgoing (negative-amount) transaction
// matching the filters, for histogram aggregation. Like Statistics and
// GetCategoryBreakdown, incoming transactions never count as "spending".
func (r *Repository) GetSpendingEntries(ctx context.Context, options FilterOptions) ([]SpendingEntry, error) {
	conditions, args := filterConditions(options)
	conditions = append(conditions, "e.amount < 0")
	query := `
		SELECT e.date, -e.amount, COALESCE(e.category_id, 0)
		FROM expenses e` + whereClauseFrom(conditions) + `
		ORDER BY e.date ASC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query spending entries: %w", err)
	}
	defer rows.Close()

	entries := make([]SpendingEntry, 0)
	for rows.Next() {
		var entry SpendingEntry
		if err := rows.Scan(&entry.Date, &entry.Amount, &entry.CategoryID); err != nil {
			return nil, fmt.Errorf("failed to scan spending entry: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate spending entries: %w", err)
	}
	return entries, nil
}

// GetLearnedCategories returns a normalized-description -> category-name
// mapping built from every expense a user has manually corrected, so a new
// CSV import can apply past corrections to matching descriptions instead of
// relying on the keyword classifier alone. When more than one corrected
// expense shares a description, the most recent correction wins.
func (r *Repository) GetLearnedCategories(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT e.description, c.name
		FROM expenses e
		JOIN categories c ON c.id = e.category_id
		WHERE e.corrected_at IS NOT NULL
		ORDER BY e.corrected_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("failed to query learned categories: %w", err)
	}
	defer rows.Close()

	learned := make(map[string]string)
	for rows.Next() {
		var description, categoryName string
		if err := rows.Scan(&description, &categoryName); err != nil {
			return nil, fmt.Errorf("failed to scan learned category: %w", err)
		}
		// Later rows (more recently corrected) overwrite earlier ones.
		learned[classification.NormalizeDescription(description)] = categoryName
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate learned categories: %w", err)
	}
	return learned, nil
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
