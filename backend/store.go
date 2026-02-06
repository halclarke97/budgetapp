package main

import (
	"bytes"
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

var (
	ErrNotFound         = errors.New("expense not found")
	ErrInvalidFrequency = errors.New("frequency must be 'weekly' or 'monthly'")
)

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

type RecurringInput struct {
	Amount    float64
	Category  string
	Note      string
	Frequency string
	StartDate time.Time
	EndDate   *time.Time
	Active    bool
}

type dataEnvelope struct {
	Version           int                `json:"version"`
	Expenses          []Expense          `json:"expenses"`
	RecurringPatterns []RecurringPattern `json:"recurring_patterns"`
}

type Store struct {
	mu                sync.RWMutex
	filePath          string
	expenses          []Expense
	recurringPatterns []RecurringPattern
	nowFn             func() time.Time
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

	s := &Store{
		filePath: path,
		nowFn:    func() time.Time { return time.Now().UTC() },
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	if err := s.runRecurringSweepAt(s.nowFn()); err != nil {
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

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		s.expenses = []Expense{}
		s.recurringPatterns = []RecurringPattern{}
		return nil
	}

	if trimmed[0] == '[' {
		// Legacy format: bare expense array.
		var legacyExpenses []Expense
		if err := json.Unmarshal(trimmed, &legacyExpenses); err != nil {
			return fmt.Errorf("parse data file: %w", err)
		}
		for i := range legacyExpenses {
			normalizeLoadedExpense(&legacyExpenses[i])
		}
		s.expenses = legacyExpenses
		s.recurringPatterns = []RecurringPattern{}
		return nil
	}

	var envelope dataEnvelope
	if err := json.Unmarshal(trimmed, &envelope); err != nil {
		return fmt.Errorf("parse data file: %w", err)
	}

	for i := range envelope.Expenses {
		normalizeLoadedExpense(&envelope.Expenses[i])
	}
	for i := range envelope.RecurringPatterns {
		normalizeLoadedPattern(&envelope.RecurringPatterns[i])
	}

	s.expenses = envelope.Expenses
	s.recurringPatterns = envelope.RecurringPatterns
	if s.expenses == nil {
		s.expenses = []Expense{}
	}
	if s.recurringPatterns == nil {
		s.recurringPatterns = []RecurringPattern{}
	}
	return nil
}

func (s *Store) List(filter ExpenseFilter) []Expense {
	_ = s.runRecurringSweepAt(s.nowFn())

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

	now := s.nowFn().UTC()
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

func (s *Store) ListRecurring() []RecurringPattern {
	_ = s.runRecurringSweepAt(s.nowFn())

	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]RecurringPattern, len(s.recurringPatterns))
	copy(result, s.recurringPatterns)
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})
	return result
}

