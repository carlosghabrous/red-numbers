package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// DatabaseConfig holds database connection configuration
type DatabaseConfig struct {
	DBPath             string
	MaxOpenConnections int
	MaxIdleConnections int
	ConnMaxLifetime    time.Duration
}

// InitializeDatabase creates a new SQLite database connection, applies migrations, and returns the connection
func InitializeDatabase(ctx context.Context, config DatabaseConfig, logger *slog.Logger) (*sql.DB, error) {
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
	if err := SeedDefaultCategories(ctx, db); err != nil {
		logger.Error("failed to seed default categories", "error", err)
		db.Close()
		return nil, err
	}
	if err := NewExpenseRepository(db).BackfillFingerprints(ctx); err != nil {
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

// CloseDatabase closes the database connection
func CloseDatabase(db *sql.DB, logger *slog.Logger) error {
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
