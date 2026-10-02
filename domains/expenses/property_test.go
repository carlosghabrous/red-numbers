package expenses

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"
)

// These are lightweight property tests (task 10.13, optional): each runs the
// same invariant across many randomly generated inputs rather than a single
// hand-picked example, without pulling in an external quickcheck library.

// Property: for any expense, correcting its category (classify -> view ->
// update) preserves every other field and is reflected immediately when the
// expense is viewed again.
func TestPropertyCorrectionPreservesOtherFields(t *testing.T) {
	db := newExpenseTestDB(t)
	repository := NewRepository(db)
	rng := rand.New(rand.NewSource(1))
	ctx := context.Background()

	for i := 0; i < 30; i++ {
		original := Expense{
			Date:        time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, rng.Intn(365)),
			Description: fmt.Sprintf("Property expense %d", i),
			Amount:      -(rng.Float64() * 1000),
			Balance:     rng.Float64() * 5000,
			CategoryID:  1,
			ImportedAt:  time.Now(),
		}
		if err := repository.Create(ctx, &original); err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		newCategoryID := int64(2)
		if _, err := repository.UpdateCategory(ctx, original.ID, newCategoryID); err != nil {
			t.Fatalf("UpdateCategory failed: %v", err)
		}

		viewed, err := repository.GetByID(ctx, original.ID)
		if err != nil || viewed == nil {
			t.Fatalf("GetByID failed: %v", err)
		}
		if viewed.CategoryID != newCategoryID {
			t.Fatalf("expected category updated to %d, got %d", newCategoryID, viewed.CategoryID)
		}
		if !viewed.Date.Equal(original.Date) || viewed.Description != original.Description ||
			viewed.Amount != original.Amount || viewed.Balance != original.Balance {
			t.Fatalf("expected all non-category fields preserved, original=%+v viewed=%+v", original, viewed)
		}
	}
}

// Property: for any combination of category/date filters, the statistics
// widget's numbers are consistent with the expenses actually returned for
// those same filters (not just plausible-looking in isolation).
func TestPropertyStatisticsAreConsistentWithFilteredResults(t *testing.T) {
	db := newExpenseTestDB(t)
	repository := NewRepository(db)
	rng := rand.New(rand.NewSource(2))
	ctx := context.Background()

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 40; i++ {
		expense := Expense{
			Date:        base.AddDate(0, 0, rng.Intn(90)),
			Description: fmt.Sprintf("Stats expense %d", i),
			Amount:      float64(rng.Intn(400)-200) + 0.5, // mix of positive and negative
			Balance:     100,
			CategoryID:  int64(1 + rng.Intn(2)),
			ImportedAt:  base,
		}
		if err := repository.Create(ctx, &expense); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	filterCases := []FilterOptions{
		{},
		{CategoryIDs: []int64{1}},
		{CategoryIDs: []int64{2}},
		{StartDate: ptrTime(base.AddDate(0, 0, 30)), EndDate: ptrTime(base.AddDate(0, 0, 60))},
	}

	for _, options := range filterCases {
		results, err := repository.GetByFilterOptions(ctx, options)
		if err != nil {
			t.Fatalf("GetByFilterOptions failed: %v", err)
		}
		stats, err := repository.GetStatistics(ctx, options)
		if err != nil {
			t.Fatalf("GetStatistics failed: %v", err)
		}

		wantCount := len(results)
		wantSpendingCount := 0
		wantTotal := 0.0
		wantIncome := 0.0
		for _, expense := range results {
			if expense.Amount < 0 {
				wantSpendingCount++
				wantTotal += -expense.Amount
			} else if expense.Amount > 0 {
				wantIncome += expense.Amount
			}
		}

		if stats.TransactionCount != wantCount {
			t.Errorf("filters=%+v: TransactionCount=%d, want %d (len of GetByFilterOptions result)", options, stats.TransactionCount, wantCount)
		}
		if stats.SpendingCount != wantSpendingCount {
			t.Errorf("filters=%+v: SpendingCount=%d, want %d", options, stats.SpendingCount, wantSpendingCount)
		}
		if diff := stats.TotalIncome - wantIncome; diff > 0.001 || diff < -0.001 {
			t.Errorf("filters=%+v: TotalIncome=%v, want %v", options, stats.TotalIncome, wantIncome)
		}
		if diff := stats.TotalSpending - wantTotal; diff > 0.001 || diff < -0.001 {
			t.Errorf("filters=%+v: TotalSpending=%v, want %v", options, stats.TotalSpending, wantTotal)
		}
	}
}

func ptrTime(t time.Time) *time.Time { return &t }

// Property: re-classification (triggered by a correction) never changes the
// total number of expenses, and every expense sharing the corrected
// description ends up in the new category — regardless of how many share it
// or what their original categories were.
func TestPropertyReclassificationMaintainsDataIntegrity(t *testing.T) {
	db := newExpenseTestDB(t)
	repository := NewRepository(db)
	rng := rand.New(rand.NewSource(3))
	ctx := context.Background()

	for trial := 0; trial < 10; trial++ {
		description := fmt.Sprintf("Recurring merchant %d", trial)
		matchCount := 1 + rng.Intn(5)
		var ids []int64
		for i := 0; i < matchCount; i++ {
			expense := Expense{
				Date:        time.Now().AddDate(0, 0, -i),
				Description: description,
				Amount:      -(rng.Float64() * 100),
				Balance:     100,
				CategoryID:  1,
				ImportedAt:  time.Now(),
			}
			if err := repository.Create(ctx, &expense); err != nil {
				t.Fatalf("Create failed: %v", err)
			}
			ids = append(ids, expense.ID)
		}
		// An unrelated expense that must never be touched.
		unrelated := Expense{Date: time.Now(), Description: fmt.Sprintf("Unrelated %d", trial), Amount: -5, Balance: 100, CategoryID: 1, ImportedAt: time.Now()}
		if err := repository.Create(ctx, &unrelated); err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		beforeCount, err := repository.CountByFilterOptions(ctx, FilterOptions{})
		if err != nil {
			t.Fatalf("CountByFilterOptions failed: %v", err)
		}

		result, err := repository.UpdateCategory(ctx, ids[0], 2)
		if err != nil {
			t.Fatalf("UpdateCategory failed: %v", err)
		}
		if result.ChangedCount != matchCount-1 {
			t.Errorf("trial %d: expected %d other expenses re-classified, got %d", trial, matchCount-1, result.ChangedCount)
		}

		afterCount, err := repository.CountByFilterOptions(ctx, FilterOptions{})
		if err != nil {
			t.Fatalf("CountByFilterOptions failed: %v", err)
		}
		if afterCount != beforeCount {
			t.Errorf("trial %d: expected row count unchanged by re-classification, before=%d after=%d", trial, beforeCount, afterCount)
		}

		for _, id := range ids {
			expense, err := repository.GetByID(ctx, id)
			if err != nil || expense == nil || expense.CategoryID != 2 {
				t.Errorf("trial %d: expected expense %d re-classified to category 2, got %+v (err=%v)", trial, id, expense, err)
			}
		}
		untouched, err := repository.GetByID(ctx, unrelated.ID)
		if err != nil || untouched == nil || untouched.CategoryID != 1 {
			t.Errorf("trial %d: expected unrelated expense to keep its category, got %+v (err=%v)", trial, untouched, err)
		}
	}
}
