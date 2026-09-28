package categories

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestServiceCreateCategory(t *testing.T) {
	db := setupCategoriesTestDB(t)
	defer db.Close()
	service := NewService(NewRepository(db))
	ctx := context.Background()

	category, err := service.CreateCategory(ctx, "  Mascotas y Vet  ")
	if err != nil {
		t.Fatalf("CreateCategory failed: %v", err)
	}
	if category.Name != "mascotas_y_vet" || category.DisplayName != "Mascotas y Vet" {
		t.Fatalf("unexpected category: %+v", category)
	}

	stored, err := NewRepository(db).GetByName(ctx, "mascotas_y_vet")
	if err != nil || stored == nil {
		t.Fatalf("expected category to be persisted, got %+v err=%v", stored, err)
	}
}

func TestServiceCreateCategoryRejectsEmptyName(t *testing.T) {
	db := setupCategoriesTestDB(t)
	defer db.Close()
	service := NewService(NewRepository(db))

	if _, err := service.CreateCategory(context.Background(), "   "); !errors.Is(err, ErrEmptyDisplayName) {
		t.Fatalf("expected ErrEmptyDisplayName, got %v", err)
	}
}

func TestServiceCreateCategoryRejectsDuplicates(t *testing.T) {
	db := setupCategoriesTestDB(t)
	defer db.Close()
	service := NewService(NewRepository(db))
	ctx := context.Background()

	if _, err := service.CreateCategory(ctx, "Casa"); !errors.Is(err, ErrDuplicateCategory) {
		t.Fatalf("expected ErrDuplicateCategory for name colliding with seeded 'casa', got %v", err)
	}
}

func TestServiceCreateCategoryRejectsTooLongName(t *testing.T) {
	db := setupCategoriesTestDB(t)
	defer db.Close()
	service := NewService(NewRepository(db))

	tooLong := strings.Repeat("a", maxDisplayNameLength+1)
	if _, err := service.CreateCategory(context.Background(), tooLong); !errors.Is(err, ErrDisplayNameTooLong) {
		t.Fatalf("expected ErrDisplayNameTooLong, got %v", err)
	}

	exactly := strings.Repeat("a", maxDisplayNameLength)
	if _, err := service.CreateCategory(context.Background(), exactly); err != nil {
		t.Fatalf("expected a name at exactly the limit to be accepted, got %v", err)
	}
}

func TestSlugify(t *testing.T) {
	tests := map[string]string{
		"Mascotas":       "mascotas",
		"Médico Dental":  "medico_dental",
		"  Niños  ":      "ninos",
		"Suministros!!!": "suministros",
	}
	for input, want := range tests {
		if got := slugify(input); got != want {
			t.Errorf("slugify(%q) = %q, want %q", input, got, want)
		}
	}
}
