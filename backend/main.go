package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type apiServer struct {
	store      *Store
	categories []Category
}

type expenseRequest struct {
	Amount    float64            `json:"amount"`
	Category  string             `json:"category"`
	Note      string             `json:"note"`
	Date      string             `json:"date"`
	Recurring *recurringSettings `json:"recurring,omitempty"`
}

type recurringSettings struct {
	Enabled   bool   `json:"enabled"`
	Frequency string `json:"frequency"`
	EndDate   string `json:"end_date"`
}

type recurringPatternRequest struct {
	Amount    float64 `json:"amount"`
	Category  string  `json:"category"`
	Note      string  `json:"note"`
	Frequency string  `json:"frequency"`
	StartDate string  `json:"start_date"`
	EndDate   string  `json:"end_date"`
	Active    *bool   `json:"active,omitempty"`
}

type expenseCreatePayload struct {
	Expense   ExpenseInput
	Recurring *RecurringPatternInput
}

func main() {
	dataPath := os.Getenv("DATA_FILE")
	if dataPath == "" {
		dataPath = detectDataPath()
	}

	st, err := NewStore(dataPath)
	if err != nil {
		log.Fatalf("failed to initialize store: %v", err)
	}
	if err := st.RunRecurringSweep(time.Now().UTC()); err != nil {
		log.Fatalf("failed to run recurring sweep: %v", err)
	}

	server := &apiServer{
		store:      st,
		categories: defaultCategories(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/expenses", server.handleExpenses)
	mux.HandleFunc("/api/expenses/", server.handleExpenseByID)
	mux.HandleFunc("/api/recurring-expenses/upcoming", server.handleRecurringUpcoming)
	mux.HandleFunc("/api/recurring-expenses", server.handleRecurringExpenses)
	mux.HandleFunc("/api/recurring-expenses/", server.handleRecurringExpenseByID)
	mux.HandleFunc("/api/categories", server.handleCategories)
	mux.HandleFunc("/api/stats", server.handleStats)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	addr := ":8080"
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}

	log.Printf("budgetapp backend listening on %s", addr)
	if err := http.ListenAndServe(addr, withCORS(loggingMiddleware(mux))); err != nil {
		log.Fatal(err)
	}
}

func (s *apiServer) handleExpenses(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if err := s.store.RunRecurringSweep(time.Now().UTC()); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to refresh recurring expenses")
			return
		}
		filter, err := parseExpenseFilter(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		expenses := s.store.List(filter)
		writeJSON(w, http.StatusOK, expenses)
	case http.MethodPost:
		input, err := decodeExpenseRequest(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		if input.Recurring != nil {
			expense, _, err := s.store.CreateWithRecurring(input.Expense, *input.Recurring)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to create recurring expense")
				return
			}
			writeJSON(w, http.StatusCreated, expense)
			return
		}

		expense, err := s.store.Create(input.Expense)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create expense")
			return
		}
		writeJSON(w, http.StatusCreated, expense)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *apiServer) handleExpenseByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/expenses/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, "expense not found")
		return
	}

	switch r.Method {
	case http.MethodGet:
		if err := s.store.RunRecurringSweep(time.Now().UTC()); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to refresh recurring expenses")
			return
		}
		expense, err := s.store.Get(id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				writeError(w, http.StatusNotFound, "expense not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to load expense")
			return
		}
		writeJSON(w, http.StatusOK, expense)
	case http.MethodPut:
		input, err := decodeExpenseRequest(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if input.Recurring != nil {
			writeError(w, http.StatusBadRequest, "recurring payload is only supported on POST /api/expenses")
			return
		}
		expense, err := s.store.Update(id, input.Expense)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				writeError(w, http.StatusNotFound, "expense not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to update expense")
			return
		}
		writeJSON(w, http.StatusOK, expense)
	case http.MethodDelete:
		err := s.store.Delete(id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				writeError(w, http.StatusNotFound, "expense not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to delete expense")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *apiServer) handleRecurringExpenses(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		patterns := s.store.ListRecurring()
		writeJSON(w, http.StatusOK, patterns)
	case http.MethodPost:
		input, err := decodeRecurringPatternRequest(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		pattern, err := s.store.CreateRecurring(input)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create recurring pattern")
			return
		}
		writeJSON(w, http.StatusCreated, pattern)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *apiServer) handleRecurringExpenseByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/recurring-expenses/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, "recurring pattern not found")
		return
	}

	switch r.Method {
	case http.MethodPut:
		input, err := decodeRecurringPatternRequest(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		pattern, err := s.store.UpdateRecurring(id, input)
		if err != nil {
			if errors.Is(err, ErrRecurringNotFound) {
				writeError(w, http.StatusNotFound, "recurring pattern not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to update recurring pattern")
			return
		}
		writeJSON(w, http.StatusOK, pattern)
	case http.MethodDelete:
		err := s.store.DeactivateRecurring(id)
		if err != nil {
			if errors.Is(err, ErrRecurringNotFound) {
				writeError(w, http.StatusNotFound, "recurring pattern not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to deactivate recurring pattern")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *apiServer) handleRecurringUpcoming(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if err := s.store.RunRecurringSweep(time.Now().UTC()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to refresh recurring expenses")
		return
	}

	days := 30
	if rawDays := strings.TrimSpace(r.URL.Query().Get("days")); rawDays != "" {
		parsedDays, err := strconv.Atoi(rawDays)
		if err != nil || parsedDays <= 0 {
			writeError(w, http.StatusBadRequest, "days must be a positive integer")
			return
		}
		if parsedDays > 365 {
			writeError(w, http.StatusBadRequest, "days must be less than or equal to 365")
			return
		}
		days = parsedDays
	}

	occurrences := s.store.UpcomingOccurrences(days, time.Now().UTC())
	writeJSON(w, http.StatusOK, occurrences)
}

func (s *apiServer) handleCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, s.categories)
}

func (s *apiServer) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if err := s.store.RunRecurringSweep(time.Now().UTC()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to refresh recurring expenses")
		return
	}

	period := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("period")))
	if period == "" {
		period = "month"
	}
	if period != "week" && period != "month" {
		writeError(w, http.StatusBadRequest, "period must be 'week' or 'month'")
		return
	}
	stats := s.store.Stats(period, time.Now().UTC())
	writeJSON(w, http.StatusOK, stats)
}

