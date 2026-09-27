package expenses

import (
	"context"
	"fmt"

	"github.com/ghab/red-numbers/domains/categories"
)

// Service coordinates expense persistence with category lookups for the router.
type Service struct {
	expenses   *Repository
	categories *categories.Repository
}

// NewService creates an expense service backed by the expense and category repositories.
func NewService(expenseRepo *Repository, categoryRepo *categories.Repository) *Service {
	return &Service{expenses: expenseRepo, categories: categoryRepo}
}

// DashboardData bundles everything the dashboard view needs to render.
type DashboardData struct {
	Expenses      []Expense
	Categories    []categories.Category
	CategoryNames map[int64]string
	TotalExpenses int
	Statistics    Statistics
}

// GetDashboard loads the expenses and categories needed to render the dashboard.
func (s *Service) GetDashboard(ctx context.Context, options FilterOptions) (DashboardData, error) {
	expenseList, err := s.expenses.GetByFilterOptions(ctx, options)
	if err != nil {
		return DashboardData{}, fmt.Errorf("failed to load expenses from the database: %w", err)
	}
	total, err := s.expenses.CountByFilterOptions(ctx, options)
	if err != nil {
		return DashboardData{}, fmt.Errorf("failed to count expenses: %w", err)
	}
	stats, err := s.expenses.GetStatistics(ctx, options)
	if err != nil {
		return DashboardData{}, fmt.Errorf("failed to calculate expense statistics: %w", err)
	}
	categoryList, err := s.categories.GetAllCategories(ctx)
	if err != nil {
		return DashboardData{}, fmt.Errorf("failed to load categories from the database: %w", err)
	}
	categoryNames := make(map[int64]string, len(categoryList))
	for _, category := range categoryList {
		categoryNames[int64(category.ID)] = category.DisplayName
	}
	return DashboardData{
		Expenses:      expenseList,
		Categories:    categoryList,
		CategoryNames: categoryNames,
		TotalExpenses: total,
		Statistics:    stats,
	}, nil
}

// GetExpenseDetail loads one expense together with the categories available for correction.
func (s *Service) GetExpenseDetail(ctx context.Context, id int64) (*Expense, []categories.Category, error) {
	expense, err := s.expenses.GetByID(ctx, id)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load expense: %w", err)
	}
	if expense == nil {
		return nil, nil, nil
	}
	categoryList, err := s.categories.GetAllCategories(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load categories: %w", err)
	}
	return expense, categoryList, nil
}

// UpdateCategory validates the target category and applies the correction.
func (s *Service) UpdateCategory(ctx context.Context, expenseID, categoryID int64) error {
	category, err := s.categories.GetByID(ctx, int(categoryID))
	if err != nil {
		return fmt.Errorf("failed to validate category: %w", err)
	}
	if category == nil {
		return errInvalidCategory
	}
	if err := s.expenses.UpdateCategory(ctx, expenseID, categoryID); err != nil {
		return fmt.Errorf("failed to update category: %w", err)
	}
	return nil
}

// DeleteAll removes every imported expense.
func (s *Service) DeleteAll(ctx context.Context) error {
	return s.expenses.DeleteAll(ctx)
}

// errInvalidCategory signals that the requested category does not exist.
var errInvalidCategory = fmt.Errorf("invalid category")

// IsInvalidCategory reports whether err was caused by an unknown category.
func IsInvalidCategory(err error) bool {
	return err == errInvalidCategory
}
