package main

import (
	"log/slog"
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	t.Setenv("DB_PATH", "")
	t.Setenv("PORT", "")
	t.Setenv("SESSION_SECRET", "")
	t.Setenv("LOG_LEVEL", "")

	config := LoadConfig()

	if config.DBPath != "./expenses.db" {
		t.Errorf("expected default DBPath, got %q", config.DBPath)
	}
	if config.PORT != 8080 {
		t.Errorf("expected default port 8080, got %d", config.PORT)
	}
	if config.SessionSecret == "" {
		t.Error("expected a non-empty default session secret")
	}
	if config.LogLevel != slog.LevelInfo {
		t.Errorf("expected default log level Info, got %v", config.LogLevel)
	}
}

func TestLoadConfigReadsEnvironment(t *testing.T) {
	t.Setenv("DB_PATH", "/tmp/custom.db")
	t.Setenv("PORT", "9090")
	t.Setenv("SESSION_SECRET", "s3cr3t")
	t.Setenv("LOG_LEVEL", "DEBUG")

	config := LoadConfig()

	if config.DBPath != "/tmp/custom.db" {
		t.Errorf("expected DBPath from env, got %q", config.DBPath)
	}
	if config.PORT != 9090 {
		t.Errorf("expected port 9090, got %d", config.PORT)
	}
	if config.SessionSecret != "s3cr3t" {
		t.Errorf("expected session secret from env, got %q", config.SessionSecret)
	}
	if config.LogLevel != slog.LevelDebug {
		t.Errorf("expected debug log level, got %v", config.LogLevel)
	}
}

func TestLoadConfigFallsBackOnInvalidPortOrLogLevel(t *testing.T) {
	t.Setenv("PORT", "not-a-number")
	t.Setenv("LOG_LEVEL", "not-a-level")

	config := LoadConfig()

	if config.PORT != 8080 {
		t.Errorf("expected fallback port 8080 for invalid PORT, got %d", config.PORT)
	}
	if config.LogLevel != slog.LevelInfo {
		t.Errorf("expected fallback log level Info for invalid LOG_LEVEL, got %v", config.LogLevel)
	}
}

func TestGetOrDefault(t *testing.T) {
	t.Setenv("SOME_TEST_VAR", "")
	if got := getOrDefault("SOME_TEST_VAR", "fallback"); got != "fallback" {
		t.Errorf("expected fallback for unset var, got %q", got)
	}
	t.Setenv("SOME_TEST_VAR", "value")
	if got := getOrDefault("SOME_TEST_VAR", "fallback"); got != "value" {
		t.Errorf("expected env value, got %q", got)
	}
}
