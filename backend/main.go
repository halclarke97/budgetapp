package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"budgetapp/backend/models"
	"budgetapp/backend/store"
)

type Server struct {
	store *store.Store
}

type expenseRequest struct {
	Amount   float64 `json:"amount"`
	Category string  `json:"category"`
	Note     string  `json:"note"`
	Date     string  `json:"date"`
}

func main() {
	dataPath := detectDataPath()
	expenseStore, err := store.New(dataPath)
	if err != nil {
		log.Fatalf("failed to initialize store: %v", err)
	}

	srv := &Server{store: expenseStore}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/expenses", srv.handleListExpenses)
	mux.HandleFunc("POST /api/expenses", srv.handleCreateExpense)
	mux.HandleFunc("GET /api/expenses/{id}", srv.handleGetExpense)
	mux.HandleFunc("PUT /api/expenses/{id}", srv.handleUpdateExpense)
	mux.HandleFunc("DELETE /api/expenses/{id}", srv.handleDeleteExpense)
	mux.HandleFunc("GET /api/categories", srv.handleListCategories)
	mux.HandleFunc("GET /api/stats", srv.handleStats)

	handler := loggingMiddleware(withCORS(mux))

	log.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", handler); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func (s *Server) handleListExpenses(w http.ResponseWriter, r *http.Request) {
	filter, err := parseExpenseFilter(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	expenses := s.store.List(filter)
	writeJSON(w, http.StatusOK, expenses)
}

func (s *Server) handleCreateExpense(w http.ResponseWriter, r *http.Request) {
	input, err := decodeExpenseRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	expense, err := s.store.Create(input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, expense)
}

func (s *Server) handleGetExpense(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	expense, err := s.store.Get(id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "expense not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, expense)
}

func (s *Server) handleUpdateExpense(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	input, err := decodeExpenseRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	expense, err := s.store.Update(id, input)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "expense not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, expense)
}

func (s *Server) handleDeleteExpense(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := s.store.Delete(id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "expense not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListCategories(w http.ResponseWriter, r *http.Request) {
	categories := defaultCategories()
	writeJSON(w, http.StatusOK, categories)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
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

func parseExpenseFilter(r *http.Request) (store.ExpenseFilter, error) {
	query := r.URL.Query()
	filter := store.ExpenseFilter{Category: normalizeCategory(query.Get("category"))}
	if fromRaw := strings.TrimSpace(query.Get("from")); fromRaw != "" {
		from, err := parseDateValue(fromRaw)
		if err != nil {
			return store.ExpenseFilter{}, errors.New("invalid from date")
		}
		filter.From = &from
	}
	if toRaw := strings.TrimSpace(query.Get("to")); toRaw != "" {
		to, err := parseDateValue(toRaw)
		if err != nil {
			return store.ExpenseFilter{}, errors.New("invalid to date")
		}
		filter.To = &to
	}
	return filter, nil
}

func decodeExpenseRequest(r *http.Request) (store.ExpenseInput, error) {
	defer r.Body.Close()
	var req expenseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return store.ExpenseInput{}, errors.New("invalid JSON payload")
	}
	if req.Amount <= 0 {
		return store.ExpenseInput{}, errors.New("amount must be greater than zero")
	}
	date := time.Now().UTC()
	if strings.TrimSpace(req.Date) != "" {
		parsed, err := parseDateValue(req.Date)
		if err != nil {
			return store.ExpenseInput{}, errors.New("invalid date")
		}
		date = parsed
	}
	return store.ExpenseInput{
		Amount:   req.Amount,
		Category: normalizeCategory(req.Category),
		Note:     strings.TrimSpace(req.Note),
		Date:     date,
	}, nil
}

func defaultCategories() []models.Category {
	return []models.Category{
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
		if _, err := filepath.Abs(candidate); err == nil {
			return candidate
		}
	}
	return candidates[0]
}
