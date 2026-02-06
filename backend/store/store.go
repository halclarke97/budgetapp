package store

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

	"budgetapp/backend/models"
)

var ErrNotFound = errors.New("expense not found")

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

type Store struct {
	mu       sync.RWMutex
	filePath string
	expenses []models.Expense
}

func New(path string) (*Store, error) {
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
		s.expenses = []models.Expense{}
		return nil
	}

	var expenses []models.Expense
	if err := json.Unmarshal(data, &expenses); err != nil {
		return fmt.Errorf("parse data file: %w", err)
	}
	for i := range expenses {
		expenses[i].Category = normalizeCategory(expenses[i].Category)
	}
	s.expenses = expenses
	return nil
}

func (s *Store) List(filter ExpenseFilter) []models.Expense {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]models.Expense, 0, len(s.expenses))
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

func (s *Store) Get(id string) (models.Expense, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, expense := range s.expenses {
		if expense.ID == id {
			return expense, nil
		}
	}
	return models.Expense{}, ErrNotFound
}

func (s *Store) Create(input ExpenseInput) (models.Expense, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	expense := models.Expense{
		ID:        newID(),
		Amount:    input.Amount,
		Category:  normalizeCategory(input.Category),
		Note:      strings.TrimSpace(input.Note),
		Date:      input.Date.UTC(),
		CreatedAt: now,
	}
	if expense.Date.IsZero() {
		expense.Date = now
	}

	s.expenses = append(s.expenses, expense)
	if err := s.persistLocked(); err != nil {
		return models.Expense{}, err
	}
	return expense, nil
}

func (s *Store) Update(id string, input ExpenseInput) (models.Expense, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, expense := range s.expenses {
		if expense.ID != id {
			continue
		}
		expense.Amount = input.Amount
		expense.Category = normalizeCategory(input.Category)
		expense.Note = strings.TrimSpace(input.Note)
		if !input.Date.IsZero() {
			expense.Date = input.Date.UTC()
		}
		s.expenses[i] = expense
		if err := s.persistLocked(); err != nil {
			return models.Expense{}, err
		}
		return expense, nil
	}
	return models.Expense{}, ErrNotFound
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

func (s *Store) Stats(period string, now time.Time) models.Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := models.Stats{
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

	stats.ByCategory = make([]models.CategoryTotal, 0, len(categoryTotals))
	for category, total := range categoryTotals {
		stats.ByCategory = append(stats.ByCategory, models.CategoryTotal{Category: category, Total: total})
	}
	sort.Slice(stats.ByCategory, func(i, j int) bool {
		return stats.ByCategory[i].Total > stats.ByCategory[j].Total
	})

	dates := make([]string, 0, len(trendTotals))
	for day := range trendTotals {
		dates = append(dates, day)
	}
	sort.Strings(dates)
	stats.Trend = make([]models.DailyTotal, 0, len(dates))
	for _, day := range dates {
		stats.Trend = append(stats.Trend, models.DailyTotal{Date: day, Total: trendTotals[day]})
	}

	return stats
}

func (s *Store) persistLocked() error {
	data, err := json.MarshalIndent(s.expenses, "", "  ")
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

func normalizeCategory(category string) string {
	cat := strings.ToLower(strings.TrimSpace(category))
	if cat == "" {
		return "other"
	}
	return cat
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
