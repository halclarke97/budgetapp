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
var ErrRecurringPatternNotFound = errors.New("recurring pattern not found")

type ExpenseFilter struct {
	Category string
	From     *time.Time
	To       *time.Time
}

type ExpenseInput struct {
	Amount   float64
	Category string
	Note     string
	Date     time.Time
}

type RecurringPatternInput struct {
	Amount    float64
	Category  string
	Note      string
	Frequency string
	StartDate time.Time
	EndDate   *time.Time
}

type persistentDataEnvelope struct {
	Version           int                `json:"version"`
	Expenses          []Expense          `json:"expenses"`
	RecurringPatterns []RecurringPattern `json:"recurring_patterns"`
}

type Store struct {
	mu                sync.RWMutex
	filePath          string
	expenses          []Expense
	recurringPatterns []RecurringPattern
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
	if len(data) == 0 {
		s.expenses = []Expense{}
		s.recurringPatterns = []RecurringPattern{}
		return nil
	}

	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		s.expenses = []Expense{}
		s.recurringPatterns = []RecurringPattern{}
		return nil
	}

	if strings.HasPrefix(trimmed, "[") {
		var expenses []Expense
		if err := json.Unmarshal(data, &expenses); err != nil {
			return fmt.Errorf("parse legacy data file: %w", err)
		}
		for i := range expenses {
			expenses[i].Category = normalizeStoreCategory(expenses[i].Category)
		}
		s.expenses = expenses
		s.recurringPatterns = []RecurringPattern{}
		return nil
	}

	var envelope persistentDataEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("parse data file: %w", err)
	}
	if envelope.Expenses == nil {
		envelope.Expenses = []Expense{}
	}
	if envelope.RecurringPatterns == nil {
		envelope.RecurringPatterns = []RecurringPattern{}
	}

	for i := range envelope.Expenses {
		envelope.Expenses[i].Category = normalizeStoreCategory(envelope.Expenses[i].Category)
	}
	for i := range envelope.RecurringPatterns {
		envelope.RecurringPatterns[i].Category = normalizeStoreCategory(envelope.RecurringPatterns[i].Category)
		envelope.RecurringPatterns[i].Frequency = normalizeFrequency(envelope.RecurringPatterns[i].Frequency)
	}

	s.expenses = envelope.Expenses
	s.recurringPatterns = envelope.RecurringPatterns
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

	s.expenses = append(s.expenses, expense)
	if err := s.persistLocked(); err != nil {
		return Expense{}, err
	}
	return expense, nil
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

func (s *Store) ListRecurring() []RecurringPattern {
	s.mu.RLock()
	defer s.mu.RUnlock()

	patterns := append([]RecurringPattern(nil), s.recurringPatterns...)
	sort.Slice(patterns, func(i, j int) bool {
		if patterns[i].Active != patterns[j].Active {
			return patterns[i].Active
		}
		if patterns[i].NextRun.Equal(patterns[j].NextRun) {
			return patterns[i].CreatedAt.Before(patterns[j].CreatedAt)
		}
		return patterns[i].NextRun.Before(patterns[j].NextRun)
	})
	return patterns
}

