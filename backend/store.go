package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrNotFound = errors.New("expense not found")
var ErrRecurringNotFound = errors.New("recurring pattern not found")

type ExpenseFilter struct {
	Category string
	From     *time.Time
	To       *time.Time
}

type ExpenseInput struct {
	Amount             float64
	Category           string
	Note               string
	Date               time.Time
	RecurringPatternID *string
}

type RecurringPatternInput struct {
	Amount    float64
	Category  string
	Note      string
	Frequency string
	StartDate time.Time
	EndDate   *time.Time
	Active    bool
}

type Store struct {
	mu                sync.RWMutex
	filePath          string
	expenses          []Expense
	recurringPatterns []RecurringPattern
}

type persistedData struct {
	Version           int                `json:"version"`
	Expenses          []Expense          `json:"expenses"`
	RecurringPatterns []RecurringPattern `json:"recurring_patterns"`
}

func NewStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(path, []byte("[]\n"), 0o644); err != nil {
			return nil, fmt.Errorf("initialize data file: %w", err)
		}
	}

	s := &Store{filePath: path}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return fmt.Errorf("read data file: %w", err)
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		s.expenses = []Expense{}
		s.recurringPatterns = []RecurringPattern{}
		return nil
	}

	if strings.HasPrefix(trimmed, "[") {
		var expenses []Expense
		if err := json.Unmarshal([]byte(trimmed), &expenses); err != nil {
			return fmt.Errorf("parse data file: %w", err)
		}
		for i := range expenses {
			expenses[i].Category = normalizeStoreCategory(expenses[i].Category)
		}
		s.expenses = expenses
		s.recurringPatterns = []RecurringPattern{}
		return nil
	}

	var persisted persistedData
	if err := json.Unmarshal([]byte(trimmed), &persisted); err != nil {
		return fmt.Errorf("parse data file: %w", err)
	}
	for i := range persisted.Expenses {
		persisted.Expenses[i].Category = normalizeStoreCategory(persisted.Expenses[i].Category)
	}
	for i := range persisted.RecurringPatterns {
		persisted.RecurringPatterns[i].Category = normalizeStoreCategory(persisted.RecurringPatterns[i].Category)
		persisted.RecurringPatterns[i].Frequency = normalizeFrequency(persisted.RecurringPatterns[i].Frequency)
		if persisted.RecurringPatterns[i].NextRunDate.IsZero() {
			persisted.RecurringPatterns[i].NextRunDate = persisted.RecurringPatterns[i].StartDate.UTC()
		}
	}
	if persisted.Expenses == nil {
		persisted.Expenses = []Expense{}
	}
	if persisted.RecurringPatterns == nil {
		persisted.RecurringPatterns = []RecurringPattern{}
	}
	if persisted.Version == 0 {
		persisted.Version = 2
	}

	s.expenses = persisted.Expenses
	s.recurringPatterns = persisted.RecurringPatterns
	return nil
}

func (s *Store) List(filter ExpenseFilter) []Expense {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Expense, 0, len(s.expenses))
	for _, expense := range s.expenses {
		if filter.Category != "" && expense.Category != filter.Category {
			continue
		}
		if filter.From != nil && expense.Date.Before(startOfDay(*filter.From)) {
			continue
		}
		if filter.To != nil && expense.Date.After(endOfDay(*filter.To)) {
			continue
		}
		result = append(result, expense)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Date.After(result[j].Date)
	})

	return result
}

func (s *Store) Get(id string) (Expense, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, expense := range s.expenses {
		if expense.ID == id {
			return expense, nil
		}
	}
	return Expense{}, ErrNotFound
}

func (s *Store) Create(input ExpenseInput) (Expense, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	expense := Expense{
		ID:        newID(),
		Amount:    input.Amount,
		Category:  normalizeStoreCategory(input.Category),
		Note:      strings.TrimSpace(input.Note),
		Date:      input.Date.UTC(),
		CreatedAt: now,
	}
	if expense.Date.IsZero() {
		expense.Date = now
	}
	if input.RecurringPatternID != nil {
		patternID := strings.TrimSpace(*input.RecurringPatternID)
		if patternID != "" {
			expense.RecurringPatternID = &patternID
		}
	}

	s.expenses = append(s.expenses, expense)
	if err := s.persistLocked(); err != nil {
		return Expense{}, err
	}
	return expense, nil
}