func parseExpenseFilter(r *http.Request) (ExpenseFilter, error) {
	query := r.URL.Query()
	filter := ExpenseFilter{Category: normalizeCategory(query.Get("category"))}

	if fromRaw := strings.TrimSpace(query.Get("from")); fromRaw != "" {
		from, err := parseDateValue(fromRaw)
		if err != nil {
			return ExpenseFilter{}, errors.New("invalid from date")
		}
		filter.From = &from
	}
	if toRaw := strings.TrimSpace(query.Get("to")); toRaw != "" {
		to, err := parseDateValue(toRaw)
		if err != nil {
			return ExpenseFilter{}, errors.New("invalid to date")
		}
		filter.To = &to
	}
	return filter, nil
}

func decodeExpenseRequest(r *http.Request) (expenseCreatePayload, error) {
	defer r.Body.Close()

	var req expenseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return expenseCreatePayload{}, errors.New("invalid JSON payload")
	}
	if req.Amount <= 0 {
		return expenseCreatePayload{}, errors.New("amount must be greater than zero")
	}

	date := time.Now().UTC()
	if strings.TrimSpace(req.Date) != "" {
		parsed, err := parseDateValue(req.Date)
		if err != nil {
			return expenseCreatePayload{}, errors.New("invalid date")
		}
		date = parsed
	}

	expense := ExpenseInput{
		Amount:   req.Amount,
		Category: normalizeCategory(req.Category),
		Note:     strings.TrimSpace(req.Note),
		Date:     date,
	}

	if req.Recurring == nil || !req.Recurring.Enabled {
		return expenseCreatePayload{Expense: expense}, nil
	}

	frequency := normalizeFrequency(req.Recurring.Frequency)
	if !isSupportedFrequency(frequency) {
		return expenseCreatePayload{}, errors.New("recurring.frequency must be 'weekly' or 'monthly' when recurring.enabled is true")
	}

	var endDate *time.Time
	if strings.TrimSpace(req.Recurring.EndDate) != "" {
		parsedEndDate, err := parseDateValue(req.Recurring.EndDate)
		if err != nil {
			return expenseCreatePayload{}, errors.New("recurring.end_date must be a valid date")
		}
		if parsedEndDate.Before(startOfDay(date)) {
			return expenseCreatePayload{}, errors.New("recurring.end_date must be on or after expense date")
		}
		endDate = &parsedEndDate
	}

	return expenseCreatePayload{
		Expense: expense,
		Recurring: &RecurringPatternInput{
			Amount:    expense.Amount,
			Category:  expense.Category,
			Note:      expense.Note,
			Frequency: frequency,
			StartDate: expense.Date,
			EndDate:   endDate,
			Active:    true,
		},
	}, nil
}