func (s *Store) CreateRecurring(input RecurringInput) (RecurringPattern, error) {
	if err := validateRecurringInput(input); err != nil {
		return RecurringPattern{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.nowFn().UTC()
	frequency := normalizeFrequency(input.Frequency)
	startDate := startOfDay(input.StartDate)

	var endDate *time.Time
	if input.EndDate != nil {
		normalizedEnd := endOfDay(*input.EndDate)
		endDate = &normalizedEnd
	}

	pattern := RecurringPattern{
		ID:          newID(),
		Amount:      input.Amount,
		Category:    normalizeStoreCategory(input.Category),
		Note:        strings.TrimSpace(input.Note),
		Frequency:   frequency,
		StartDate:   startDate,
		NextRunDate: startDate,
		EndDate:     endDate,
		Active:      input.Active,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	s.recurringPatterns = append(s.recurringPatterns, pattern)
	if _, err := s.generateDueRecurringLocked(now); err != nil {
		return RecurringPattern{}, err
	}
	if err := s.persistLocked(); err != nil {
		return RecurringPattern{}, err
	}
	return pattern, nil
}

func (s *Store) UpdateRecurring(id string, input RecurringInput) (RecurringPattern, error) {
	if err := validateRecurringInput(input); err != nil {
		return RecurringPattern{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	frequency := normalizeFrequency(input.Frequency)
	startDate := startOfDay(input.StartDate)
	var endDate *time.Time
	if input.EndDate != nil {
		normalizedEnd := endOfDay(*input.EndDate)
		endDate = &normalizedEnd
	}
	now := s.nowFn().UTC()

	for i, pattern := range s.recurringPatterns {
		if pattern.ID != id {
			continue
		}

		pattern.Amount = input.Amount
		pattern.Category = normalizeStoreCategory(input.Category)
		pattern.Note = strings.TrimSpace(input.Note)
		pattern.Frequency = frequency
		pattern.StartDate = startDate
		pattern.EndDate = endDate
		pattern.Active = input.Active
		if pattern.NextRunDate.Before(startDate) {
			pattern.NextRunDate = startDate
		}
		if pattern.EndDate != nil && startOfDay(pattern.NextRunDate).After(*pattern.EndDate) {
			pattern.Active = false
		}
		pattern.UpdatedAt = now
		s.recurringPatterns[i] = pattern

		if _, err := s.generateDueRecurringLocked(now); err != nil {
			return RecurringPattern{}, err
		}
		if err := s.persistLocked(); err != nil {
			return RecurringPattern{}, err
		}
		return pattern, nil
	}

	return RecurringPattern{}, ErrNotFound
}

func (s *Store) DeactivateRecurring(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.nowFn().UTC()
	for i := range s.recurringPatterns {
		if s.recurringPatterns[i].ID != id {
			continue
		}
		s.recurringPatterns[i].Active = false
		s.recurringPatterns[i].UpdatedAt = now
		return s.persistLocked()
	}
	return ErrNotFound
}

func (s *Store) UpcomingRecurring(days int, now time.Time) []UpcomingRecurringOccurrence {
	if days <= 0 {
		days = 30
	}
	if days > 365 {
		days = 365
	}

	_ = s.runRecurringSweepAt(now)

	s.mu.RLock()
	defer s.mu.RUnlock()

	windowStart := startOfDay(now)
	windowEnd := endOfDay(windowStart.AddDate(0, 0, days))
	upcoming := make([]UpcomingRecurringOccurrence, 0)

	for _, pattern := range s.recurringPatterns {
		if !pattern.Active {
			continue
		}

		next := startOfDay(pattern.NextRunDate)
		for !next.After(windowEnd) {
			if pattern.EndDate != nil && next.After(*pattern.EndDate) {
				break
			}
			if !next.Before(windowStart) {
				upcoming = append(upcoming, UpcomingRecurringOccurrence{
					PatternID: pattern.ID,
					Date:      atNoonUTC(next),
					Amount:    pattern.Amount,
					Category:  pattern.Category,
					Note:      pattern.Note,
				})
			}
			advanced, err := advanceRecurringDate(pattern.Frequency, pattern.StartDate.Day(), next)
			if err != nil {
				break
			}
			next = startOfDay(advanced)
		}
	}

	sort.Slice(upcoming, func(i, j int) bool {
		if upcoming[i].Date.Equal(upcoming[j].Date) {
			return upcoming[i].PatternID < upcoming[j].PatternID
		}
		return upcoming[i].Date.Before(upcoming[j].Date)
	})

	return upcoming
}

func (s *Store) Stats(period string, now time.Time) Stats {
	_ = s.runRecurringSweepAt(now)

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

func (s *Store) runRecurringSweepAt(now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	changed, err := s.generateDueRecurringLocked(now.UTC())
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	return s.persistLocked()
}

func (s *Store) generateDueRecurringLocked(now time.Time) (bool, error) {
	if len(s.recurringPatterns) == 0 {
		return false, nil
	}

	changed := false
	dueLimit := endOfDay(now)

	for i := range s.recurringPatterns {
		pattern := &s.recurringPatterns[i]
		if !pattern.Active {
			continue
		}
		if pattern.NextRunDate.IsZero() {
			pattern.NextRunDate = startOfDay(pattern.StartDate)
			changed = true
		}

		for !startOfDay(pattern.NextRunDate).After(dueLimit) {
			nextRunDay := startOfDay(pattern.NextRunDate)
			if pattern.EndDate != nil && nextRunDay.After(*pattern.EndDate) {
				pattern.Active = false
				pattern.UpdatedAt = now
				changed = true
				break
			}

			if !s.hasOccurrenceLocked(pattern.ID, nextRunDay) {
				patternID := pattern.ID
				s.expenses = append(s.expenses, Expense{
					ID:                 newID(),
					Amount:             pattern.Amount,
					Category:           pattern.Category,
					Note:               pattern.Note,
					Date:               atNoonUTC(nextRunDay),
					CreatedAt:          now,
					RecurringPatternID: &patternID,
				})
				changed = true
			}

			nextRunDate, err := advanceRecurringDate(pattern.Frequency, pattern.StartDate.Day(), nextRunDay)
			if err != nil {
				pattern.Active = false
				pattern.UpdatedAt = now
				changed = true
				break
			}
			pattern.NextRunDate = startOfDay(nextRunDate)
			pattern.UpdatedAt = now
			changed = true

			if pattern.EndDate != nil && startOfDay(pattern.NextRunDate).After(*pattern.EndDate) {
				pattern.Active = false
				pattern.UpdatedAt = now
				changed = true
				break
			}
		}
	}

	return changed, nil
}

func (s *Store) hasOccurrenceLocked(patternID string, date time.Time) bool {
	target := startOfDay(date)
	for _, expense := range s.expenses {
		if expense.RecurringPatternID == nil || *expense.RecurringPatternID != patternID {
			continue
		}
		if startOfDay(expense.Date).Equal(target) {
			return true
		}
	}
	return false
}

func (s *Store) persistLocked() error {
	envelope := dataEnvelope{
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

func normalizeLoadedExpense(expense *Expense) {
	expense.Category = normalizeStoreCategory(expense.Category)
	if !expense.Date.IsZero() {
		expense.Date = expense.Date.UTC()
	}
	if !expense.CreatedAt.IsZero() {
		expense.CreatedAt = expense.CreatedAt.UTC()
	}
}

func normalizeLoadedPattern(pattern *RecurringPattern) {
	pattern.Category = normalizeStoreCategory(pattern.Category)
	pattern.Frequency = normalizeFrequency(pattern.Frequency)
	pattern.StartDate = startOfDay(pattern.StartDate)
	if pattern.NextRunDate.IsZero() {
		pattern.NextRunDate = pattern.StartDate
	}
	pattern.NextRunDate = startOfDay(pattern.NextRunDate)
	if pattern.EndDate != nil {
		normalized := endOfDay(*pattern.EndDate)
		pattern.EndDate = &normalized
	}
	if !pattern.CreatedAt.IsZero() {
		pattern.CreatedAt = pattern.CreatedAt.UTC()
	}
	if !pattern.UpdatedAt.IsZero() {
		pattern.UpdatedAt = pattern.UpdatedAt.UTC()
	}
}

func validateRecurringInput(input RecurringInput) error {
	if input.Amount <= 0 {
		return errors.New("amount must be greater than zero")
	}
	if strings.TrimSpace(input.Frequency) == "" {
		return ErrInvalidFrequency
	}
	if normalizeFrequency(input.Frequency) == "" {
		return ErrInvalidFrequency
	}
	if input.StartDate.IsZero() {
		return errors.New("start date is required")
	}
	if input.EndDate != nil && endOfDay(*input.EndDate).Before(startOfDay(input.StartDate)) {
		return errors.New("end date must be on or after start date")
	}
	return nil
}

func normalizeFrequency(frequency string) string {
	switch strings.ToLower(strings.TrimSpace(frequency)) {
	case "weekly":
		return "weekly"
	case "monthly":
		return "monthly"
	default:
		return ""
	}
}

func advanceRecurringDate(frequency string, preferredDay int, current time.Time) (time.Time, error) {
	current = startOfDay(current)
	switch normalizeFrequency(frequency) {
	case "weekly":
		return current.AddDate(0, 0, 7), nil
	case "monthly":
		year, month, _ := current.Date()
		nextMonth := month + 1
		nextYear := year
		if nextMonth > time.December {
			nextMonth = time.January
			nextYear++
		}
		if preferredDay < 1 {
			preferredDay = current.Day()
		}
		lastDay := daysInMonth(nextYear, nextMonth)
		if preferredDay > lastDay {
			preferredDay = lastDay
		}
		return time.Date(nextYear, nextMonth, preferredDay, 0, 0, 0, 0, time.UTC), nil
	default:
		return time.Time{}, ErrInvalidFrequency
	}
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func atNoonUTC(day time.Time) time.Time {
	day = startOfDay(day)
	return time.Date(day.Year(), day.Month(), day.Day(), 12, 0, 0, 0, time.UTC)
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
