package upload

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ghab/red-numbers/domains/categories"
	"github.com/ghab/red-numbers/domains/classification"
	"github.com/ghab/red-numbers/domains/expenses"
)

// Service coordinates CSV parsing, classification, and persistence for an upload.
type Service struct {
	logger            *slog.Logger
	csvParser         *CSVParser
	classifier        *classification.Classifier
	expenseRepo       *expenses.Repository
	categoryRepo      *categories.Repository
	classificationLog *classification.Repository
}

// NewService creates an upload service from its already-constructed collaborators.
// expenseRepo, categoryRepo, and classificationLog may be nil to run in a
// classify-only mode (used by tests that don't need persistence).
func NewService(logger *slog.Logger, csvParser *CSVParser, classifier *classification.Classifier, expenseRepo *expenses.Repository, categoryRepo *categories.Repository, classificationLog *classification.Repository) *Service {
	return &Service{
		logger:            logger,
		csvParser:         csvParser,
		classifier:        classifier,
		expenseRepo:       expenseRepo,
		categoryRepo:      categoryRepo,
		classificationLog: classificationLog,
	}
}

// ParseFile parses a CSV file into expenses, reporting how many rows were skipped.
func (s *Service) ParseFile(filePath string) ([]expenses.Expense, int, error) {
	return s.csvParser.ParseCSVFileWithStats(filePath)
}

// ClassifyExpenses assigns a category and confidence level to each expense.
func (s *Service) ClassifyExpenses(ctx context.Context, expenseList []expenses.Expense) ([]classification.Classification, error) {
	classifications := make([]classification.Classification, len(expenseList))
	if s.categoryRepo == nil {
		for index := range expenseList {
			classifications[index] = s.classifier.Classify(expenseList[index].Description, expenseList[index].Amount)
		}
		return classifications, nil
	}

	allCategories, err := s.categoryRepo.GetAllCategories(ctx)
	if err != nil {
		return nil, err
	}
	categoryIDs := make(map[string]int64, len(allCategories))
	for _, category := range allCategories {
		categoryIDs[category.Name] = int64(category.ID)
	}

	// Descriptions the user has manually corrected before take priority over
	// the keyword classifier, so a re-import of a recurring merchant doesn't
	// repeat a mistake that was already fixed once.
	var learned map[string]string
	if s.expenseRepo != nil {
		learned, err = s.expenseRepo.GetLearnedCategories(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to load learned categories: %w", err)
		}
	}

	for index := range expenseList {
		expense := &expenseList[index]
		learnedCategory, isLearned := learned[classification.NormalizeDescription(expense.Description)]

		var result classification.Classification
		switch {
		case expense.Amount > 0:
			// The income rule is unconditional (see classifier.Classify) and
			// must win over a learned description, consistent with the
			// startup backfill that enforces it on every positive-amount row.
			result = classification.Classification{CategoryName: "income", Confidence: "high"}
		case isLearned:
			result = classification.Classification{CategoryName: learnedCategory, Confidence: "high"}
		default:
			result = s.classifier.Classify(expense.Description, expense.Amount)
		}

		categoryID, exists := categoryIDs[result.CategoryName]
		if !exists {
			return nil, fmt.Errorf("category %q is not seeded", result.CategoryName)
		}
		expense.CategoryID = categoryID
		expense.ConfidenceLevel = result.Confidence
		classifications[index] = result
	}
	return classifications, nil
}

// SaveExpenses persists expenses and logs their classification decisions.
// It is a no-op returning zero when the service has no expense repository configured.
func (s *Service) SaveExpenses(ctx context.Context, expenseList []expenses.Expense, classifications []classification.Classification) (int, error) {
	if s.expenseRepo == nil {
		return 0, nil
	}
	savedCount, err := s.expenseRepo.CreateBatchWithCount(ctx, expenseList)
	if err != nil {
		return 0, err
	}
	for index, result := range classifications {
		if err := s.classificationLog.Log(ctx, expenseList[index].ID, result); err != nil {
			s.logger.WarnContext(ctx, "Failed to log classification", slog.String("error", err.Error()))
		}
	}
	return savedCount, nil
}

// HasPersistence reports whether the service is configured to save expenses.
func (s *Service) HasPersistence() bool {
	return s.expenseRepo != nil
}
