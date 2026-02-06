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
	ErrNotFound                 = errors.New("expense not found")
	ErrRecurringPatternNotFound = errors.New("recurring pattern not found")
	ErrInvalidRecurringPattern  = errors.New("invalid recurring pattern")
)

const storeDataVersion = 2

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

type ExpenseRecurringInput struct {
	Frequency string
	EndDate   *time.Time
}

type RecurringPatternInput struct {
	Amount      float64
	Category    string
	Note        string
	Frequency   string
	StartDate   time.Time
	NextRunDate time.Time
	EndDate     *time.Time
	Active      bool
}

type storeEnvelope struct {
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
		if err := os.WriteFile(path, emptyStoreEnvelopeJSON(), 0o644); err != nil {
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

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		s.expenses = []Expense{}
		s.recurringPatterns = []RecurringPattern{}
		return nil
	}

	loadedLegacyFormat := false
	switch trimmed[0] {
	case '[':
		var expenses []Expense
		if err := json.Unmarshal(trimmed, &expenses); err != nil {
			return fmt.Errorf("parse legacy data file: %w", err)
		}
		for i := range expenses {
			normalizeLoadedExpense(&expenses[i])
		}
		s.expenses = expenses
		s.recurringPatterns = []RecurringPattern{}
		loadedLegacyFormat = true
	case '{':
		var envelope storeEnvelope
		if err := json.Unmarshal(trimmed, &envelope); err != nil {
			return fmt.Errorf("parse data file: %w", err)
		}
		if envelope.Expenses == nil {
			envelope.Expenses = []Expense{}
		}
		if envelope.RecurringPatterns == nil {
			envelope.RecurringPatterns = []RecurringPattern{}
		}
		for i := range envelope.Expenses {
			normalizeLoadedExpense(&envelope.Expenses[i])
		}
		for i := range envelope.RecurringPatterns {
			normalizeLoadedPattern(&envelope.RecurringPatterns[i])
		}
		s.expenses = envelope.Expenses
		s.recurringPatterns = envelope.RecurringPatterns
	default:
		return errors.New("data file must be JSON object or array")
	}

	if loadedLegacyFormat {
		if err := s.persistLocked(); err != nil {
			return fmt.Errorf("migrate legacy data file: %w", err)
		}
	}
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
	expense, _, err := s.CreateExpenseWithRecurring(input, nil)
	return expense, err
}

func (s *Store) CreateExpenseWithRecurring(input ExpenseInput, recurring *ExpenseRecurringInput) (Expense, *RecurringPattern, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	expenseDate := input.Date.UTC()
	if expenseDate.IsZero() {
		expenseDate = now
	}

	expense := Expense{
		ID:        newID(),
		Amount:    input.Amount,
		Category:  normalizeStoreCategory(input.Category),
		Note:      strings.TrimSpace(input.Note),
		Date:      expenseDate,
		CreatedAt: now,
	}

	var createdPattern *RecurringPattern
	if recurring != nil {
		frequency, err := normalizeRecurringFrequency(recurring.Frequency)
		if err != nil {
			return Expense{}, nil, err
		}

		var endDate *time.Time
		if recurring.EndDate != nil {
			end := recurring.EndDate.UTC()
			if end.Before(startOfDay(expenseDate)) {
				return Expense{}, nil, fmt.Errorf("%w: end_date must be on or after expense date", ErrInvalidRecurringPattern)
			}
			endDate = &end
		}

		patternID := newID()
		expense.RecurringPatternID = &patternID

		pattern, err := recurringPatternFromInput(patternID, now, RecurringPatternInput{
			Amount:      input.Amount,
			Category:    input.Category,
			Note:        input.Note,
			Frequency:   frequency,
			StartDate:   expenseDate,
			NextRunDate: expenseDate,
			EndDate:     endDate,
			Active:      true,
		})
		if err != nil {
			return Expense{}, nil, err
		}
		pattern.UpdatedAt = now
		s.recurringPatterns = append(s.recurringPatterns, pattern)
		createdPattern = &pattern
	}

	s.expenses = append(s.expenses, expense)
	if err := s.persistLocked(); err != nil {
		return Expense{}, nil, err
	}
	return expense, createdPattern, nil
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

func (s *Store) ListRecurringPatterns() []RecurringPattern {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]RecurringPattern, len(s.recurringPatterns))
	copy(result, s.recurringPatterns)
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

