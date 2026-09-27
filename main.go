package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/ghab/red-numbers/database"
	"github.com/ghab/red-numbers/domains/categories"
	"github.com/ghab/red-numbers/domains/classification"
	"github.com/ghab/red-numbers/domains/expenses"
	"github.com/ghab/red-numbers/domains/upload"
	"github.com/ghab/red-numbers/platform"
)

// Config holds application configuration from environment variables
type Config struct {
	DBPath        string
	PORT          int
	SessionSecret string
	LogLevel      slog.Level
}

// LoadConfig reads configuration from environment variables with defaults
func LoadConfig() Config {
	port := 8080
	if portStr := os.Getenv("PORT"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}

	logLevel := slog.LevelInfo
	if logLevelStr := os.Getenv("LOG_LEVEL"); logLevelStr != "" {
		if err := logLevel.UnmarshalText([]byte(logLevelStr)); err != nil {
			// If parsing fails, use default Info level
			logLevel = slog.LevelInfo
		}
	}

	return Config{
		DBPath:        getOrDefault("DB_PATH", "./expenses.db"),
		PORT:          port,
		SessionSecret: getOrDefault("SESSION_SECRET", "dev-secret-key-change-in-production"),
		LogLevel:      logLevel,
	}
}

// getOrDefault returns environment variable or default value
func getOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// initLogging sets up structured logging with the configured log level
func initLogging(logLevel slog.Level) {
	opts := &slog.HandlerOptions{
		Level: logLevel,
	}
	handler := slog.NewTextHandler(os.Stdout, opts)
	slog.SetDefault(slog.New(handler))
}

func main() {
	// Load configuration
	config := LoadConfig()

	// Initialize structured logging
	initLogging(config.LogLevel)

	// Log startup information
	logger := slog.Default()
	logger.Info("Starting Expense Tracking Application",
		slog.String("port", fmt.Sprintf("%d", config.PORT)),
		slog.String("db_path", config.DBPath),
		slog.String("log_level", config.LogLevel.String()),
	)

	// Initialize database with migrations
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := database.Initialize(ctx, database.Config{
		DBPath:             config.DBPath,
		MaxOpenConnections: 10,
		MaxIdleConnections: 5,
		ConnMaxLifetime:    5 * time.Minute,
	}, logger)
	if err != nil {
		logger.Error("Failed to initialize database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer database.Close(db, logger)

	// Shared dependencies passed to every domain handler.
	deps := platform.Dependencies{Logger: logger}

	// Repositories: the abstraction each domain uses to read/write its data.
	categoryRepo := categories.NewRepository(db)
	expenseRepo := expenses.NewRepository(db)
	classificationRepo := classification.NewRepository(db)

	// Services: built once here and injected into handlers, rather than
	// constructed inside the handlers themselves.
	classifier := classification.NewClassifier()
	classificationService := classification.NewService(classificationRepo)
	expenseService := expenses.NewService(expenseRepo, categoryRepo)
	categoryService := categories.NewService(categoryRepo)
	uploadService := upload.NewService(logger, upload.NewCSVParser(), classifier, expenseRepo, categoryRepo, classificationRepo)

	// One handler per domain.
	uploadHandler := upload.NewHandler(deps, uploadService)
	expenseHandler := expenses.NewHandler(deps, expenseService)
	classificationHandler := classification.NewHandler(deps, classificationService)
	categoryHandler := categories.NewHandler(deps, categoryService)

	// Create HTTP router
	mux := http.NewServeMux()

	// Register basic health check endpoint
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		logger.DebugContext(r.Context(), "Health check requested")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","timestamp":"%s"}`, time.Now().Format(time.RFC3339))
	})

	// Register domain routes
	mux.HandleFunc("GET /", expenseHandler.HandleGetDashboard)
	mux.HandleFunc("POST /expenses/delete-all", expenseHandler.HandlePostDeleteAll)
	mux.HandleFunc("GET /expenses/{id}", expenseHandler.HandleGetExpenseDetail)
	mux.HandleFunc("POST /expenses/{id}", expenseHandler.HandlePostExpenseDetail)

	mux.HandleFunc("GET /upload", uploadHandler.HandleGetUpload)
	mux.HandleFunc("POST /upload", uploadHandler.HandlePostUpload)

	mux.HandleFunc("GET /classification-log", classificationHandler.HandleGetClassificationLog)

	mux.HandleFunc("POST /categories", categoryHandler.HandlePostCreate)

	// Create HTTP server with reasonable timeouts
	server := &http.Server{
		Addr:           fmt.Sprintf(":%d", config.PORT),
		Handler:        mux,
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   15 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	// Start server
	logger.Info("HTTP server listening",
		slog.String("address", server.Addr),
	)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("Server error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
