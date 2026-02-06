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
	if len(strings.TrimSpace(string(data))) == 0 {
		s.expenses = []Expense{}
		s.recurringPatterns = []RecurringPattern{}
		return nil
	}

	trimmed := strings.TrimSpace(string(data))
	loadedLegacyFormat := false
	switch trimmed[0] {
	case '[':
		var expenses []Expense
		if err := json.Unmarshal(data, &expenses); err != nil {
			return fmt.Errorf("parse legacy data file: %w", err)
		}
		normalizeExpenses(expenses)
		s.expenses = expenses
		s.recurringPatterns = []RecurringPattern{}
		loadedLegacyFormat = true
	case '{':
		var envelope storeEnvelope
		if err := json.Unmarshal(data, &envelope); err != nil {
			return fmt.Errorf("parse data file envelope: %w", err)
		}
		if envelope.Expenses == nil {
			envelope.Expenses = []Expense{}
		}
		if envelope.RecurringPatterns == nil {
			envelope.RecurringPatterns = []RecurringPattern{}
		}
		normalizeExpenses(envelope.Expenses)
		normalizeRecurringPatterns(envelope.RecurringPatterns)
		s.expenses = envelope.Expenses
		s.recurringPatterns = envelope.RecurringPatterns
	default:
		return fmt.Errorf("parse data file: unsupported JSON format")
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
		endDate = &end
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

func normalizeExpenses(expenses []Expense) {
	for i := range expenses {
		expenses[i].Category = normalizeStoreCategory(expenses[i].Category)
		if expenses[i].RecurringPatternID != nil {
			recurringPatternID := strings.TrimSpace(*expenses[i].RecurringPatternID)
			if recurringPatternID == "" {
				expenses[i].RecurringPatternID = nil
				continue
			}
			expenses[i].RecurringPatternID = &recurringPatternID
		}
	}
}

func normalizeRecurringPatterns(patterns []RecurringPattern) {
	for i := range patterns {
		patterns[i].Category = normalizeStoreCategory(patterns[i].Category)
		patterns[i].Note = strings.TrimSpace(patterns[i].Note)
		patterns[i].Frequency = strings.ToLower(strings.TrimSpace(patterns[i].Frequency))
		if patterns[i].EndDate != nil {
			end := patterns[i].EndDate.UTC()
			patterns[i].EndDate = &end
		}
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