func (s *Store) UpcomingRecurringOccurrences(days int, now time.Time) []UpcomingRecurringOccurrence {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if days < 1 {
		days = 30
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}

	windowStart := startOfDay(now)
	windowEnd := endOfDay(now.AddDate(0, 0, days))
	occurrences := make([]UpcomingRecurringOccurrence, 0)

	for _, pattern := range s.recurringPatterns {
		if !pattern.Active {
			continue
		}

		frequency := normalizeFrequency(pattern.Frequency)
		if frequency == "" {
			continue
		}

		next := pattern.NextRunDate.UTC()
		if next.IsZero() {
			next = pattern.StartDate.UTC()
		}
		if next.IsZero() {
			continue
		}

		anchorDay := pattern.StartDate.Day()
		if anchorDay < 1 {
			anchorDay = next.Day()
		}

		var endDay time.Time
		hasEndDate := pattern.EndDate != nil
		if hasEndDate {
			endDay = endOfDay(pattern.EndDate.UTC())
		}

		for next.Before(windowStart) {
			if hasEndDate && next.After(endDay) {
				next = time.Time{}
				break
			}
			advanced, ok := advanceRecurringDate(next, frequency, anchorDay)
			if !ok || !advanced.After(next) {
				next = time.Time{}
				break
			}
			next = advanced
		}
		if next.IsZero() {
			continue
		}

		for !next.After(windowEnd) {
			if hasEndDate && next.After(endDay) {
				break
			}
			occurrences = append(occurrences, UpcomingRecurringOccurrence{
				RecurringPatternID: pattern.ID,
				Date:               next,
				Amount:             pattern.Amount,
				Category:           normalizeStoreCategory(pattern.Category),
				Note:               strings.TrimSpace(pattern.Note),
				Frequency:          frequency,
			})
			advanced, ok := advanceRecurringDate(next, frequency, anchorDay)
			if !ok || !advanced.After(next) {
				break
			}
			next = advanced
		}
	}

	sort.Slice(occurrences, func(i, j int) bool {
		if occurrences[i].Date.Equal(occurrences[j].Date) {
			return occurrences[i].RecurringPatternID < occurrences[j].RecurringPatternID
		}
		return occurrences[i].Date.Before(occurrences[j].Date)
	})
	return occurrences
}

func (s *Store) CreateRecurringPattern(input RecurringPatternInput) (RecurringPattern, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	pattern, err := recurringPatternFromInput("", time.Time{}, input)
	if err != nil {
		return RecurringPattern{}, err
	}
	now := time.Now().UTC()
	pattern.ID = newID()
	pattern.CreatedAt = now
	pattern.UpdatedAt = now

	s.recurringPatterns = append(s.recurringPatterns, pattern)
	if err := s.persistLocked(); err != nil {
		return RecurringPattern{}, err
	}
	return pattern, nil
}

func (s *Store) GetRecurringPattern(id string) (RecurringPattern, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, pattern := range s.recurringPatterns {
		if pattern.ID == id {
			return pattern, nil
		}
	}
	return RecurringPattern{}, ErrRecurringPatternNotFound
}

func (s *Store) UpdateRecurringPattern(id string, input RecurringPatternInput) (RecurringPattern, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, existing := range s.recurringPatterns {
		if existing.ID != id {
			continue
		}
		updated, err := recurringPatternFromInput(existing.ID, existing.CreatedAt, input)
		if err != nil {
			return RecurringPattern{}, err
		}
		updated.UpdatedAt = time.Now().UTC()
		s.recurringPatterns[i] = updated
		if err := s.persistLocked(); err != nil {
			return RecurringPattern{}, err
		}
		return updated, nil
	}

	return RecurringPattern{}, ErrRecurringPatternNotFound
}