func (s *Store) CreateWithRecurring(input ExpenseInput, recurring RecurringPatternInput) (Expense, RecurringPattern, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	startDate := input.Date.UTC()
	if startDate.IsZero() {
		startDate = now
	}

	pattern := RecurringPattern{
		ID:          newID(),
		Amount:      recurring.Amount,
		Category:    normalizeStoreCategory(recurring.Category),
		Note:        strings.TrimSpace(recurring.Note),
		Frequency:   normalizeFrequency(recurring.Frequency),
		StartDate:   startDate,
		NextRunDate: advanceRecurringDate(startDate, recurring.Frequency, startDate.Day()),
		EndDate:     copyTimePtr(recurring.EndDate),
		Active:      true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if pattern.NextRunDate.IsZero() {
		pattern.NextRunDate = startDate
	}

	patternID := pattern.ID
	expense := Expense{
		ID:                 newID(),
		Amount:             input.Amount,
		Category:           normalizeStoreCategory(input.Category),
		Note:               strings.TrimSpace(input.Note),
		Date:               startDate,
		CreatedAt:          now,
		RecurringPatternID: &patternID,
	}

	s.recurringPatterns = append(s.recurringPatterns, pattern)
	s.expenses = append(s.expenses, expense)
	if err := s.persistLocked(); err != nil {
		return Expense{}, RecurringPattern{}, err
	}
	return expense, pattern, nil
}

func (s *Store) Update(id string, input ExpenseInput) (Expense, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, expense := range s.expenses {
		if expense.ID != id {
			continue
		}
		expense.Amount = input.Amount
		expense.Category = normalizeStoreCategory(input.Category)
		expense.Note = strings.TrimSpace(input.Note)
		if !input.Date.IsZero() {
			expense.Date = input.Date.UTC()
		}
		s.expenses[i] = expense
		if err := s.persistLocked(); err != nil {
			return Expense{}, err
		}
		return expense, nil
	}
	return Expense{}, ErrNotFound
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, expense := range s.expenses {
		if expense.ID != id {
			continue
		}
		s.expenses = append(s.expenses[:i], s.expenses[i+1:]...)
		return s.persistLocked()
	}
	return ErrNotFound
}

func (s *Store) ListRecurring() []RecurringPattern {
	s.mu.RLock()
	defer s.mu.RUnlock()

	patterns := make([]RecurringPattern, len(s.recurringPatterns))
	copy(patterns, s.recurringPatterns)
	sort.Slice(patterns, func(i, j int) bool {
		if patterns[i].Active != patterns[j].Active {
			return patterns[i].Active
		}
		if !patterns[i].NextRunDate.Equal(patterns[j].NextRunDate) {
			return patterns[i].NextRunDate.Before(patterns[j].NextRunDate)
		}
		return patterns[i].CreatedAt.After(patterns[j].CreatedAt)
	})
	return patterns
}

func (s *Store) CreateRecurring(input RecurringPatternInput) (RecurringPattern, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	startDate := input.StartDate.UTC()
	if startDate.IsZero() {
		startDate = now
	}

	pattern := RecurringPattern{
		ID:          newID(),
		Amount:      input.Amount,
		Category:    normalizeStoreCategory(input.Category),
		Note:        strings.TrimSpace(input.Note),
		Frequency:   normalizeFrequency(input.Frequency),
		StartDate:   startDate,
		NextRunDate: startDate,
		EndDate:     copyTimePtr(input.EndDate),
		Active:      input.Active,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	s.recurringPatterns = append(s.recurringPatterns, pattern)
	if err := s.persistLocked(); err != nil {
		return RecurringPattern{}, err
	}
	return pattern, nil
}

func (s *Store) UpdateRecurring(id string, input RecurringPatternInput) (RecurringPattern, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.recurringPatterns {
		if s.recurringPatterns[i].ID != id {
			continue
		}
		original := s.recurringPatterns[i]
		updated := original
		updated.Amount = input.Amount
		updated.Category = normalizeStoreCategory(input.Category)
		updated.Note = strings.TrimSpace(input.Note)
		updated.Frequency = normalizeFrequency(input.Frequency)
		updated.StartDate = input.StartDate.UTC()
		updated.EndDate = copyTimePtr(input.EndDate)
		updated.Active = input.Active
		updated.UpdatedAt = time.Now().UTC()

		if updated.NextRunDate.IsZero() || !sameUTCDate(updated.StartDate, original.StartDate) || updated.Frequency != original.Frequency {
			updated.NextRunDate = updated.StartDate
		}
		if updated.EndDate != nil && updated.NextRunDate.After(endOfDay(*updated.EndDate)) {
			updated.Active = false
		}

		s.recurringPatterns[i] = updated
		if err := s.persistLocked(); err != nil {
			return RecurringPattern{}, err
		}
		return updated, nil
	}

	return RecurringPattern{}, ErrRecurringNotFound
}

func (s *Store) DeactivateRecurring(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.recurringPatterns {
		if s.recurringPatterns[i].ID != id {
			continue
		}
		if !s.recurringPatterns[i].Active {
			return nil
		}
		s.recurringPatterns[i].Active = false
		s.recurringPatterns[i].UpdatedAt = time.Now().UTC()
		return s.persistLocked()
	}
	return ErrRecurringNotFound
}

func (s *Store) UpcomingOccurrences(days int, now time.Time) []UpcomingOccurrence {
	s.mu.RLock()
	defer s.mu.RUnlock()

	start := startOfDay(now.UTC())
	horizon := endOfDay(now.UTC().AddDate(0, 0, days))
	occurrences := make([]UpcomingOccurrence, 0)

	for _, pattern := range s.recurringPatterns {
		if !pattern.Active {
			continue
		}
		next := pattern.NextRunDate.UTC()
		if next.IsZero() {
			next = pattern.StartDate.UTC()
		}
		if next.IsZero() {
			continue
		}

		for !next.After(horizon) {
			if pattern.EndDate != nil && next.After(endOfDay(*pattern.EndDate)) {
				break
			}
			if !next.Before(start) {
				occurrences = append(occurrences, UpcomingOccurrence{
					RecurringPatternID: pattern.ID,
					Amount:             pattern.Amount,
					Category:           pattern.Category,
					Note:               pattern.Note,
					Date:               next,
				})
			}
			advanced := advanceRecurringDate(next, pattern.Frequency, pattern.StartDate.Day())
			if advanced.IsZero() || !advanced.After(next) {
				break
			}
			next = advanced
		}
	}

	sort.Slice(occurrences, func(i, j int) bool {
		if !occurrences[i].Date.Equal(occurrences[j].Date) {
			return occurrences[i].Date.Before(occurrences[j].Date)
		}
		if occurrences[i].Category != occurrences[j].Category {
			return occurrences[i].Category < occurrences[j].Category
		}
		return occurrences[i].Amount < occurrences[j].Amount
	})
	return occurrences
}

func (s *Store) RunRecurringSweep(now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now = now.UTC()
	today := endOfDay(now)
	changed := false

	for i := range s.recurringPatterns {
		pattern := &s.recurringPatterns[i]
		if !pattern.Active {
			continue
		}
		if pattern.NextRunDate.IsZero() {
			if pattern.StartDate.IsZero() {
				continue
			}
			pattern.NextRunDate = pattern.StartDate.UTC()
			changed = true
		}

		for !pattern.NextRunDate.After(today) {
			if pattern.EndDate != nil && pattern.NextRunDate.After(endOfDay(*pattern.EndDate)) {
				pattern.Active = false
				pattern.UpdatedAt = now
				changed = true
				break
			}

			if !s.expenseExistsForPatternDateLocked(pattern.ID, pattern.NextRunDate) {
				patternID := pattern.ID
				s.expenses = append(s.expenses, Expense{
					ID:                 newID(),
					Amount:             pattern.Amount,
					Category:           normalizeStoreCategory(pattern.Category),
					Note:               strings.TrimSpace(pattern.Note),
					Date:               pattern.NextRunDate.UTC(),
					CreatedAt:          now,
					RecurringPatternID: &patternID,
				})
				changed = true
			}

			nextRunDate := advanceRecurringDate(pattern.NextRunDate, pattern.Frequency, pattern.StartDate.Day())
			if nextRunDate.IsZero() || !nextRunDate.After(pattern.NextRunDate) {
				pattern.Active = false
				pattern.UpdatedAt = now
				changed = true
				break
			}
			pattern.NextRunDate = nextRunDate
			pattern.UpdatedAt = now
			changed = true
		}

		if pattern.EndDate != nil && pattern.NextRunDate.After(endOfDay(*pattern.EndDate)) && pattern.Active {
			pattern.Active = false
			pattern.UpdatedAt = now
			changed = true
		}
	}

	if changed {
		if err := s.persistLocked(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) expenseExistsForPatternDateLocked(patternID string, date time.Time) bool {
	for _, expense := range s.expenses {
		if expense.RecurringPatternID == nil || *expense.RecurringPatternID != patternID {
			continue
		}
		if sameUTCDate(expense.Date, date) {
			return true
		}
	}
	return false
}

func (s *Store) Stats(period string, now time.Time) Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := Stats{
		TotalExpenses: len(s.expenses),
		Period:        period,
	}
	if stats.Period == "" {
		stats.Period = "month"
	}

	categoryTotals := map[string]float64{}
	trendTotals := map[string]float64{}
	periodStart := startForPeriod(stats.Period, now.UTC())

	for _, expense := range s.expenses {
		stats.TotalAmount += expense.Amount
		categoryTotals[expense.Category] += expense.Amount
		if !expense.Date.Before(periodStart) {
			stats.PeriodTotal += expense.Amount
			day := expense.Date.UTC().Format("2006-01-02")
			trendTotals[day] += expense.Amount
		}
	}

	stats.ByCategory = make([]CategoryTotal, 0, len(categoryTotals))
	for category, total := range categoryTotals {
		stats.ByCategory = append(stats.ByCategory, CategoryTotal{Category: category, Total: total})
	}
	sort.Slice(stats.ByCategory, func(i, j int) bool {
		return stats.ByCategory[i].Total > stats.ByCategory[j].Total
	})

	dates := make([]string, 0, len(trendTotals))
	for day := range trendTotals {
		dates = append(dates, day)
	}
	sort.Strings(dates)
	stats.Trend = make([]DailyTotal, 0, len(dates))
	for _, day := range dates {
		stats.Trend = append(stats.Trend, DailyTotal{Date: day, Total: trendTotals[day]})
	}

	return stats
}

func (s *Store) persistLocked() error {
	data, err := json.MarshalIndent(persistedData{
		Version:           2,
		Expenses:          s.expenses,
		RecurringPatterns: s.recurringPatterns,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal data: %w", err)
	}
	tmpPath := s.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write temp data file: %w", err)
	}
	if err := os.Rename(tmpPath, s.filePath); err != nil {
		return fmt.Errorf("replace data file: %w", err)
	}
	return nil
}

func newID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("exp_%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func normalizeStoreCategory(category string) string {
	cat := strings.ToLower(strings.TrimSpace(category))
	if cat == "" {
		return "other"
	}
	return cat
}

func normalizeFrequency(frequency string) string {
	return strings.ToLower(strings.TrimSpace(frequency))
}

func isSupportedFrequency(frequency string) bool {
	normalized := normalizeFrequency(frequency)
	return normalized == "weekly" || normalized == "monthly"
}

func advanceRecurringDate(t time.Time, frequency string, anchorDay int) time.Time {
	if anchorDay <= 0 {
		anchorDay = t.Day()
	}
	switch normalizeFrequency(frequency) {
	case "weekly":
		return t.UTC().AddDate(0, 0, 7)
	case "monthly":
		return addOneMonthClamped(t.UTC(), anchorDay)
	default:
		return time.Time{}
	}
}

func addOneMonthClamped(t time.Time, anchorDay int) time.Time {
	year, month, _ := t.Date()
	hour, minute, second := t.Clock()
	nanosecond := t.Nanosecond()

	nextMonth := month + 1
	maxDay := daysInMonth(year, nextMonth)
	day := anchorDay
	if day > maxDay {
		day = maxDay
	}

	return time.Date(year, nextMonth, day, hour, minute, second, nanosecond, time.UTC)
}

func daysInMonth(year int, month time.Month) int {
	startOfNext := time.Date(year, month+1, 1, 0, 0, 0, 0, time.UTC)
	endOfCurrent := startOfNext.AddDate(0, 0, -1)
	return endOfCurrent.Day()
}

func sameUTCDate(a, b time.Time) bool {
	a = a.UTC()
	b = b.UTC()
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}

func copyTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copied := value.UTC()
	return &copied
}

func startForPeriod(period string, now time.Time) time.Time {
	now = now.UTC()
	switch period {
	case "week":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		start := now.AddDate(0, 0, -(weekday - 1))
		return startOfDay(start)
	case "month":
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	default:
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
}

func startOfDay(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func endOfDay(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC)
}
