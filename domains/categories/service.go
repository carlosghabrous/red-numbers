package categories

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrEmptyDisplayName signals that no display name was submitted.
var ErrEmptyDisplayName = errors.New("display name is required")

// ErrDuplicateCategory signals that a category with the same name already exists.
var ErrDuplicateCategory = errors.New("category already exists")

// Service coordinates category creation for the router.
type Service struct {
	categories *Repository
}

// NewService creates a category service backed by the category repository.
func NewService(categoryRepo *Repository) *Service {
	return &Service{categories: categoryRepo}
}

// CreateCategory validates a display name, derives its internal key, and stores it.
func (s *Service) CreateCategory(ctx context.Context, displayName string) (*Category, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return nil, ErrEmptyDisplayName
	}
	name := slugify(displayName)
	if name == "" {
		return nil, ErrEmptyDisplayName
	}
	existing, err := s.categories.GetByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to check for an existing category: %w", err)
	}
	if existing != nil {
		return nil, ErrDuplicateCategory
	}
	category := &Category{Name: name, DisplayName: displayName}
	if err := s.categories.Create(ctx, category); err != nil {
		return nil, fmt.Errorf("failed to create category: %w", err)
	}
	return category, nil
}

// slugify turns a display name into the lowercase, underscore-separated key
// stored as the category's unique name (e.g. "Mascotas y vet" -> "mascotas_y_vet").
func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer(
		"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n",
	).Replace(value)

	var builder strings.Builder
	previousWasSeparator := true // avoid a leading underscore
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			previousWasSeparator = false
			continue
		}
		if !previousWasSeparator {
			builder.WriteRune('_')
			previousWasSeparator = true
		}
	}
	return strings.TrimSuffix(builder.String(), "_")
}
