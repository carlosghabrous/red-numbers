package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// MigrationManager handles database schema versioning and migration execution
type MigrationManager struct {
	db             *sql.DB
	logger         *slog.Logger
	migrationsPath string
}

// Migration represents a single database schema migration
type Migration struct {
	Name    string
	Content string
}

// NewMigrationManager creates a new migration manager
func NewMigrationManager(db *sql.DB, logger *slog.Logger) *MigrationManager {
	return &MigrationManager{
		db:             db,
		logger:         logger,
		migrationsPath: "migrations", // relative path from where app runs
	}
}

// Initialize sets up the migration tracking table if it doesn't exist
func (m *MigrationManager) Initialize(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS _migrations (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	`

	if _, err := m.db.ExecContext(ctx, query); err != nil {
		m.logger.Error("failed to create _migrations table", "error", err)
		return fmt.Errorf("failed to initialize migrations table: %w", err)
	}

	m.logger.Info("migrations table initialized")
	return nil
}

// ApplyMigrations executes all pending migrations in order
func (m *MigrationManager) ApplyMigrations(ctx context.Context) error {
	// Initialize migration tracking table
	if err := m.Initialize(ctx); err != nil {
		return err
	}

	// Load all available migrations from file system
	migrations, err := m.loadMigrations()
	if err != nil {
		m.logger.Error("failed to load migrations", "error", err)
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	// Sort migrations by name to ensure consistent order
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Name < migrations[j].Name
	})

	m.logger.Info("found migrations", "count", len(migrations))

	// Get already applied migrations
	applied, err := m.getAppliedMigrations(ctx)
	if err != nil {
		m.logger.Error("failed to get applied migrations", "error", err)
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Apply pending migrations
	for _, migration := range migrations {
		if _, exists := applied[migration.Name]; exists {
			m.logger.Debug("migration already applied", "name", migration.Name)
			continue
		}

		m.logger.Info("applying migration", "name", migration.Name)

		// Begin transaction for migration
		tx, err := m.db.BeginTx(ctx, nil)
		if err != nil {
			m.logger.Error("failed to begin transaction", "error", err, "migration", migration.Name)
			return fmt.Errorf("failed to begin transaction for migration %s: %w", migration.Name, err)
		}

		// Execute migration SQL
		if _, err := tx.ExecContext(ctx, migration.Content); err != nil {
			tx.Rollback()
			m.logger.Error("failed to execute migration", "error", err, "migration", migration.Name)
			return fmt.Errorf("failed to execute migration %s: %w", migration.Name, err)
		}

		// Record migration as applied
		query := `INSERT INTO _migrations (name, applied_at) VALUES (?, ?)`
		if _, err := tx.ExecContext(ctx, query, migration.Name, time.Now()); err != nil {
			tx.Rollback()
			m.logger.Error("failed to record migration", "error", err, "migration", migration.Name)
			return fmt.Errorf("failed to record migration %s: %w", migration.Name, err)
		}

		// Commit transaction
		if err := tx.Commit(); err != nil {
			m.logger.Error("failed to commit migration transaction", "error", err, "migration", migration.Name)
			return fmt.Errorf("failed to commit migration %s: %w", migration.Name, err)
		}

		m.logger.Info("migration applied successfully", "name", migration.Name)
	}

	m.logger.Info("all pending migrations completed")
	return nil
}

// loadMigrations reads all SQL migration files from the filesystem
func (m *MigrationManager) loadMigrations() ([]Migration, error) {
	var migrations []Migration

	// Read all files from migrations directory
	entries, err := os.ReadDir(m.migrationsPath)
	if err != nil {
		// If migrations directory doesn't exist, that's ok - just no migrations to apply
		if os.IsNotExist(err) {
			m.logger.Warn("migrations directory not found", "path", m.migrationsPath)
			return migrations, nil
		}
		return nil, fmt.Errorf("failed to read migrations directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		// Read migration file content
		filePath := filepath.Join(m.migrationsPath, entry.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read migration file %s: %w", entry.Name(), err)
		}

		migrations = append(migrations, Migration{
			Name:    entry.Name(),
			Content: string(content),
		})
	}

	if len(migrations) == 0 {
		m.logger.Warn("no migration files found")
	}

	return migrations, nil
}

// getAppliedMigrations returns a map of applied migration names
func (m *MigrationManager) getAppliedMigrations(ctx context.Context) (map[string]bool, error) {
	applied := make(map[string]bool)

	query := `SELECT name FROM _migrations`
	rows, err := m.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query applied migrations: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("failed to scan migration name: %w", err)
		}
		applied[name] = true
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating applied migrations: %w", err)
	}

	return applied, nil
}

// GetAppliedMigrationCount returns the number of applied migrations
func (m *MigrationManager) GetAppliedMigrationCount(ctx context.Context) (int, error) {
	var count int

	query := `SELECT COUNT(*) FROM _migrations`
	if err := m.db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count applied migrations: %w", err)
	}

	return count, nil
}

// GetLastAppliedMigration returns the name and timestamp of the last applied migration
func (m *MigrationManager) GetLastAppliedMigration(ctx context.Context) (string, time.Time, error) {
	var name string
	var appliedAt time.Time

	query := `SELECT name, applied_at FROM _migrations ORDER BY applied_at DESC LIMIT 1`
	err := m.db.QueryRowContext(ctx, query).Scan(&name, &appliedAt)
	if err == sql.ErrNoRows {
		return "", time.Time{}, nil
	}
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to query last migration: %w", err)
	}

	return name, appliedAt, nil
}
