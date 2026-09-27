// Package database wires up the SQLite connection, migrations, and seed data
// shared by every domain. It is infrastructure, not a business domain.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/ghab/red-numbers/domains/categories"
	"github.com/ghab/red-numbers/domains/expenses"
	_ "github.com/mattn/go-sqlite3"
)

// Config holds database connection configuration.
type Config struct {
	DBPath             string
	MaxOpenConnections int
	MaxIdleConnections int
	ConnMaxLifetime    time.Duration
}

// Initialize creates a new SQLite database connection, applies migrations, and returns the connection.
func Initialize(ctx context.Context, config Config, logger *slog.Logger) (*sql.DB, error) {
	// Set defaults if not provided
	if config.MaxOpenConnections == 0 {
		config.MaxOpenConnections = 10
	}
	if config.MaxIdleConnections == 0 {
		config.MaxIdleConnections = 5
	}
	if config.ConnMaxLifetime == 0 {
		config.ConnMaxLifetime = 5 * time.Minute
	}

	// Open SQLite database
	logger.Info("Opening database", "path", config.DBPath)
	db, err := sql.Open("sqlite3", config.DBPath)
	if err != nil {
		logger.Error("failed to open database", "error", err)
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(config.MaxOpenConnections)
	db.SetMaxIdleConns(config.MaxIdleConnections)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)

	// Test the connection
	if err := db.PingContext(ctx); err != nil {
		logger.Error("failed to ping database", "error", err)
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("database connection established")

	// Apply migrations
	logger.Info("initializing database migrations")
	migrationManager := NewMigrationManager(db, logger)
	if err := migrationManager.ApplyMigrations(ctx); err != nil {
		logger.Error("failed to apply migrations", "error", err)
		db.Close()
		return nil, err
	}
	if err := categories.SeedDefaultCategories(ctx, db); err != nil {
		logger.Error("failed to seed default categories", "error", err)
		db.Close()
		return nil, err
	}
	if err := expenses.NewRepository(db).BackfillFingerprints(ctx); err != nil {
		logger.Error("failed to backfill expense fingerprints", "error", err)
		db.Close()
		return nil, err
	}

	// Log migration completion
	count, err := migrationManager.GetAppliedMigrationCount(ctx)
	if err != nil {
		logger.Warn("failed to get applied migration count", "error", err)
	} else {
		logger.Info("migrations applied successfully", "count", count)
	}

	return db, nil
}

// Close closes the database connection.
func Close(db *sql.DB, logger *slog.Logger) error {
	if db == nil {
		return nil
	}

	if err := db.Close(); err != nil {
		logger.Error("failed to close database", "error", err)
		return err
	}

	logger.Info("database connection closed")
	return nil
}
