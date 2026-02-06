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
	Amount    float64                 `json:"amount"`
	Category  string                  `json:"category"`
	Note      string                  `json:"note"`
	Date      string                  `json:"date"`
	Recurring *expenseRecurringConfig `json:"recurring,omitempty"`
}

type expenseRecurringConfig struct {
	Enabled   bool   `json:"enabled"`
	Frequency string `json:"frequency"`
	EndDate   string `json:"end_date"`
}

type recurringPatternRequest struct {
	Amount      float64 `json:"amount"`
	Category    string  `json:"category"`
	Note        string  `json:"note"`
	Frequency   string  `json:"frequency"`
	StartDate   string  `json:"start_date"`
	NextRunDate string  `json:"next_run_date"`
	EndDate     *string `json:"end_date"`
	Active      *bool   `json:"active"`
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
	if _, err := st.SweepRecurringExpenses(time.Now().UTC()); err != nil {
		log.Fatalf("failed to sweep recurring expenses: %v", err)
	}

	server := &apiServer{
		store:      st,
		categories: defaultCategories(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/expenses", server.handleExpenses)
	mux.HandleFunc("/api/expenses/", server.handleExpenseByID)
	mux.HandleFunc("/api/recurring-expenses/upcoming", server.handleUpcomingRecurringExpenses)
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
		if _, err := s.store.SweepRecurringExpenses(time.Now().UTC()); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to sweep recurring expenses")
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
		input, recurring, err := decodeExpenseCreateRequest(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		expense, _, err := s.store.CreateExpenseWithRecurring(input, recurring)
		if err != nil {
			if errors.Is(err, ErrInvalidRecurringPattern) {
				writeError(w, http.StatusBadRequest, recurringPatternErrorMessage(err))
				return
			}
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
		expense, err := s.store.Update(id, input)
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

func (s *apiServer) handleCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, s.categories)
}

func (s *apiServer) handleRecurringExpenses(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		patterns := s.store.ListRecurringPatterns()
		writeJSON(w, http.StatusOK, patterns)
	case http.MethodPost:
		input, err := decodeRecurringPatternRequest(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		pattern, err := s.store.CreateRecurringPattern(input)
		if err != nil {
			if errors.Is(err, ErrInvalidRecurringPattern) {
				writeError(w, http.StatusBadRequest, recurringPatternErrorMessage(err))
				return
			}
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
		pattern, err := s.store.UpdateRecurringPattern(id, input)
		if err != nil {
			if errors.Is(err, ErrRecurringPatternNotFound) {
				writeError(w, http.StatusNotFound, "recurring pattern not found")
				return
			}
			if errors.Is(err, ErrInvalidRecurringPattern) {
				writeError(w, http.StatusBadRequest, recurringPatternErrorMessage(err))
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to update recurring pattern")
			return
		}
		writeJSON(w, http.StatusOK, pattern)
	case http.MethodDelete:
		pattern, err := s.store.GetRecurringPattern(id)
		if err != nil {
			if errors.Is(err, ErrRecurringPatternNotFound) {
				writeError(w, http.StatusNotFound, "recurring pattern not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to load recurring pattern")
			return
		}
		if !pattern.Active {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_, err = s.store.UpdateRecurringPattern(id, RecurringPatternInput{
			Amount:      pattern.Amount,
			Category:    pattern.Category,
			Note:        pattern.Note,
			Frequency:   pattern.Frequency,
			StartDate:   pattern.StartDate,
			NextRunDate: pattern.NextRunDate,
			EndDate:     pattern.EndDate,
			Active:      false,
		})
		if err != nil {
			if errors.Is(err, ErrRecurringPatternNotFound) {
				writeError(w, http.StatusNotFound, "recurring pattern not found")
				return
			}
			if errors.Is(err, ErrInvalidRecurringPattern) {
				writeError(w, http.StatusBadRequest, recurringPatternErrorMessage(err))
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

func (s *apiServer) handleUpcomingRecurringExpenses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if _, err := s.store.SweepRecurringExpenses(time.Now().UTC()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to sweep recurring expenses")
		return
	}

	days, err := parseUpcomingDays(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	upcoming := s.store.UpcomingRecurringOccurrences(days, time.Now().UTC())
	writeJSON(w, http.StatusOK, upcoming)
}

func (s *apiServer) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if _, err := s.store.SweepRecurringExpenses(time.Now().UTC()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to sweep recurring expenses")
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

func parseUpcomingDays(r *http.Request) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("days"))
	if raw == "" {
		return 30, nil
	}
	days, err := strconv.Atoi(raw)
	if err != nil || days < 1 {
		return 0, errors.New("days must be a positive integer")
	}
	return days, nil
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

func decodeExpenseCreateRequest(r *http.Request) (ExpenseInput, *ExpenseRecurringInput, error) {
	defer r.Body.Close()

	var req expenseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return ExpenseInput{}, nil, errors.New("invalid JSON payload")
	}
	if req.Amount <= 0 {
		return ExpenseInput{}, nil, errors.New("amount must be greater than zero")
	}

	date := time.Now().UTC()
	if strings.TrimSpace(req.Date) != "" {
		parsed, err := parseDateValue(req.Date)
		if err != nil {
			return ExpenseInput{}, nil, errors.New("invalid date")
		}
		date = parsed
	}

	recurring, err := decodeExpenseRecurringConfig(req.Recurring, date)
	if err != nil {
		return ExpenseInput{}, nil, err
	}

	return ExpenseInput{
		Amount:   req.Amount,
		Category: normalizeCategory(req.Category),
		Note:     strings.TrimSpace(req.Note),
		Date:     date,
	}, recurring, nil
}

func decodeExpenseRequest(r *http.Request) (ExpenseInput, error) {
	defer r.Body.Close()

	var req expenseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return ExpenseInput{}, errors.New("invalid JSON payload")
	}
	if req.Amount <= 0 {
		return ExpenseInput{}, errors.New("amount must be greater than zero")
	}

	date := time.Now().UTC()
	if strings.TrimSpace(req.Date) != "" {
		parsed, err := parseDateValue(req.Date)
		if err != nil {
			return ExpenseInput{}, errors.New("invalid date")
		}
		date = parsed
	}

	return ExpenseInput{
		Amount:   req.Amount,
		Category: normalizeCategory(req.Category),
		Note:     strings.TrimSpace(req.Note),
		Date:     date,
	}, nil
}

func decodeExpenseRecurringConfig(config *expenseRecurringConfig, expenseDate time.Time) (*ExpenseRecurringInput, error) {
	if config == nil || !config.Enabled {
		return nil, nil
	}

	frequency := strings.ToLower(strings.TrimSpace(config.Frequency))
	if frequency != "weekly" && frequency != "monthly" {
		return nil, errors.New("recurring.frequency must be 'weekly' or 'monthly' when recurring.enabled is true")
	}

	var endDate *time.Time
	if rawEndDate := strings.TrimSpace(config.EndDate); rawEndDate != "" {
		parsed, err := parseDateValue(rawEndDate)
		if err != nil {
			return nil, errors.New("recurring.end_date must be a valid date")
		}
		if parsed.Before(startOfDay(expenseDate)) {
			return nil, errors.New("recurring.end_date must be on or after expense date")
		}
		endDate = &parsed
	}

	return &ExpenseRecurringInput{
		Frequency: frequency,
		EndDate:   endDate,
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

	frequency := strings.ToLower(strings.TrimSpace(req.Frequency))
	if frequency != "weekly" && frequency != "monthly" {
		return RecurringPatternInput{}, errors.New("frequency must be 'weekly' or 'monthly'")
	}

	startDate := time.Now().UTC()
	if raw := strings.TrimSpace(req.StartDate); raw != "" {
		parsed, err := parseDateValue(raw)
		if err != nil {
			return RecurringPatternInput{}, errors.New("start_date must be a valid date")
		}
		startDate = parsed
	}

	nextRunDate := startDate
	if raw := strings.TrimSpace(req.NextRunDate); raw != "" {
		parsed, err := parseDateValue(raw)
		if err != nil {
			return RecurringPatternInput{}, errors.New("next_run_date must be a valid date")
		}
		nextRunDate = parsed
	}

	var endDate *time.Time
	if req.EndDate != nil && strings.TrimSpace(*req.EndDate) != "" {
		parsed, err := parseDateValue(*req.EndDate)
		if err != nil {
			return RecurringPatternInput{}, errors.New("end_date must be a valid date")
		}
		endDate = &parsed
	}

	active := true
	if req.Active != nil {
		active = *req.Active
	}

	return RecurringPatternInput{
		Amount:      req.Amount,
		Category:    normalizeCategory(req.Category),
		Note:        strings.TrimSpace(req.Note),
		Frequency:   frequency,
		StartDate:   startDate,
		NextRunDate: nextRunDate,
		EndDate:     endDate,
		Active:      active,
	}, nil
}

func recurringPatternErrorMessage(err error) string {
	const prefix = "invalid recurring pattern: "
	message := strings.TrimSpace(err.Error())
	return strings.TrimPrefix(message, prefix)
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