func decodeRecurringPatternRequest(r *http.Request) (RecurringPatternInput, error) {
	defer r.Body.Close()

	var req recurringPatternRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return RecurringPatternInput{}, errors.New("invalid JSON payload")
	}
	if req.Amount <= 0 {
		return RecurringPatternInput{}, errors.New("amount must be greater than zero")
	}

	frequency := normalizeFrequency(req.Frequency)
	if !isSupportedFrequency(frequency) {
		return RecurringPatternInput{}, errors.New("frequency must be 'weekly' or 'monthly'")
	}

	if strings.TrimSpace(req.StartDate) == "" {
		return RecurringPatternInput{}, errors.New("start_date is required")
	}
	startDate, err := parseDateValue(req.StartDate)
	if err != nil {
		return RecurringPatternInput{}, errors.New("start_date must be a valid date")
	}

	var endDate *time.Time
	if strings.TrimSpace(req.EndDate) != "" {
		parsedEndDate, err := parseDateValue(req.EndDate)
		if err != nil {
			return RecurringPatternInput{}, errors.New("end_date must be a valid date")
		}
		if parsedEndDate.Before(startOfDay(startDate)) {
			return RecurringPatternInput{}, errors.New("end_date must be on or after start_date")
		}
		endDate = &parsedEndDate
	}

	active := true
	if req.Active != nil {
		active = *req.Active
	}

	return RecurringPatternInput{
		Amount:    req.Amount,
		Category:  normalizeCategory(req.Category),
		Note:      strings.TrimSpace(req.Note),
		Frequency: frequency,
		StartDate: startDate,
		EndDate:   endDate,
		Active:    active,
	}, nil
}

func defaultCategories() []Category {
	return []Category{
		{ID: "food", Name: "🍔 Food & Dining", Color: "#F97316"},
		{ID: "transportation", Name: "🚗 Transportation", Color: "#0EA5E9"},
		{ID: "housing", Name: "🏠 Housing", Color: "#22C55E"},
		{ID: "entertainment", Name: "🎮 Entertainment", Color: "#EC4899"},
		{ID: "shopping", Name: "🛒 Shopping", Color: "#8B5CF6"},
		{ID: "health", Name: "💊 Health", Color: "#EF4444"},
		{ID: "education", Name: "📚 Education", Color: "#FACC15"},
		{ID: "other", Name: "💼 Other", Color: "#64748B"},
	}
}

func normalizeCategory(category string) string {
	cat := strings.ToLower(strings.TrimSpace(category))
	if cat == "" {
		return ""
	}
	return cat
}

func parseDateValue(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	formats := []string{time.RFC3339, "2006-01-02", "2006-01-02T15:04:05"}
	for _, format := range formats {
		if parsed, err := time.Parse(format, raw); err == nil {
			if format == "2006-01-02" {
				return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 12, 0, 0, 0, time.UTC), nil
			}
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, errors.New("invalid date")
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func detectDataPath() string {
	candidates := []string{"data/expenses.json", filepath.Join("backend", "data", "expenses.json")}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return candidates[0]
}
