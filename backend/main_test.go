package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExpenseAPIFlow(t *testing.T) {
	t.Parallel()

	mux, _ := newTestAPIHandler(t)

	postBody := map[string]any{
		"amount":   12.5,
		"category": "food",
		"note":     "Lunch",
		"date":     time.Now().UTC().Format("2006-01-02"),
	}
	created := doJSON[Expense](t, mux, http.MethodPost, "/api/expenses", postBody, http.StatusCreated)
	if created.ID == "" {
		t.Fatal("expected created expense to have an id")
	}

	list := doJSON[[]Expense](t, mux, http.MethodGet, "/api/expenses", nil, http.StatusOK)
	if len(list) != 1 {
		t.Fatalf("expected 1 expense, got %d", len(list))
	}

	_ = doJSON[[]Category](t, mux, http.MethodGet, "/api/categories", nil, http.StatusOK)
	stats := doJSON[Stats](t, mux, http.MethodGet, "/api/stats?period=month", nil, http.StatusOK)
	if stats.TotalExpenses != 1 {
		t.Fatalf("expected total expenses 1, got %d", stats.TotalExpenses)
	}
	if stats.PeriodTotal != 12.5 {
		t.Fatalf("expected period total 12.5, got %v", stats.PeriodTotal)
	}

	updateBody := map[string]any{
		"amount":   20,
		"category": "food",
		"note":     "Dinner",
		"date":     time.Now().UTC().Format("2006-01-02"),
	}
	updated := doJSON[Expense](t, mux, http.MethodPut, "/api/expenses/"+created.ID, updateBody, http.StatusOK)
	if updated.Note != "Dinner" || updated.Amount != 20 {
		t.Fatalf("unexpected update result: %+v", updated)
	}

	doRaw(t, mux, http.MethodDelete, "/api/expenses/"+created.ID, nil, http.StatusNoContent)
	listAfterDelete := doJSON[[]Expense](t, mux, http.MethodGet, "/api/expenses", nil, http.StatusOK)
	if len(listAfterDelete) != 0 {
		t.Fatalf("expected no expenses after delete, got %d", len(listAfterDelete))
	}
}

func TestRecurringFlowAPIRegression(t *testing.T) {
	t.Parallel()

	mux, _ := newTestAPIHandler(t)

	startDate := time.Now().UTC().AddDate(0, 0, -14).Format("2006-01-02")
	patternReq := map[string]any{
		"amount":     9.25,
		"category":   "housing",
		"note":       "Recurring regression check",
		"frequency":  "weekly",
		"start_date": startDate,
	}
	pattern := doJSON[RecurringPattern](t, mux, http.MethodPost, "/api/recurring-expenses", patternReq, http.StatusCreated)
	if pattern.ID == "" {
		t.Fatal("expected recurring pattern id")
	}

	expenses := doJSON[[]Expense](t, mux, http.MethodGet, "/api/expenses", nil, http.StatusOK)
	firstCount := countExpensesForPattern(expenses, pattern.ID)
	if firstCount < 1 {
		t.Fatalf("expected generated recurring expenses, got %d", firstCount)
	}

	expenses = doJSON[[]Expense](t, mux, http.MethodGet, "/api/expenses", nil, http.StatusOK)
	secondCount := countExpensesForPattern(expenses, pattern.ID)
	if secondCount != firstCount {
		t.Fatalf("expected idempotent generation count %d, got %d", firstCount, secondCount)
	}

	stats := doJSON[Stats](t, mux, http.MethodGet, "/api/stats?period=month", nil, http.StatusOK)
	if stats.TotalExpenses < firstCount {
		t.Fatalf("expected stats to include generated recurring expenses; total=%d generated=%d", stats.TotalExpenses, firstCount)
	}

	upcoming := doJSON[[]UpcomingRecurringOccurrence](t, mux, http.MethodGet, "/api/recurring-expenses/upcoming?days=14", nil, http.StatusOK)
	if len(upcoming) == 0 {
		t.Fatal("expected upcoming recurring occurrences")
	}
}

func newTestAPIHandler(t *testing.T) (http.Handler, *Store) {
	t.Helper()

	tempDir := t.TempDir()
	dataPath := filepath.Join(tempDir, "expenses.json")
	if err := os.WriteFile(dataPath, []byte("[]\n"), 0o644); err != nil {
		t.Fatalf("write temp data file: %v", err)
	}

	st, err := NewStore(dataPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	server := &apiServer{store: st, categories: defaultCategories()}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/expenses", server.handleExpenses)
	mux.HandleFunc("/api/expenses/", server.handleExpenseByID)
	mux.HandleFunc("/api/categories", server.handleCategories)
	mux.HandleFunc("/api/stats", server.handleStats)
	mux.HandleFunc("/api/recurring-expenses/upcoming", server.handleRecurringUpcoming)
	mux.HandleFunc("/api/recurring-expenses", server.handleRecurringExpenses)
	mux.HandleFunc("/api/recurring-expenses/", server.handleRecurringExpenseByID)
	return mux, st
}

func countExpensesForPattern(expenses []Expense, patternID string) int {
	count := 0
	for _, expense := range expenses {
		if expense.RecurringPatternID != nil && *expense.RecurringPatternID == patternID {
			count++
		}
	}
	return count
}

func doRaw(t *testing.T, handler http.Handler, method, path string, payload any, expectedStatus int) []byte {
	t.Helper()

	var body bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			t.Fatalf("encode request: %v", err)
		}
	}

	req, err := http.NewRequest(method, path, &body)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != expectedStatus {
		t.Fatalf("expected status %d, got %d", expectedStatus, recorder.Code)
	}

	respBytes, err := io.ReadAll(recorder.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	return respBytes
}

func doJSON[T any](t *testing.T, handler http.Handler, method, path string, payload any, expectedStatus int) T {
	t.Helper()

	var value T
	respBytes := doRaw(t, handler, method, path, payload, expectedStatus)
	if len(respBytes) == 0 {
		return value
	}

	if err := json.Unmarshal(respBytes, &value); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return value
}
