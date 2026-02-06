package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"testing"
	"time"
)

func TestRecurringWeeklyGenerationIdempotency(t *testing.T) {
	t.Parallel()

	st, _ := newStoreWithRawData(t, []byte("[]\n"))
	fixedNow := time.Date(2026, 2, 10, 9, 0, 0, 0, time.UTC)
	st.nowFn = func() time.Time { return fixedNow }

	pattern, err := st.CreateRecurring(RecurringInput{
		Amount:    18.4,
		Category:  "food",
		Note:      "weekly recurrence",
		Frequency: "weekly",
		StartDate: time.Date(2026, 1, 13, 12, 0, 0, 0, time.UTC),
		Active:    true,
	})
	if err != nil {
		t.Fatalf("create recurring: %v", err)
	}

	wantDates := []string{"2026-01-13", "2026-01-20", "2026-01-27", "2026-02-03", "2026-02-10"}
	gotDates := recurringExpenseDates(st.List(ExpenseFilter{}), pattern.ID)
	if !slices.Equal(gotDates, wantDates) {
		t.Fatalf("unexpected weekly generated dates\nwant: %v\ngot:  %v", wantDates, gotDates)
	}

	// Running the sweep multiple times must not create duplicates.
	gotDatesAfterSecondSweep := recurringExpenseDates(st.List(ExpenseFilter{}), pattern.ID)
	if !slices.Equal(gotDatesAfterSecondSweep, wantDates) {
		t.Fatalf("expected idempotent generation\nwant: %v\ngot:  %v", wantDates, gotDatesAfterSecondSweep)
	}
}

func TestRecurringMonthlyMonthEndClamp(t *testing.T) {
	t.Parallel()

	st, _ := newStoreWithRawData(t, []byte("[]\n"))
	fixedNow := time.Date(2024, 4, 30, 10, 0, 0, 0, time.UTC)
	st.nowFn = func() time.Time { return fixedNow }

	endDate := time.Date(2024, 4, 30, 12, 0, 0, 0, time.UTC)
	pattern, err := st.CreateRecurring(RecurringInput{
		Amount:    100,
		Category:  "housing",
		Note:      "month-end clamp",
		Frequency: "monthly",
		StartDate: time.Date(2024, 1, 31, 12, 0, 0, 0, time.UTC),
		EndDate:   &endDate,
		Active:    true,
	})
	if err != nil {
		t.Fatalf("create recurring: %v", err)
	}

	wantDates := []string{"2024-01-31", "2024-02-29", "2024-03-31", "2024-04-30"}
	gotDates := recurringExpenseDates(st.List(ExpenseFilter{}), pattern.ID)
	if !slices.Equal(gotDates, wantDates) {
		t.Fatalf("unexpected monthly generated dates\nwant: %v\ngot:  %v", wantDates, gotDates)
	}

	patterns := st.ListRecurring()
	if len(patterns) != 1 {
		t.Fatalf("expected one recurring pattern, got %d", len(patterns))
	}
	if patterns[0].Active {
		t.Fatal("expected pattern to deactivate after passing end date")
	}

	gotDatesAfterSecondSweep := recurringExpenseDates(st.List(ExpenseFilter{}), pattern.ID)
	if !slices.Equal(gotDatesAfterSecondSweep, wantDates) {
		t.Fatalf("expected idempotent monthly generation\nwant: %v\ngot:  %v", wantDates, gotDatesAfterSecondSweep)
	}
}

func TestLegacyDataMigrationCompatibility(t *testing.T) {
	t.Parallel()

	legacyExpenses := []Expense{
		{
			ID:        "legacy-1",
			Amount:    11.5,
			Category:  "FOOD",
			Note:      "legacy entry",
			Date:      time.Date(2025, 12, 1, 12, 0, 0, 0, time.UTC),
			CreatedAt: time.Date(2025, 12, 1, 12, 0, 0, 0, time.UTC),
		},
	}
	raw, err := json.Marshal(legacyExpenses)
	if err != nil {
		t.Fatalf("marshal legacy payload: %v", err)
	}

	st, dataPath := newStoreWithRawData(t, append(raw, '\n'))
	loaded := st.List(ExpenseFilter{})
	if len(loaded) != 1 {
		t.Fatalf("expected 1 migrated legacy expense, got %d", len(loaded))
	}
	if loaded[0].Category != "food" {
		t.Fatalf("expected normalized category 'food', got %q", loaded[0].Category)
	}

	st.nowFn = func() time.Time { return time.Date(2025, 12, 15, 9, 0, 0, 0, time.UTC) }
	_, err = st.CreateRecurring(RecurringInput{
		Amount:    7,
		Category:  "transportation",
		Note:      "migrated recurring",
		Frequency: "weekly",
		StartDate: time.Date(2025, 12, 8, 12, 0, 0, 0, time.UTC),
		Active:    true,
	})
	if err != nil {
		t.Fatalf("create recurring after legacy migration: %v", err)
	}

	reloaded, err := NewStore(dataPath)
	if err != nil {
		t.Fatalf("reload store: %v", err)
	}
	if len(reloaded.ListRecurring()) != 1 {
		t.Fatalf("expected one recurring pattern after reload, got %d", len(reloaded.ListRecurring()))
	}

	persisted, err := os.ReadFile(dataPath)
	if err != nil {
		t.Fatalf("read persisted store file: %v", err)
	}
	if !bytes.Contains(persisted, []byte(`"version": 2`)) {
		t.Fatalf("expected versioned envelope after migration, file was: %s", string(persisted))
	}
	if !bytes.Contains(persisted, []byte(`"recurring_patterns"`)) {
		t.Fatalf("expected recurring_patterns in persisted envelope, file was: %s", string(persisted))
	}
}

func newStoreWithRawData(t *testing.T, raw []byte) (*Store, string) {
	t.Helper()

	tempDir := t.TempDir()
	dataPath := filepath.Join(tempDir, "expenses.json")
	if err := os.WriteFile(dataPath, raw, 0o644); err != nil {
		t.Fatalf("write store file: %v", err)
	}

	st, err := NewStore(dataPath)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	return st, dataPath
}

func recurringExpenseDates(expenses []Expense, patternID string) []string {
	dates := make([]string, 0)
	for _, expense := range expenses {
		if expense.RecurringPatternID == nil || *expense.RecurringPatternID != patternID {
			continue
		}
		dates = append(dates, expense.Date.UTC().Format("2006-01-02"))
	}
	sort.Strings(dates)
	return dates
}
