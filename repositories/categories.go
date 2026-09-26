package repositories

import (
	"context"
	"database/sql"

	"github.com/ghab/red-numbers/models"
)

// CategoryRepository handles category data access
type CategoryRepository struct {
	db *sql.DB
}

// NewCategoryRepository creates a new category repository
func NewCategoryRepository(db *sql.DB) *CategoryRepository {
	return &CategoryRepository{db: db}
}

// GetAllCategories returns all categories from the database
func (r *CategoryRepository) GetAllCategories(ctx context.Context) ([]models.Category, error) {
	query := `
		SELECT id, name, display_name, created_at
		FROM categories
		ORDER BY id ASC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []models.Category
	for rows.Next() {
		var category models.Category
		if err := rows.Scan(&category.ID, &category.Name, &category.DisplayName, &category.CreatedAt); err != nil {
			return nil, err
		}
		categories = append(categories, category)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return categories, nil
}

// GetByID returns a category by ID
func (r *CategoryRepository) GetByID(ctx context.Context, id int) (*models.Category, error) {
	query := `
		SELECT id, name, display_name, created_at
		FROM categories
		WHERE id = ?
	`

	var category models.Category
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&category.ID,
		&category.Name,
		&category.DisplayName,
		&category.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &category, nil
}

// GetByName returns a category by name
func (r *CategoryRepository) GetByName(ctx context.Context, name string) (*models.Category, error) {
	query := `
		SELECT id, name, display_name, created_at
		FROM categories
		WHERE name = ?
	`

	var category models.Category
	err := r.db.QueryRowContext(ctx, query, name).Scan(
		&category.ID,
		&category.Name,
		&category.DisplayName,
		&category.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &category, nil
}

// Create inserts a new category into the database
func (r *CategoryRepository) Create(ctx context.Context, category *models.Category) error {
	query := `
		INSERT INTO categories (name, display_name)
		VALUES (?, ?)
	`

	result, err := r.db.ExecContext(ctx, query, category.Name, category.DisplayName)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	category.ID = int(id)
	return nil
}
