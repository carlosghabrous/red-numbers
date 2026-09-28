package upload

import (
	"io"
	"log/slog"
	"testing"

	"github.com/ghab/red-numbers/domains/classification"
)

func TestHasPersistence(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	classifyOnly := NewService(logger, NewCSVParser(), classification.NewClassifier(), nil, nil, nil)
	if classifyOnly.HasPersistence() {
		t.Error("expected HasPersistence to be false without an expense repository")
	}
}
