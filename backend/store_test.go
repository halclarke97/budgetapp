package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreLoadsLegacyExpensesAndPersistsEnvelope(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "expenses.json")
	legacyExpense := []Expense{
		{
			ID:        "legacy-1",
			Amount:    19.25,
			Category:  "Food",
			Note:      "Lunch",
			Date:      time.Date(2026, time.January, 15, 12, 0, 0, 0, time.UTC),
			CreatedAt: time.Date(2026, time.January, 15, 12, 5, 0, 0, time.UTC),
		},
	}
	legacyBytes, err := json.Marshal(legacyExpense)
	if err != nil {
		t.Fatalf("marshal legacy expense: %v", err)
	}
	if err := os.WriteFile(path, append(legacyBytes, '\n'), 0o644); err != nil {
		t.Fatalf("write legacy data: %v", err)
	}

	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("create store from legacy data: %v", err)
	}

	expenses := store.List(ExpenseFilter{})
	if len(expenses) != 1 {
		t.Fatalf("expected 1 legacy expense, got %d", len(expenses))
	}
	if expenses[0].Category != "food" {
		t.Fatalf("expected normalized category 'food', got %q", expenses[0].Category)
	}

	if _, err := store.Create(ExpenseInput{
		Amount:   8.5,
		Category: "transportation",
		Note:     "Bus",
		Date:     time.Date(2026, time.January, 16, 12, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("create expense after legacy load: %v", err)
	}

	assertEnvelopeShape(t, path)
}

func TestStoreLoadsVersionedEnvelope(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "expenses.json")
	patternID := "pattern-1"
	envelope := storeEnvelope{
		Version: storeDataVersion,
		Expenses: []Expense{
			{
				ID:                 "expense-1",
				Amount:             42,
				Category:           "Shopping",
				Note:               "Monthly subscription",
				Date:               time.Date(2026, time.February, 1, 12, 0, 0, 0, time.UTC),
				CreatedAt:          time.Date(2026, time.February, 1, 12, 1, 0, 0, time.UTC),
				RecurringPatternID: &patternID,
			},
		},
		RecurringPatterns: []RecurringPattern{
			{
				ID:          patternID,
				Amount:      42,
				Category:    "Shopping",
				Note:        "Monthly subscription",
				Frequency:   "monthly",
				StartDate:   time.Date(2026, time.February, 1, 12, 0, 0, 0, time.UTC),
				NextRunDate: time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC),
				Active:      true,
				CreatedAt:   time.Date(2026, time.February, 1, 12, 1, 0, 0, time.UTC),
				UpdatedAt:   time.Date(2026, time.February, 1, 12, 1, 0, 0, time.UTC),
			},
		},
	}
	bytes, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	if err := os.WriteFile(path, append(bytes, '\n'), 0o644); err != nil {
		t.Fatalf("write envelope data: %v", err)
	}

	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("create store from envelope data: %v", err)
	}

	expenses := store.List(ExpenseFilter{})
	if len(expenses) != 1 {
		t.Fatalf("expected 1 expense from envelope, got %d", len(expenses))
	}
	if expenses[0].Category != "shopping" {
		t.Fatalf("expected normalized category 'shopping', got %q", expenses[0].Category)
	}
	if expenses[0].RecurringPatternID == nil || *expenses[0].RecurringPatternID != patternID {
		t.Fatalf("expected recurring_pattern_id %q on expense", patternID)
	}

	patterns := store.ListRecurringPatterns()
	if len(patterns) != 1 {
		t.Fatalf("expected 1 recurring pattern, got %d", len(patterns))
	}
	if patterns[0].Category != "shopping" {
		t.Fatalf("expected normalized pattern category 'shopping', got %q", patterns[0].Category)
	}
}

func TestRecurringPatternCRUD(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "expenses.json")
	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	startDate := time.Date(2026, time.February, 6, 12, 0, 0, 0, time.UTC)
	created, err := store.CreateRecurringPattern(RecurringPatternInput{
		Amount:      50,
		Category:    "Housing",
		Note:        "Rent",
		Frequency:   "monthly",
		StartDate:   startDate,
		NextRunDate: startDate,
		Active:      true,
	})
	if err != nil {
		t.Fatalf("create recurring pattern: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected created recurring pattern id")
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatal("expected create timestamps to be set")
	}

	fetched, err := store.GetRecurringPattern(created.ID)
	if err != nil {
		t.Fatalf("get recurring pattern: %v", err)
	}
	if fetched.Note != "Rent" {
		t.Fatalf("expected note 'Rent', got %q", fetched.Note)
	}

	endDate := time.Date(2026, time.December, 31, 12, 0, 0, 0, time.UTC)
	updated, err := store.UpdateRecurringPattern(created.ID, RecurringPatternInput{
		Amount:      55,
		Category:    "Housing",
		Note:        "Updated rent",
		Frequency:   "monthly",
		StartDate:   created.StartDate,
		NextRunDate: created.NextRunDate.AddDate(0, 1, 0),
		EndDate:     &endDate,
		Active:      false,
	})
	if err != nil {
		t.Fatalf("update recurring pattern: %v", err)
	}
	if updated.Amount != 55 || updated.Note != "Updated rent" {
		t.Fatalf("unexpected update values: %+v", updated)
	}
	if updated.Active {
		t.Fatal("expected updated recurring pattern to be inactive")
	}
	if !updated.CreatedAt.Equal(created.CreatedAt) {
		t.Fatal("expected created_at to remain unchanged after update")
	}

	if err := store.DeleteRecurringPattern(created.ID); err != nil {
		t.Fatalf("delete recurring pattern: %v", err)
	}
	if _, err := store.GetRecurringPattern(created.ID); !errors.Is(err, ErrRecurringPatternNotFound) {
		t.Fatalf("expected ErrRecurringPatternNotFound after delete, got %v", err)
	}
	if len(store.ListRecurringPatterns()) != 0 {
		t.Fatal("expected no recurring patterns after delete")
	}

	assertEnvelopeShape(t, path)
}

