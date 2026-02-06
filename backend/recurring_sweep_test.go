package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSweepRecurringExpensesGeneratesAllDueAndAdvancesNextRunDate(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC)
	next := start
	pattern := RecurringPattern{
		ID:          "pat_weekly",
		Amount:      30,
		Category:    "food",
		Note:        "Lunch",
		Frequency:   "weekly",
		StartDate:   start,
		NextRunDate: next,
		Active:      true,
	}

	store := newStoreWithRecurringPatterns(t, []RecurringPattern{pattern}, nil)
	generated, err := store.SweepRecurringExpenses(time.Date(2026, time.January, 22, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("sweep recurring expenses: %v", err)
	}
	if generated != 4 {
		t.Fatalf("expected 4 generated expenses, got %d", generated)
	}

	expenses := store.List(ExpenseFilter{})
	if len(expenses) != 4 {
		t.Fatalf("expected 4 expenses in store, got %d", len(expenses))
	}

	wantDates := map[string]struct{}{
		"2026-01-01": {},
		"2026-01-08": {},
		"2026-01-15": {},
		"2026-01-22": {},
	}
	for _, expense := range expenses {
		if expense.RecurringPatternID == nil || *expense.RecurringPatternID != pattern.ID {
			t.Fatalf("expected generated expense with recurring pattern id %q, got %+v", pattern.ID, expense.RecurringPatternID)
		}
		if _, ok := wantDates[expense.Date.UTC().Format("2006-01-02")]; !ok {
			t.Fatalf("unexpected generated expense date: %s", expense.Date.UTC().Format("2006-01-02"))
		}
	}

	patterns := store.ListRecurringPatterns()
	if len(patterns) != 1 {
		t.Fatalf("expected 1 recurring pattern, got %d", len(patterns))
	}
	if got := patterns[0].NextRunDate.UTC(); !got.Equal(time.Date(2026, time.January, 29, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected next run date 2026-01-29, got %s", got)
	}
}

func TestSweepRecurringExpensesIsIdempotentForPatternDate(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC)
	pattern := RecurringPattern{
		ID:          "pat_idempotent",
		Amount:      45,
		Category:    "transportation",
		Note:        "Pass",
		Frequency:   "weekly",
		StartDate:   start,
		NextRunDate: start,
		Active:      true,
	}
	existingPatternID := pattern.ID
	existing := Expense{
		ID:                 "exp_existing",
		Amount:             45,
		Category:           "transportation",
		Note:               "Pass",
		Date:               start,
		CreatedAt:          start,
		RecurringPatternID: &existingPatternID,
	}

	store := newStoreWithRecurringPatterns(t, []RecurringPattern{pattern}, []Expense{existing})

	generated, err := store.SweepRecurringExpenses(time.Date(2026, time.January, 15, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("first sweep recurring expenses: %v", err)
	}
	if generated != 2 {
		t.Fatalf("expected 2 generated expenses (Jan 8 + Jan 15), got %d", generated)
	}

	generatedAgain, err := store.SweepRecurringExpenses(time.Date(2026, time.January, 15, 18, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("second sweep recurring expenses: %v", err)
	}
	if generatedAgain != 0 {
		t.Fatalf("expected second sweep to generate 0 expenses, got %d", generatedAgain)
	}

	expenses := store.List(ExpenseFilter{})
	if len(expenses) != 3 {
		t.Fatalf("expected 3 total expenses after idempotent sweeps, got %d", len(expenses))
	}

	seenDates := map[string]struct{}{}
	for _, expense := range expenses {
		if expense.RecurringPatternID == nil || *expense.RecurringPatternID != pattern.ID {
			continue
		}
		dateKey := expense.Date.UTC().Format("2006-01-02")
		if _, exists := seenDates[dateKey]; exists {
			t.Fatalf("duplicate expense generated for date %s", dateKey)
		}
		seenDates[dateKey] = struct{}{}
	}

	if len(seenDates) != 3 {
		t.Fatalf("expected recurring dates Jan 1/8/15, got %d unique dates", len(seenDates))
	}

	patterns := store.ListRecurringPatterns()
	if got := patterns[0].NextRunDate.UTC(); !got.Equal(time.Date(2026, time.January, 22, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected next run date 2026-01-22, got %s", got)
	}
}

func TestSweepRecurringExpensesMonthlyClampsToMonthEnd(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.January, 31, 12, 0, 0, 0, time.UTC)
	pattern := RecurringPattern{
		ID:          "pat_monthly",
		Amount:      100,
		Category:    "housing",
		Note:        "Rent",
		Frequency:   "monthly",
		StartDate:   start,
		NextRunDate: start,
		Active:      true,
	}

	store := newStoreWithRecurringPatterns(t, []RecurringPattern{pattern}, nil)
	generated, err := store.SweepRecurringExpenses(time.Date(2026, time.March, 31, 8, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("sweep recurring expenses: %v", err)
	}
	if generated != 3 {
		t.Fatalf("expected 3 generated monthly expenses, got %d", generated)
	}

	expenses := store.List(ExpenseFilter{})
	if len(expenses) != 3 {
		t.Fatalf("expected 3 monthly expenses, got %d", len(expenses))
	}

	wantDates := map[string]struct{}{
		"2026-01-31": {},
		"2026-02-28": {},
		"2026-03-31": {},
	}
	for _, expense := range expenses {
		if _, ok := wantDates[expense.Date.UTC().Format("2006-01-02")]; !ok {
			t.Fatalf("unexpected monthly generated date: %s", expense.Date.UTC().Format("2006-01-02"))
		}
	}

	patterns := store.ListRecurringPatterns()
	if got := patterns[0].NextRunDate.UTC(); !got.Equal(time.Date(2026, time.April, 30, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected next run date 2026-04-30 after month-end clamp, got %s", got)
	}
}

func TestSweepRecurringExpensesWeeklyStopsAfterEndDate(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, time.January, 15, 12, 0, 0, 0, time.UTC)
	pattern := RecurringPattern{
		ID:          "pat_weekly_end",
		Amount:      25,
		Category:    "food",
		Note:        "Meal prep",
		Frequency:   "weekly",
		StartDate:   start,
		NextRunDate: start,
		EndDate:     &endDate,
		Active:      true,
	}

	store := newStoreWithRecurringPatterns(t, []RecurringPattern{pattern}, nil)
	generated, err := store.SweepRecurringExpenses(time.Date(2026, time.January, 31, 8, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("sweep recurring expenses: %v", err)
	}
	if generated != 3 {
		t.Fatalf("expected 3 generated weekly expenses before end_date, got %d", generated)
	}

	expenses := store.List(ExpenseFilter{})
	if len(expenses) != 3 {
		t.Fatalf("expected 3 generated weekly expenses, got %d", len(expenses))
	}

	wantDates := map[string]struct{}{
		"2026-01-01": {},
		"2026-01-08": {},
		"2026-01-15": {},
	}
	for _, expense := range expenses {
		dateKey := expense.Date.UTC().Format("2006-01-02")
		if _, ok := wantDates[dateKey]; !ok {
			t.Fatalf("unexpected date generated past end_date: %s", dateKey)
		}
	}

	patterns := store.ListRecurringPatterns()
	if got := patterns[0].NextRunDate.UTC(); !got.Equal(time.Date(2026, time.January, 22, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected next run date to advance to first date after end_date, got %s", got)
	}
}

func TestSweepRecurringExpensesMonthlyClampsAcrossLeapAndThirtyDayMonths(t *testing.T) {
	t.Parallel()

	start := time.Date(2024, time.January, 31, 12, 0, 0, 0, time.UTC)
	pattern := RecurringPattern{
		ID:          "pat_monthly_leap",
		Amount:      120,
		Category:    "housing",
		Note:        "Lease",
		Frequency:   "monthly",
		StartDate:   start,
		NextRunDate: start,
		Active:      true,
	}

	store := newStoreWithRecurringPatterns(t, []RecurringPattern{pattern}, nil)
	generated, err := store.SweepRecurringExpenses(time.Date(2024, time.May, 31, 8, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("sweep recurring expenses: %v", err)
	}
	if generated != 5 {
		t.Fatalf("expected 5 generated monthly expenses, got %d", generated)
	}

	wantDates := map[string]struct{}{
		"2024-01-31": {},
		"2024-02-29": {},
		"2024-03-31": {},
		"2024-04-30": {},
		"2024-05-31": {},
	}
	for _, expense := range store.List(ExpenseFilter{}) {
		dateKey := expense.Date.UTC().Format("2006-01-02")
		if _, ok := wantDates[dateKey]; !ok {
			t.Fatalf("unexpected monthly generated date: %s", dateKey)
		}
	}

	patterns := store.ListRecurringPatterns()
	if got := patterns[0].NextRunDate.UTC(); !got.Equal(time.Date(2024, time.June, 30, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected next run date 2024-06-30 after monthly clamping, got %s", got)
	}
}

func TestSweepRecurringExpensesIsIdempotentWithPreexistingFutureOccurrences(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC)
	pattern := RecurringPattern{
		ID:          "pat_preexisting",
		Amount:      60,
		Category:    "transportation",
		Note:        "Transit pass",
		Frequency:   "weekly",
		StartDate:   start,
		NextRunDate: time.Date(2026, time.January, 8, 12, 0, 0, 0, time.UTC),
		Active:      true,
	}
	patternID := pattern.ID
	existing := []Expense{
		{
			ID:                 "exp_existing_8",
			Amount:             60,
			Category:           "transportation",
			Note:               "Transit pass",
			Date:               time.Date(2026, time.January, 8, 12, 0, 0, 0, time.UTC),
			CreatedAt:          start,
			RecurringPatternID: &patternID,
		},
		{
			ID:                 "exp_existing_22",
			Amount:             60,
			Category:           "transportation",
			Note:               "Transit pass",
			Date:               time.Date(2026, time.January, 22, 12, 0, 0, 0, time.UTC),
			CreatedAt:          start,
			RecurringPatternID: &patternID,
		},
	}

	store := newStoreWithRecurringPatterns(t, []RecurringPattern{pattern}, existing)
	generated, err := store.SweepRecurringExpenses(time.Date(2026, time.January, 22, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("sweep recurring expenses: %v", err)
	}
	if generated != 1 {
		t.Fatalf("expected only missing Jan 15 to be generated, got %d", generated)
	}

	expenses := store.List(ExpenseFilter{})
	if len(expenses) != 3 {
		t.Fatalf("expected 3 total recurring expenses, got %d", len(expenses))
	}

	seenDates := map[string]struct{}{}
	for _, expense := range expenses {
		if expense.RecurringPatternID == nil || *expense.RecurringPatternID != pattern.ID {
			continue
		}
		dateKey := expense.Date.UTC().Format("2006-01-02")
		if _, exists := seenDates[dateKey]; exists {
			t.Fatalf("duplicate expense generated for date %s", dateKey)
		}
		seenDates[dateKey] = struct{}{}
	}
	if len(seenDates) != 3 {
		t.Fatalf("expected exactly Jan 8/15/22 recurring dates, got %d", len(seenDates))
	}

	patterns := store.ListRecurringPatterns()
	if got := patterns[0].NextRunDate.UTC(); !got.Equal(time.Date(2026, time.January, 29, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected next run date 2026-01-29, got %s", got)
	}
}

func newStoreWithRecurringPatterns(t *testing.T, patterns []RecurringPattern, expenses []Expense) *Store {
	t.Helper()

	tempDir := t.TempDir()
	dataPath := filepath.Join(tempDir, "expenses.json")
	if expenses == nil {
		expenses = []Expense{}
	}
	if patterns == nil {
		patterns = []RecurringPattern{}
	}

	envelope := storeEnvelope{
		Version:           storeDataVersion,
		Expenses:          expenses,
		RecurringPatterns: patterns,
	}
	data, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		t.Fatalf("marshal data file: %v", err)
	}
	if err := os.WriteFile(dataPath, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("write data file: %v", err)
	}

	store, err := NewStore(dataPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	return store
}