func (s *Store) DeleteRecurringPattern(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, pattern := range s.recurringPatterns {
		if pattern.ID != id {
			continue
		}
		s.recurringPatterns = append(s.recurringPatterns[:i], s.recurringPatterns[i+1:]...)
		return s.persistLocked()
	}

	return ErrRecurringPatternNotFound
}

func (s *Store) SweepRecurringExpenses(now time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	cutoff := endOfDay(now)

	occurrences := make(map[string]struct{}, len(s.expenses))
	for _, expense := range s.expenses {
		if expense.RecurringPatternID == nil || *expense.RecurringPatternID == "" {
			continue
		}
		occurrences[recurrenceOccurrenceKey(*expense.RecurringPatternID, expense.Date)] = struct{}{}
	}

	generated := 0
	changed := false

	for i := range s.recurringPatterns {
		pattern := &s.recurringPatterns[i]
		if !pattern.Active {
			continue
		}

		frequency := normalizeFrequency(pattern.Frequency)
		if frequency == "" {
			continue
		}
		if pattern.Frequency != frequency {
			pattern.Frequency = frequency
			pattern.UpdatedAt = now
			changed = true
		}

		next := pattern.NextRunDate.UTC()
		if next.IsZero() {
			next = pattern.StartDate.UTC()
			if next.IsZero() {
				continue
			}
			pattern.NextRunDate = next
			pattern.UpdatedAt = now
			changed = true
		}

		anchorDay := pattern.StartDate.Day()
		if anchorDay < 1 {
			anchorDay = next.Day()
		}

		for !next.After(cutoff) {
			if pattern.EndDate != nil && next.After(endOfDay(pattern.EndDate.UTC())) {
				break
			}

			occurrenceKey := recurrenceOccurrenceKey(pattern.ID, next)
			if _, exists := occurrences[occurrenceKey]; !exists {
				patternID := pattern.ID
				s.expenses = append(s.expenses, Expense{
					ID:                 newID(),
					Amount:             pattern.Amount,
					Category:           normalizeStoreCategory(pattern.Category),
					Note:               strings.TrimSpace(pattern.Note),
					Date:               next,
					CreatedAt:          now,
					RecurringPatternID: &patternID,
				})
				occurrences[occurrenceKey] = struct{}{}
				generated++
				changed = true
			}

			advanced, ok := advanceRecurringDate(next, frequency, anchorDay)
			if !ok || !advanced.After(next) {
				break
			}
			next = advanced
			if !pattern.NextRunDate.Equal(next) {
				pattern.NextRunDate = next
				pattern.UpdatedAt = now
				changed = true
			}
		}
	}

	if !changed {
		return generated, nil
	}
	if err := s.persistLocked(); err != nil {
		return generated, err
	}
	return generated, nil
}

