package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Expense struct {
	ID        string    `json:"id"`
	Amount    float64   `json:"amount"`
	Category  string    `json:"category"`
	Note      string    `json:"note"`
	Date      time.Time `json:"date"`
	CreatedAt time.Time `json:"created_at"`
}

type RecurringPattern struct {
	ID          string     `json:"id"`
	Amount      float64    `json:"amount"`
	Category    string     `json:"category"`
	Note        string     `json:"note"`
	Frequency   string     `json:"frequency"`
	StartDate   time.Time  `json:"start_date"`
	EndDate     *time.Time `json:"end_date,omitempty"`
	NextRunDate time.Time  `json:"next_run_date"`
	Active      bool       `json:"active"`
}

type statsResponse struct {
	TotalSpending float64            `json:"total_spending"`
	ExpenseCount  int                `json:"expense_count"`
	ByCategory    map[string]float64 `json:"by_category"`
	MonthlyTotal  float64            `json:"monthly_total"`
	Last30Days    float64            `json:"last_30_days"`
}

type upcomingOccurrence struct {
	PatternID string    `json:"pattern_id"`
	Amount    float64   `json:"amount"`
	Category  string    `json:"category"`
	Note      string    `json:"note"`
	Date      time.Time `json:"date"`
}

type storedData struct {
	Expenses          []Expense          `json:"expenses"`
	RecurringPatterns []RecurringPattern `json:"recurring_patterns"`
	Categories        []string           `json:"categories"`
	NextExpenseID     int                `json:"next_expense_id"`
	NextPatternID     int                `json:"next_pattern_id"`
}

type app struct {
	mu       sync.Mutex
	store    storedData
	dataFile string
}