func (s *Store) CreateRecurring(input RecurringPatternInput) (RecurringPattern, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	frequency := normalizeFrequency(input.Frequency)
	startDate := normalizeDate(input.StartDate)
	if startDate.IsZero() {
		startDate = normalizeDate(now)
	}

	var endDate *time.Time
	if input.EndDate != nil {
		normalized := normalizeDate(*input.EndDate)
		if normalized.Before(startDate) {
			return RecurringPattern{}, errors.New("end date cannot be before start date")
		}
		endDate = &normalized
	}

	pattern := RecurringPattern{
		ID:        newID(),
		Amount:    input.Amount,
		Category:  normalizeStoreCategory(input.Category),
		Note:      strings.TrimSpace(input.Note),
		Frequency: frequency,
		StartDate: startDate,
		NextRun:   startDate,
		EndDate:   endDate,
		Active:    true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	today := normalizeDate(now)
	for pattern.NextRun.Before(today) {
		pattern.NextRun = advanceRecurringDate(pattern.NextRun, pattern.Frequency)
	}
	if pattern.EndDate != nil && pattern.NextRun.After(*pattern.EndDate) {
		pattern.Active = false
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

	frequency := normalizeFrequency(input.Frequency)
	startDate := normalizeDate(input.StartDate)
	if startDate.IsZero() {
		startDate = normalizeDate(time.Now().UTC())
	}

	var endDate *time.Time
	if input.EndDate != nil {
		normalized := normalizeDate(*input.EndDate)
		if normalized.Before(startDate) {
			return RecurringPattern{}, errors.New("end date cannot be before start date")
		}
		endDate = &normalized
	}

	for i, pattern := range s.recurringPatterns {
		if pattern.ID != id {
			continue
		}

		now := time.Now().UTC()
		pattern.Amount = input.Amount
		pattern.Category = normalizeStoreCategory(input.Category)
		pattern.Note = strings.TrimSpace(input.Note)
		pattern.Frequency = frequency
		pattern.StartDate = startDate
		pattern.EndDate = endDate
		pattern.NextRun = startDate
		if pattern.Active {
			today := normalizeDate(now)
			for pattern.NextRun.Before(today) {
				pattern.NextRun = advanceRecurringDate(pattern.NextRun, pattern.Frequency)
			}
			if pattern.EndDate != nil && pattern.NextRun.After(*pattern.EndDate) {
				pattern.Active = false
			}
		}
		pattern.UpdatedAt = now

		s.recurringPatterns[i] = pattern
		if err := s.persistLocked(); err != nil {
			return RecurringPattern{}, err
		}
		return pattern, nil
	}

	return RecurringPattern{}, ErrRecurringPatternNotFound
}

func (s *Store) DeactivateRecurring(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, pattern := range s.recurringPatterns {
		if pattern.ID != id {
			continue
		}
		if !pattern.Active {
			return nil
		}
		pattern.Active = false
		pattern.UpdatedAt = time.Now().UTC()
		s.recurringPatterns[i] = pattern
		return s.persistLocked()
	}
	return ErrRecurringPatternNotFound
}

func (s *Store) UpcomingRecurring(days int, now time.Time) []UpcomingRecurringOccurrence {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if days <= 0 {
		days = 30
	}

	start := normalizeDate(now)
	horizon := start.AddDate(0, 0, days)
	upcoming := make([]UpcomingRecurringOccurrence, 0, len(s.recurringPatterns))

	for _, pattern := range s.recurringPatterns {
		if !pattern.Active {
			continue
		}

		nextDate := pattern.NextRun
		if nextDate.IsZero() {
			nextDate = pattern.StartDate
		}
		nextDate = normalizeDate(nextDate)
		if nextDate.IsZero() {
			continue
		}

		for !nextDate.After(horizon) {
			if pattern.EndDate != nil && nextDate.After(*pattern.EndDate) {
				break
			}
			if !nextDate.Before(start) {
				upcoming = append(upcoming, UpcomingRecurringOccurrence{
					PatternID: pattern.ID,
					Date:      nextDate,
					Amount:    pattern.Amount,
					Category:  pattern.Category,
					Note:      pattern.Note,
				})
			}
			nextDate = advanceRecurringDate(nextDate, pattern.Frequency)
		}
	}

	sort.Slice(upcoming, func(i, j int) bool {
		if upcoming[i].Date.Equal(upcoming[j].Date) {
			if upcoming[i].Category == upcoming[j].Category {
				return upcoming[i].Amount > upcoming[j].Amount
			}
			return upcoming[i].Category < upcoming[j].Category
		}
		return upcoming[i].Date.Before(upcoming[j].Date)
	})

	return upcoming
}

func (s *Store) SweepRecurring(now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now = now.UTC()
	today := normalizeDate(now)
	changed := false

	for i := range s.recurringPatterns {
		pattern := s.recurringPatterns[i]
		if !pattern.Active {
			continue
		}

		if pattern.NextRun.IsZero() {
			pattern.NextRun = normalizeDate(pattern.StartDate)
			if pattern.NextRun.IsZero() {
				pattern.NextRun = today
			}
			changed = true
		}

		for !pattern.NextRun.After(today) {
			if pattern.EndDate != nil && pattern.NextRun.After(*pattern.EndDate) {
				pattern.Active = false
				pattern.UpdatedAt = now
				changed = true
				break
			}

			if !s.hasGeneratedExpenseLocked(pattern.ID, pattern.NextRun) {
				patternID := pattern.ID
				s.expenses = append(s.expenses, Expense{
					ID:                 newID(),
					Amount:             pattern.Amount,
					Category:           pattern.Category,
					Note:               pattern.Note,
					Date:               pattern.NextRun,
					CreatedAt:          now,
					RecurringPatternID: &patternID,
				})
				changed = true
			}

			pattern.NextRun = advanceRecurringDate(pattern.NextRun, pattern.Frequency)
			pattern.UpdatedAt = now
			changed = true

			if pattern.EndDate != nil && pattern.NextRun.After(*pattern.EndDate) {
				pattern.Active = false
				pattern.UpdatedAt = now
				changed = true
				break
			}
		}

		s.recurringPatterns[i] = pattern
	}

	if !changed {
		return nil
	}

	sort.Slice(s.expenses, func(i, j int) bool {
		return s.expenses[i].Date.After(s.expenses[j].Date)
	})

	return s.persistLocked()
}

func (s *Store) hasGeneratedExpenseLocked(patternID string, date time.Time) bool {
	dateKey := normalizeDate(date).Format("2006-01-02")
	for _, expense := range s.expenses {
		if expense.RecurringPatternID == nil || *expense.RecurringPatternID != patternID {
			continue
		}
		if normalizeDate(expense.Date).Format("2006-01-02") == dateKey {
			return true
		}
	}
	return false
}

func (s *Store) persistLocked() error {
	envelope := persistentDataEnvelope{
		Version:           2,
		Expenses:          s.expenses,
		RecurringPatterns: s.recurringPatterns,
	}

	data, err := json.MarshalIndent(envelope, "", "  ")
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
	value := strings.ToLower(strings.TrimSpace(frequency))
	switch value {
	case "weekly", "monthly":
		return value
	default:
		return "monthly"
	}
}

func normalizeDate(t time.Time) time.Time {
	if t.IsZero() {
		return time.Time{}
	}
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 12, 0, 0, 0, time.UTC)
}

func advanceRecurringDate(date time.Time, frequency string) time.Time {
	date = normalizeDate(date)
	if date.IsZero() {
		return date
	}

	switch normalizeFrequency(frequency) {
	case "weekly":
		return normalizeDate(date.AddDate(0, 0, 7))
	case "monthly":
		nextMonth := date.AddDate(0, 1, 0)
		days := daysInMonth(nextMonth.Year(), nextMonth.Month())
		day := date.Day()
		if day > days {
			day = days
		}
		return time.Date(nextMonth.Year(), nextMonth.Month(), day, 12, 0, 0, 0, time.UTC)
	default:
		return normalizeDate(date.AddDate(0, 1, 0))
	}
}

func daysInMonth(year int, month time.Month) int {
	if month == time.December {
		return 31
	}
	firstOfNext := time.Date(year, month+1, 1, 0, 0, 0, 0, time.UTC)
	lastOfMonth := firstOfNext.AddDate(0, 0, -1)
	return lastOfMonth.Day()
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
