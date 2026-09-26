package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
)

// PreparedStatementCache manages a cache of prepared statements for performance optimization
type PreparedStatementCache struct {
	statements map[string]*sql.Stmt
	mu         sync.RWMutex
}

// NewPreparedStatementCache creates a new prepared statement cache
func NewPreparedStatementCache() *PreparedStatementCache {
	return &PreparedStatementCache{
		statements: make(map[string]*sql.Stmt),
	}
}

// GetOrPrepare retrieves a cached prepared statement or prepares a new one
// This method provides a thread-safe way to cache and reuse prepared statements
func (psc *PreparedStatementCache) GetOrPrepare(ctx context.Context, db *sql.DB, query string) (*sql.Stmt, error) {
	// First, check if statement is already cached (read lock for performance)
	psc.mu.RLock()
	if stmt, exists := psc.statements[query]; exists {
		psc.mu.RUnlock()
		return stmt, nil
	}
	psc.mu.RUnlock()

	// Statement not in cache, prepare it
	stmt, err := db.PrepareContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare statement: %w", err)
	}

	// Store in cache (write lock)
	psc.mu.Lock()
	psc.statements[query] = stmt
	psc.mu.Unlock()

	return stmt, nil
}

// Close closes all cached prepared statements
func (psc *PreparedStatementCache) Close() error {
	logger := slog.Default()
	psc.mu.Lock()
	defer psc.mu.Unlock()

	var errs []error
	for query, stmt := range psc.statements {
		if err := stmt.Close(); err != nil {
			logger.Warn("Failed to close prepared statement",
				slog.String("query_prefix", truncateQuery(query, 50)),
				slog.String("error", err.Error()),
			)
			errs = append(errs, err)
		}
	}
	psc.statements = make(map[string]*sql.Stmt)

	if len(errs) > 0 {
		return fmt.Errorf("failed to close some prepared statements: %v", errs)
	}
	return nil
}

// Stats returns statistics about the prepared statement cache
func (psc *PreparedStatementCache) Stats() map[string]interface{} {
	psc.mu.RLock()
	defer psc.mu.RUnlock()

	return map[string]interface{}{
		"cached_statements": len(psc.statements),
	}
}

// truncateQuery returns a truncated version of a query for logging
func truncateQuery(query string, maxLen int) string {
	if len(query) > maxLen {
		return query[:maxLen] + "..."
	}
	return query
}

// SeedDefaultCategories inserts the default expense categories into the database
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

// GetConnectionPoolStats returns statistics about the database connection pool
func GetConnectionPoolStats(db *sql.DB) map[string]interface{} {
	stats := db.Stats()
	return map[string]interface{}{
		"open_connections":     stats.OpenConnections,
		"in_use":               stats.InUse,
		"idle":                 stats.Idle,
		"wait_count":           stats.WaitCount,
		"wait_duration_ms":     stats.WaitDuration.Milliseconds(),
		"max_idle_closed":      stats.MaxIdleClosed,
		"max_lifetime_closed":  stats.MaxLifetimeClosed,
		"max_open_connections": 10,
		"max_idle_connections": 5,
	}
}