func TestLegacyMigrationRemainsCompatibleWithRecurringOperations(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "expenses.json")
	blankPatternID := "   "
	legacyExpense := []Expense{
		{
			ID:                 "legacy-compat-1",
			Amount:             17.5,
			Category:           "Food",
			Note:               "Brunch",
			Date:               time.Date(2026, time.January, 5, 12, 0, 0, 0, time.UTC),
			CreatedAt:          time.Date(2026, time.January, 5, 12, 5, 0, 0, time.UTC),
			RecurringPatternID: &blankPatternID,
		},
	}

	legacyBytes, err := json.Marshal(legacyExpense)
	if err != nil {
		t.Fatalf("marshal legacy expense: %v", err)
	}
	if err := os.WriteFile(path, append(legacyBytes, '\n'), 0o644); err != nil {
		t.Fatalf("write legacy data: %v", err)
	}

	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("create store from legacy data: %v", err)
	}

	expenses := store.List(ExpenseFilter{})
	if len(expenses) != 1 {
		t.Fatalf("expected 1 legacy expense, got %d", len(expenses))
	}
	if expenses[0].RecurringPatternID != nil {
		t.Fatalf("expected blank recurring_pattern_id to be normalized to nil, got %+v", expenses[0].RecurringPatternID)
	}

	start := time.Date(2026, time.January, 12, 12, 0, 0, 0, time.UTC)
	pattern, err := store.CreateRecurringPattern(RecurringPatternInput{
		Amount:      9.99,
		Category:    "shopping",
		Note:        "Coffee subscription",
		Frequency:   "weekly",
		StartDate:   start,
		NextRunDate: start,
		Active:      true,
	})
	if err != nil {
		t.Fatalf("create recurring pattern after migration: %v", err)
	}

	generated, err := store.SweepRecurringExpenses(time.Date(2026, time.January, 26, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("sweep recurring expenses after migration: %v", err)
	}
	if generated != 3 {
		t.Fatalf("expected 3 generated recurring expenses after migration, got %d", generated)
	}

	reopened, err := NewStore(path)
	if err != nil {
		t.Fatalf("reopen migrated store: %v", err)
	}
	reopenedPatterns := reopened.ListRecurringPatterns()
	if len(reopenedPatterns) != 1 || reopenedPatterns[0].ID != pattern.ID {
		t.Fatalf("expected recurring pattern %q to persist after reopen", pattern.ID)
	}
	reopenedExpenses := reopened.List(ExpenseFilter{})
	if len(reopenedExpenses) != 4 {
		t.Fatalf("expected 4 total expenses after reopen (1 legacy + 3 generated), got %d", len(reopenedExpenses))
	}

	assertEnvelopeShape(t, path)
}

func assertEnvelopeShape(t *testing.T, path string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted file: %v", err)
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("decode persisted envelope: %v", err)
	}

	versionRaw, ok := root["version"]
	if !ok {
		t.Fatal("persisted file missing version")
	}
	expensesRaw, ok := root["expenses"]
	if !ok {
		t.Fatal("persisted file missing expenses")
	}
	patternsRaw, ok := root["recurring_patterns"]
	if !ok {
		t.Fatal("persisted file missing recurring_patterns")
	}

	var version int
	if err := json.Unmarshal(versionRaw, &version); err != nil {
		t.Fatalf("decode envelope version: %v", err)
	}
	if version != storeDataVersion {
		t.Fatalf("expected envelope version %d, got %d", storeDataVersion, version)
	}

	var expenses []Expense
	if err := json.Unmarshal(expensesRaw, &expenses); err != nil {
		t.Fatalf("decode envelope expenses: %v", err)
	}
	var patterns []RecurringPattern
	if err := json.Unmarshal(patternsRaw, &patterns); err != nil {
		t.Fatalf("decode envelope recurring patterns: %v", err)
	}
}