func (s *Store) persistLocked() error {
	envelope := storeEnvelope{
		Version:           storeDataVersion,
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

func normalizeRecurringFrequency(frequency string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(frequency))
	switch normalized {
	case "weekly", "monthly":
		return normalized, nil
	case "":
		return "", fmt.Errorf("%w: frequency is required", ErrInvalidRecurringPattern)
	default:
		return "", fmt.Errorf("%w: unsupported frequency %q", ErrInvalidRecurringPattern, frequency)
	}
}

func normalizeFrequency(frequency string) string {
	normalized, err := normalizeRecurringFrequency(frequency)
	if err != nil {
		return ""
	}
	return normalized
}

func recurringPatternFromInput(id string, createdAt time.Time, input RecurringPatternInput) (RecurringPattern, error) {
	frequency, err := normalizeRecurringFrequency(input.Frequency)
	if err != nil {
		return RecurringPattern{}, err
	}
	if input.Amount <= 0 {
		return RecurringPattern{}, fmt.Errorf("%w: amount must be greater than zero", ErrInvalidRecurringPattern)
	}

	now := time.Now().UTC()
	startDate := input.StartDate.UTC()
	if startDate.IsZero() {
		startDate = now
	}
	nextRunDate := input.NextRunDate.UTC()
	if nextRunDate.IsZero() {
		nextRunDate = startDate
	}

	var endDate *time.Time
	if input.EndDate != nil {
		end := input.EndDate.UTC()
		if end.Before(startOfDay(startDate)) {
			return RecurringPattern{}, fmt.Errorf("%w: end_date must be on or after start_date", ErrInvalidRecurringPattern)
		}
		endDate = &end
	}
	if nextRunDate.Before(startOfDay(startDate)) {
		return RecurringPattern{}, fmt.Errorf("%w: next_run_date must be on or after start_date", ErrInvalidRecurringPattern)
	}

	return RecurringPattern{
		ID:          id,
		Amount:      input.Amount,
		Category:    normalizeStoreCategory(input.Category),
		Note:        strings.TrimSpace(input.Note),
		Frequency:   frequency,
		StartDate:   startDate,
		NextRunDate: nextRunDate,
		EndDate:     endDate,
		Active:      input.Active,
		CreatedAt:   createdAt,
	}, nil
}

func normalizeLoadedExpense(expense *Expense) {
	expense.Category = normalizeStoreCategory(expense.Category)
	if expense.RecurringPatternID != nil {
		recurringPatternID := strings.TrimSpace(*expense.RecurringPatternID)
		if recurringPatternID == "" {
			expense.RecurringPatternID = nil
		} else {
			expense.RecurringPatternID = &recurringPatternID
		}
	}
	if !expense.Date.IsZero() {
		expense.Date = expense.Date.UTC()
	}
	if !expense.CreatedAt.IsZero() {
		expense.CreatedAt = expense.CreatedAt.UTC()
	}
}

func normalizeLoadedPattern(pattern *RecurringPattern) {
	pattern.Category = normalizeStoreCategory(pattern.Category)
	pattern.Note = strings.TrimSpace(pattern.Note)
	pattern.Frequency = strings.ToLower(strings.TrimSpace(pattern.Frequency))
	if !pattern.StartDate.IsZero() {
		pattern.StartDate = pattern.StartDate.UTC()
	}
	if !pattern.NextRunDate.IsZero() {
		pattern.NextRunDate = pattern.NextRunDate.UTC()
	}
	if pattern.EndDate != nil {
		end := pattern.EndDate.UTC()
		pattern.EndDate = &end
	}
	if !pattern.CreatedAt.IsZero() {
		pattern.CreatedAt = pattern.CreatedAt.UTC()
	}
	if !pattern.UpdatedAt.IsZero() {
		pattern.UpdatedAt = pattern.UpdatedAt.UTC()
	}
}

func emptyStoreEnvelopeJSON() []byte {
	data, err := json.MarshalIndent(storeEnvelope{
		Version:           storeDataVersion,
		Expenses:          []Expense{},
		RecurringPatterns: []RecurringPattern{},
	}, "", "  ")
	if err != nil {
		return []byte("{}\n")
	}
	return append(data, '\n')
}

func recurrenceOccurrenceKey(patternID string, date time.Time) string {
	return patternID + "|" + date.UTC().Format("2006-01-02")
}

func advanceRecurringDate(current time.Time, frequency string, anchorDay int) (time.Time, bool) {
	current = current.UTC()
	switch frequency {
	case "weekly":
		return current.AddDate(0, 0, 7), true
	case "monthly":
		nextMonth := time.Date(current.Year(), current.Month(), 1, current.Hour(), current.Minute(), current.Second(), current.Nanosecond(), time.UTC).AddDate(0, 1, 0)
		if anchorDay < 1 {
			anchorDay = 1
		}
		day := anchorDay
		maxDay := daysInMonth(nextMonth.Year(), nextMonth.Month())
		if day > maxDay {
			day = maxDay
		}
		return time.Date(nextMonth.Year(), nextMonth.Month(), day, current.Hour(), current.Minute(), current.Second(), current.Nanosecond(), time.UTC), true
	default:
		return time.Time{}, false
	}
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
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