func main() {
	dataFile := os.Getenv("BUDGETAPP_DATA_FILE")
	if strings.TrimSpace(dataFile) == "" {
		dataFile = filepath.Join("data", "store.json")
	}

	a, err := newApp(dataFile)
	if err != nil {
		log.Fatalf("failed to initialize app: %v", err)
	}

	log.Printf("BudgetApp backend listening on :8080")
	if err := http.ListenAndServe(":8080", a); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func newApp(dataFile string) (*app, error) {
	a := &app{dataFile: dataFile}
	if err := a.load(); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *app) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/healthz":
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	case strings.HasPrefix(r.URL.Path, "/api/expenses"):
		a.handleExpenses(w, r)
		return
	case r.Method == http.MethodGet && r.URL.Path == "/api/categories":
		a.handleGetCategories(w)
		return
	case r.Method == http.MethodGet && r.URL.Path == "/api/stats":
		a.handleGetStats(w)
		return
	case r.URL.Path == "/api/recurring-expenses/upcoming":
		a.handleRecurringUpcoming(w, r)
		return
	case strings.HasPrefix(r.URL.Path, "/api/recurring-expenses"):
		a.handleRecurring(w, r)
		return
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func (a *app) handleExpenses(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/api/expenses" && r.Method == http.MethodGet:
		a.handleListExpenses(w)
	case r.URL.Path == "/api/expenses" && r.Method == http.MethodPost:
		a.handleCreateExpense(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/expenses/"):
		id := strings.TrimPrefix(r.URL.Path, "/api/expenses/")
		if strings.Contains(id, "/") || id == "" {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		switch r.Method {
		case http.MethodGet:
			a.handleGetExpense(w, id)
		case http.MethodPut:
			a.handleUpdateExpense(w, r, id)
		case http.MethodDelete:
			a.handleDeleteExpense(w, id)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func (a *app) handleRecurring(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/api/recurring-expenses" && r.Method == http.MethodGet:
		a.handleListRecurringPatterns(w)
	case r.URL.Path == "/api/recurring-expenses" && r.Method == http.MethodPost:
		a.handleCreateRecurringPattern(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/recurring-expenses/"):
		id := strings.TrimPrefix(r.URL.Path, "/api/recurring-expenses/")
		if strings.Contains(id, "/") || id == "" {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		switch r.Method {
		case http.MethodPut:
			a.handleUpdateRecurringPattern(w, r, id)
		case http.MethodDelete:
			a.handleDeleteRecurringPattern(w, id)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

type expenseInput struct {
	Amount   float64 `json:"amount"`
	Category string  `json:"category"`
	Note     string  `json:"note"`
	Date     string  `json:"date"`
}

func (a *app) handleListExpenses(w http.ResponseWriter) {
	a.mu.Lock()
	if err := a.sweepRecurringLocked(); err != nil {
		a.mu.Unlock()
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	expenses := make([]Expense, len(a.store.Expenses))
	copy(expenses, a.store.Expenses)
	a.mu.Unlock()

	sort.Slice(expenses, func(i, j int) bool {
		return expenses[i].Date.After(expenses[j].Date)
	})

	writeJSON(w, http.StatusOK, expenses)
}

func (a *app) handleCreateExpense(w http.ResponseWriter, r *http.Request) {
	var in expenseInput
	if err := decodeJSON(r, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	in.Category = strings.TrimSpace(in.Category)
	if in.Amount <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "amount must be greater than 0"})
		return
	}
	if in.Category == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "category is required"})
		return
	}

	expenseDate, err := parseInputDate(in.Date)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid date"})
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now().UTC()
	expense := Expense{
		ID:        fmt.Sprintf("exp_%06d", a.store.NextExpenseID),
		Amount:    in.Amount,
		Category:  in.Category,
		Note:      strings.TrimSpace(in.Note),
		Date:      expenseDate,
		CreatedAt: now,
	}

	a.store.NextExpenseID++
	a.store.Expenses = append(a.store.Expenses, expense)
	if err := a.saveLocked(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, expense)
}

func (a *app) handleGetExpense(w http.ResponseWriter, id string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, e := range a.store.Expenses {
		if e.ID == id {
			writeJSON(w, http.StatusOK, e)
			return
		}
	}

	writeJSON(w, http.StatusNotFound, map[string]string{"error": "expense not found"})
}

func (a *app) handleUpdateExpense(w http.ResponseWriter, r *http.Request, id string) {
	var in expenseInput
	if err := decodeJSON(r, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	in.Category = strings.TrimSpace(in.Category)
	if in.Amount <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "amount must be greater than 0"})
		return
	}
	if in.Category == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "category is required"})
		return
	}

	var (
		expenseDate time.Time
		err         error
	)
	if strings.TrimSpace(in.Date) != "" {
		expenseDate, err = parseInputDate(in.Date)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid date"})
			return
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	for i := range a.store.Expenses {
		if a.store.Expenses[i].ID != id {
			continue
		}

		a.store.Expenses[i].Amount = in.Amount
		a.store.Expenses[i].Category = in.Category
		a.store.Expenses[i].Note = strings.TrimSpace(in.Note)
		if !expenseDate.IsZero() {
			a.store.Expenses[i].Date = expenseDate
		}

		if err := a.saveLocked(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, a.store.Expenses[i])
		return
	}

	writeJSON(w, http.StatusNotFound, map[string]string{"error": "expense not found"})
}

func (a *app) handleDeleteExpense(w http.ResponseWriter, id string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for i := range a.store.Expenses {
		if a.store.Expenses[i].ID != id {
			continue
		}

		a.store.Expenses = append(a.store.Expenses[:i], a.store.Expenses[i+1:]...)
		if err := a.saveLocked(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		w.WriteHeader(http.StatusNoContent)
		return
	}

	writeJSON(w, http.StatusNotFound, map[string]string{"error": "expense not found"})
}

func (a *app) handleGetCategories(w http.ResponseWriter) {
	a.mu.Lock()
	defer a.mu.Unlock()

	categories := a.collectCategoriesLocked()
	writeJSON(w, http.StatusOK, categories)
}

func (a *app) handleGetStats(w http.ResponseWriter) {
	a.mu.Lock()
	if err := a.sweepRecurringLocked(); err != nil {
		a.mu.Unlock()
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	last30Cutoff := now.AddDate(0, 0, -30)

	resp := statsResponse{
		ByCategory: make(map[string]float64),
	}

	for _, e := range a.store.Expenses {
		resp.TotalSpending += e.Amount
		resp.ExpenseCount++
		resp.ByCategory[e.Category] += e.Amount
		if !e.Date.Before(monthStart) {
			resp.MonthlyTotal += e.Amount
		}
		if !e.Date.Before(last30Cutoff) {
			resp.Last30Days += e.Amount
		}
	}

	a.mu.Unlock()
	writeJSON(w, http.StatusOK, resp)
}

type recurringPatternInput struct {
	Amount    *float64 `json:"amount"`
	Category  *string  `json:"category"`
	Note      *string  `json:"note"`
	Frequency *string  `json:"frequency"`
	StartDate *string  `json:"start_date"`
	EndDate   *string  `json:"end_date"`
	Active    *bool    `json:"active"`
}

func (a *app) handleListRecurringPatterns(w http.ResponseWriter) {
	a.mu.Lock()
	if err := a.sweepRecurringLocked(); err != nil {
		a.mu.Unlock()
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	patterns := make([]RecurringPattern, len(a.store.RecurringPatterns))
	copy(patterns, a.store.RecurringPatterns)
	a.mu.Unlock()

	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].NextRunDate.Before(patterns[j].NextRunDate)
	})

	writeJSON(w, http.StatusOK, patterns)
}

func (a *app) handleCreateRecurringPattern(w http.ResponseWriter, r *http.Request) {
	var in recurringPatternInput
	if err := decodeJSON(r, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if in.Amount == nil || *in.Amount <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "amount must be greater than 0"})
		return
	}
	if in.Category == nil || strings.TrimSpace(*in.Category) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "category is required"})
		return
	}
	if in.Frequency == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "frequency is required"})
		return
	}
	frequency := normalizeFrequency(*in.Frequency)
	if frequency == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "frequency must be weekly or monthly"})
		return
	}
	if in.StartDate == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "start_date is required"})
		return
	}
	startDate, err := parseInputDate(*in.StartDate)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid start_date"})
		return
	}

	var endDate *time.Time
	if in.EndDate != nil && strings.TrimSpace(*in.EndDate) != "" {
		parsed, err := parseInputDate(*in.EndDate)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid end_date"})
			return
		}
		if parsed.Before(startDate) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "end_date must be on or after start_date"})
			return
		}
		endDate = &parsed
	}

	note := ""
	if in.Note != nil {
		note = strings.TrimSpace(*in.Note)
	}

	active := true
	if in.Active != nil {
		active = *in.Active
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	pattern := RecurringPattern{
		ID:          fmt.Sprintf("rec_%06d", a.store.NextPatternID),
		Amount:      *in.Amount,
		Category:    strings.TrimSpace(*in.Category),
		Note:        note,
		Frequency:   frequency,
		StartDate:   startDate,
		EndDate:     endDate,
		NextRunDate: startDate,
		Active:      active,
	}

	a.store.NextPatternID++
	a.store.RecurringPatterns = append(a.store.RecurringPatterns, pattern)
	if err := a.sweepRecurringLocked(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := a.saveLocked(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, pattern)
}

func (a *app) handleUpdateRecurringPattern(w http.ResponseWriter, r *http.Request, id string) {
	var in recurringPatternInput
	if err := decodeJSON(r, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	for i := range a.store.RecurringPatterns {
		p := &a.store.RecurringPatterns[i]
		if p.ID != id {
			continue
		}

		if in.Amount != nil {
			if *in.Amount <= 0 {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "amount must be greater than 0"})
				return
			}
			p.Amount = *in.Amount
		}
		if in.Category != nil {
			cat := strings.TrimSpace(*in.Category)
			if cat == "" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "category is required"})
				return
			}
			p.Category = cat
		}
		if in.Note != nil {
			p.Note = strings.TrimSpace(*in.Note)
		}
		if in.Frequency != nil {
			freq := normalizeFrequency(*in.Frequency)
			if freq == "" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "frequency must be weekly or monthly"})
				return
			}
			p.Frequency = freq
		}
		if in.StartDate != nil {
			startDate, err := parseInputDate(*in.StartDate)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid start_date"})
				return
			}
			p.StartDate = startDate
		}
		if in.EndDate != nil {
			if strings.TrimSpace(*in.EndDate) == "" {
				p.EndDate = nil
			} else {
				parsedEndDate, err := parseInputDate(*in.EndDate)
				if err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid end_date"})
					return
				}
				p.EndDate = &parsedEndDate
			}
		}
		if in.Active != nil {
			p.Active = *in.Active
		}

		if p.EndDate != nil && p.EndDate.Before(p.StartDate) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "end_date must be on or after start_date"})
			return
		}

		if p.NextRunDate.Before(p.StartDate) {
			p.NextRunDate = p.StartDate
		}

		if err := a.sweepRecurringLocked(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if err := a.saveLocked(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, *p)
		return
	}

	writeJSON(w, http.StatusNotFound, map[string]string{"error": "recurring pattern not found"})
}

