package categories

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
)

// SeedDefaultCategories inserts the default expense categories into the database.
func SeedDefaultCategories(ctx context.Context, db *sql.DB) error {
	logger := slog.Default()

	categories := []struct {
		name        string
		displayName string
	}{
		{"supermercado", "Supermercado"},
		{"medico", "Médico"},
		{"niños", "Niños"},
		{"ocio", "Ocio"},
		{"deporte", "Deporte"},
		{"suministros", "Suministros"},
		{"casa", "Casa"},
		{"income", "Income"},
	}

	for _, cat := range categories {
		query := `INSERT OR IGNORE INTO categories (name, display_name) VALUES (?, ?)`
		if _, err := db.ExecContext(ctx, query, cat.name, cat.displayName); err != nil {
			logger.Error("Failed to seed category",
				slog.String("category", cat.name),
				slog.String("error", err.Error()),
			)
			return fmt.Errorf("failed to seed category %s: %w", cat.name, err)
		}
	}

	logger.Info("Default categories seeded successfully",
		slog.Int("category_count", len(categories)),
	)
	return nil
}
