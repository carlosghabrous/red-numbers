package categories

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// setupCategoriesTestDB creates a temporary in-memory database for testing categories
func setupCategoriesTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Create the categories table
	schemaSQL := `
CREATE TABLE categories (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Seed default categories
INSERT OR IGNORE INTO categories (id, name, display_name) VALUES
(1, 'supermercado', 'Supermercado'),
(2, 'medico', 'Médico'),
(3, 'niños', 'Niños'),
(4, 'ocio', 'Ocio'),
(5, 'deporte', 'Deporte'),
(6, 'suministros', 'Suministros'),
(7, 'casa', 'Casa');
	`

	if _, err := db.Exec(schemaSQL); err != nil {
		t.Fatalf("Failed to execute schema: %v", err)
	}

	return db
}

// TestGetAllCategories verifies that all 7 default categories are seeded and returned
func TestGetAllCategories(t *testing.T) {
	db := setupCategoriesTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	// Get all categories
	categories, err := repo.GetAllCategories(ctx)
	if err != nil {
		t.Fatalf("GetAllCategories failed: %v", err)
	}

	// Verify we have exactly 7 categories
	if len(categories) != 7 {
		t.Errorf("Expected 7 categories, got %d", len(categories))
	}

	// Verify all expected categories are present with correct display_name
	expectedCategories := map[string]string{
		"supermercado": "Supermercado",
		"medico":       "Médico",
		"niños":        "Niños",
		"ocio":         "Ocio",
		"deporte":      "Deporte",
		"suministros":  "Suministros",
		"casa":         "Casa",
	}

	categoryMap := make(map[string]string)
	for _, cat := range categories {
		categoryMap[cat.Name] = cat.DisplayName
	}

	for expectedName, expectedDisplay := range expectedCategories {
		if actualDisplay, exists := categoryMap[expectedName]; !exists {
			t.Errorf("Category %q not found in results", expectedName)
		} else if actualDisplay != expectedDisplay {
			t.Errorf("Category %q has display_name %q, expected %q", expectedName, actualDisplay, expectedDisplay)
		}
	}

	// Verify categories have valid timestamps
	for _, cat := range categories {
		if cat.CreatedAt.IsZero() {
			t.Errorf("Category %q has zero CreatedAt timestamp", cat.Name)
		}
		if cat.CreatedAt.After(time.Now().Add(1 * time.Second)) {
			t.Errorf("Category %q has CreatedAt in the future", cat.Name)
		}
	}
}

// TestGetByID verifies that a category can be retrieved by ID
func TestGetByID(t *testing.T) {
	db := setupCategoriesTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	// Get category by ID
	category, err := repo.GetByID(ctx, 1)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if category == nil {
		t.Fatal("Expected category, got nil")
	}

	if category.ID != 1 || category.Name != "supermercado" || category.DisplayName != "Supermercado" {
		t.Errorf("GetByID returned unexpected category: %+v", category)
	}
}

// TestGetByName verifies that a category can be retrieved by name
func TestGetByName(t *testing.T) {
	db := setupCategoriesTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	// Get category by name
	category, err := repo.GetByName(ctx, "medico")
	if err != nil {
		t.Fatalf("GetByName failed: %v", err)
	}

	if category == nil {
		t.Fatal("Expected category, got nil")
	}

	if category.ID != 2 || category.Name != "medico" || category.DisplayName != "Médico" {
		t.Errorf("GetByName returned unexpected category: %+v", category)
	}
}

// TestCategoriesIdempotent verifies that INSERT OR IGNORE prevents duplicate insertions
func TestCategoriesIdempotent(t *testing.T) {
	db := setupCategoriesTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	// Get categories first time
	categories1, err := repo.GetAllCategories(ctx)
	if err != nil {
		t.Fatalf("First GetAllCategories failed: %v", err)
	}

	count1 := len(categories1)

	// Try to insert the same categories again (simulating a second migration run)
	insertSQL := `
INSERT OR IGNORE INTO categories (id, name, display_name) VALUES
(1, 'supermercado', 'Supermercado'),
(2, 'medico', 'Médico'),
(3, 'niños', 'Niños'),
(4, 'ocio', 'Ocio'),
(5, 'deporte', 'Deporte'),
(6, 'suministros', 'Suministros'),
(7, 'casa', 'Casa');
	`

	if _, err := db.Exec(insertSQL); err != nil {
		t.Fatalf("Failed to execute insert again: %v", err)
	}

	// Get categories again
	categories2, err := repo.GetAllCategories(ctx)
	if err != nil {
		t.Fatalf("Second GetAllCategories failed: %v", err)
	}

	count2 := len(categories2)

	// Verify no new categories were added (idempotent)
	if count1 != count2 {
		t.Errorf("Expected %d categories after second insert, got %d (should be idempotent)", count1, count2)
	}

	// Verify still 7 categories
	if count2 != 7 {
		t.Errorf("Expected 7 categories after idempotent check, got %d", count2)
	}
}

// TestGetNonExistentCategory verifies that GetByID returns nil for non-existent ID
func TestGetNonExistentCategory(t *testing.T) {
	db := setupCategoriesTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	// Try to get non-existent category
	category, err := repo.GetByID(ctx, 999)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if category != nil {
		t.Errorf("Expected nil for non-existent category, got %+v", category)
	}
}

// TestGetAllCategoriesOrder verifies that categories are returned alphabetically
// by display name, regardless of insertion/ID order, so dropdowns are sorted.
func TestGetAllCategoriesOrder(t *testing.T) {
	db := setupCategoriesTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	categories, err := repo.GetAllCategories(ctx)
	if err != nil {
		t.Fatalf("GetAllCategories failed: %v", err)
	}

	wantOrder := []string{"Casa", "Deporte", "Médico", "Niños", "Ocio", "Suministros", "Supermercado"}
	if len(categories) != len(wantOrder) {
		t.Fatalf("expected %d categories, got %d", len(wantOrder), len(categories))
	}
	for i, want := range wantOrder {
		if categories[i].DisplayName != want {
			t.Errorf("expected %q at position %d, got %q", want, i, categories[i].DisplayName)
		}
	}
}

// TestCategoryFieldValues verifies that all required fields have correct values
func TestCategoryFieldValues(t *testing.T) {
	db := setupCategoriesTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	categories, err := repo.GetAllCategories(ctx)
	if err != nil {
		t.Fatalf("GetAllCategories failed: %v", err)
	}

	expectedData := []struct {
		id          int
		name        string
		displayName string
	}{
		{7, "casa", "Casa"},
		{5, "deporte", "Deporte"},
		{2, "medico", "Médico"},
		{3, "niños", "Niños"},
		{4, "ocio", "Ocio"},
		{6, "suministros", "Suministros"},
		{1, "supermercado", "Supermercado"},
	}

	for i, expected := range expectedData {
		if i >= len(categories) {
			t.Errorf("Expected category at index %d not found", i)
			continue
		}

		cat := categories[i]
		if cat.ID != expected.id {
			t.Errorf("Category %d: expected ID %d, got %d", i, expected.id, cat.ID)
		}
		if cat.Name != expected.name {
			t.Errorf("Category %d: expected name %q, got %q", i, expected.name, cat.Name)
		}
		if cat.DisplayName != expected.displayName {
			t.Errorf("Category %d: expected display_name %q, got %q", i, expected.displayName, cat.DisplayName)
		}
	}
}