func (a *app) handleDeleteRecurringPattern(w http.ResponseWriter, id string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for i := range a.store.RecurringPatterns {
		if a.store.RecurringPatterns[i].ID != id {
			continue
		}

		a.store.RecurringPatterns = append(a.store.RecurringPatterns[:i], a.store.RecurringPatterns[i+1:]...)
		if err := a.saveLocked(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		w.WriteHeader(http.StatusNoContent)
		return
	}

	writeJSON(w, http.StatusNotFound, map[string]string{"error": "recurring pattern not found"})
}

func (a *app) handleRecurringUpcoming(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	days := 30
	if raw := strings.TrimSpace(r.URL.Query().Get("days")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "days must be a positive integer"})
			return
		}
		if parsed > 365 {
			parsed = 365
		}
		days = parsed
	}

	a.mu.Lock()
	if err := a.sweepRecurringLocked(); err != nil {
		a.mu.Unlock()
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	now := time.Now().UTC()
	until := now.AddDate(0, 0, days)
	upcoming := make([]upcomingOccurrence, 0)

	for _, pattern := range a.store.RecurringPatterns {
		if !pattern.Active {
			continue
		}

		next := pattern.NextRunDate
		for i := 0; i < 24; i++ {
			if next.After(until) {
				break
			}
			if pattern.EndDate != nil && next.After(*pattern.EndDate) {
				break
			}
			if !next.Before(now) {
				upcoming = append(upcoming, upcomingOccurrence{
					PatternID: pattern.ID,
					Amount:    pattern.Amount,
					Category:  pattern.Category,
					Note:      pattern.Note,
					Date:      next,
				})
			}
			next = advanceNextRunDate(pattern, next)
		}
	}
	a.mu.Unlock()

	sort.Slice(upcoming, func(i, j int) bool {
		return upcoming[i].Date.Before(upcoming[j].Date)
	})

	writeJSON(w, http.StatusOK, upcoming)
}

func (a *app) sweepRecurringLocked() error {
	now := time.Now().UTC()
	changed := false

	for i := range a.store.RecurringPatterns {
		pattern := &a.store.RecurringPatterns[i]
		if !pattern.Active {
			continue
		}

		for !pattern.NextRunDate.After(now) {
			if pattern.EndDate != nil && pattern.NextRunDate.After(*pattern.EndDate) {
				pattern.Active = false
				changed = true
				break
			}

			expense := Expense{
				ID:        fmt.Sprintf("exp_%06d", a.store.NextExpenseID),
				Amount:    pattern.Amount,
				Category:  pattern.Category,
				Note:      pattern.Note,
				Date:      pattern.NextRunDate,
				CreatedAt: now,
			}
			a.store.NextExpenseID++
			a.store.Expenses = append(a.store.Expenses, expense)
			pattern.NextRunDate = advanceNextRunDate(*pattern, pattern.NextRunDate)
			changed = true

			if pattern.EndDate != nil && pattern.NextRunDate.After(*pattern.EndDate) {
				pattern.Active = false
				changed = true
				break
			}
		}
	}

	if changed {
		if err := a.saveLocked(); err != nil {
			return err
		}
	}

	return nil
}

func advanceNextRunDate(pattern RecurringPattern, current time.Time) time.Time {
	switch pattern.Frequency {
	case "weekly":
		return current.AddDate(0, 0, 7)
	case "monthly":
		return addMonthWithAnchor(current, pattern.StartDate.Day())
	default:
		return current.AddDate(0, 0, 7)
	}
}

func addMonthWithAnchor(current time.Time, anchorDay int) time.Time {
	year, month, _ := current.Date()
	hour, minute, second := current.Clock()
	nano := current.Nanosecond()
	location := current.Location()

	nextMonth := month + 1
	if nextMonth > 12 {
		nextMonth = 1
		year++
	}

	lastDay := daysInMonth(year, nextMonth)
	day := anchorDay
	if day > lastDay {
		day = lastDay
	}

	return time.Date(year, nextMonth, day, hour, minute, second, nano, location)
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func normalizeFrequency(input string) string {
	f := strings.ToLower(strings.TrimSpace(input))
	switch f {
	case "weekly", "monthly":
		return f
	default:
		return ""
	}
}

func parseInputDate(raw string) (time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Now().UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse("2006-01-02", value); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
	}
	return time.Time{}, errors.New("invalid date")
}

func decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid JSON payload: %w", err)
	}
	if decoder.More() {
		return errors.New("invalid JSON payload: extra content")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("failed to write JSON response: %v", err)
	}
}

func (a *app) load() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(a.dataFile), 0o755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}

	content, err := os.ReadFile(a.dataFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("read data file: %w", err)
		}
		a.store = storedData{
			Expenses:          []Expense{},
			RecurringPatterns: []RecurringPattern{},
			Categories:        defaultCategories(),
			NextExpenseID:     1,
			NextPatternID:     1,
		}
		return a.saveLocked()
	}

	if len(strings.TrimSpace(string(content))) == 0 {
		a.store = storedData{
			Expenses:          []Expense{},
			RecurringPatterns: []RecurringPattern{},
			Categories:        defaultCategories(),
			NextExpenseID:     1,
			NextPatternID:     1,
		}
		return a.saveLocked()
	}

	if err := json.Unmarshal(content, &a.store); err != nil {
		return fmt.Errorf("decode data file: %w", err)
	}

	if len(a.store.Categories) == 0 {
		a.store.Categories = defaultCategories()
	}
	if a.store.NextExpenseID < 1 {
		a.store.NextExpenseID = len(a.store.Expenses) + 1
	}
	if a.store.NextPatternID < 1 {
		a.store.NextPatternID = len(a.store.RecurringPatterns) + 1
	}

	return nil
}

func (a *app) saveLocked() error {
	payload, err := json.MarshalIndent(a.store, "", "  ")
	if err != nil {
		return fmt.Errorf("encode data: %w", err)
	}

	tmpPath := a.dataFile + ".tmp"
	if err := os.WriteFile(tmpPath, payload, 0o644); err != nil {
		return fmt.Errorf("write temp data file: %w", err)
	}
	if err := os.Rename(tmpPath, a.dataFile); err != nil {
		return fmt.Errorf("replace data file: %w", err)
	}
	return nil
}

func (a *app) collectCategoriesLocked() []string {
	set := make(map[string]struct{})
	categories := make([]string, 0)

	for _, c := range defaultCategories() {
		if _, ok := set[c]; ok {
			continue
		}
		set[c] = struct{}{}
		categories = append(categories, c)
	}

	for _, c := range a.store.Categories {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if _, ok := set[c]; ok {
			continue
		}
		set[c] = struct{}{}
		categories = append(categories, c)
	}

	for _, e := range a.store.Expenses {
		c := strings.TrimSpace(e.Category)
		if c == "" {
			continue
		}
		if _, ok := set[c]; ok {
			continue
		}
		set[c] = struct{}{}
		categories = append(categories, c)
	}

	for _, p := range a.store.RecurringPatterns {
		c := strings.TrimSpace(p.Category)
		if c == "" {
			continue
		}
		if _, ok := set[c]; ok {
			continue
		}
		set[c] = struct{}{}
		categories = append(categories, c)
	}

	sort.Strings(categories)
	return categories
}

func defaultCategories() []string {
	return []string{
		"food",
		"transport",
		"housing",
		"utilities",
		"entertainment",
		"health",
		"shopping",
		"other",
	}
}
