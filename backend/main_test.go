package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExpenseAPIFlow(t *testing.T) {
	t.Parallel()

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
	mux.HandleFunc("/api/recurring-expenses/upcoming", server.handleUpcomingRecurringExpenses)
	mux.HandleFunc("/api/recurring-expenses", server.handleRecurringExpenses)
	mux.HandleFunc("/api/recurring-expenses/", server.handleRecurringExpenseByID)
	mux.HandleFunc("/api/categories", server.handleCategories)
	mux.HandleFunc("/api/stats", server.handleStats)

	postBody := map[string]any{
		"amount":   12.5,
		"category": "food",
		"note":     "Lunch",
		"date":     time.Now().UTC().Format("2006-01-02"),
		"recurring": map[string]any{
			"enabled":   true,
			"frequency": "weekly",
			"end_date":  time.Now().UTC().AddDate(0, 1, 0).Format("2006-01-02"),
		},
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
		"recurring": map[string]any{
			"enabled": false,
		},
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

func TestRecurringExpenseAPIFlow(t *testing.T) {
	t.Parallel()

	server, mux := newTestAPIServer(t)

	now := time.Now().UTC()
	startDate := now.AddDate(0, 0, 1).Format("2006-01-02")
	nextRunDate := now.AddDate(0, 0, 8).Format("2006-01-02")
	postBody := map[string]any{
		"amount":        19.99,
		"category":      "shopping",
		"note":          "Subscription",
		"frequency":     "weekly",
		"start_date":    startDate,
		"next_run_date": nextRunDate,
		"active":        true,
	}
	created := doJSON[RecurringPattern](t, mux, http.MethodPost, "/api/recurring-expenses", postBody, http.StatusCreated)
	if created.ID == "" {
		t.Fatal("expected recurring pattern ID")
	}

	list := doJSON[[]RecurringPattern](t, mux, http.MethodGet, "/api/recurring-expenses", nil, http.StatusOK)
	if len(list) != 1 {
		t.Fatalf("expected 1 recurring pattern, got %d", len(list))
	}

	endDate := now.AddDate(0, 1, 0).Format("2006-01-02")
	updateBody := map[string]any{
		"amount":        24.5,
		"category":      "shopping",
		"note":          "Updated subscription",
		"frequency":     "weekly",
		"start_date":    startDate,
		"next_run_date": nextRunDate,
		"end_date":      endDate,
		"active":        true,
	}
	updated := doJSON[RecurringPattern](t, mux, http.MethodPut, "/api/recurring-expenses/"+created.ID, updateBody, http.StatusOK)
	if updated.Amount != 24.5 || updated.Note != "Updated subscription" {
		t.Fatalf("unexpected recurring update result: %+v", updated)
	}

	upcoming := doJSON[[]UpcomingRecurringOccurrence](t, mux, http.MethodGet, "/api/recurring-expenses/upcoming?days=30", nil, http.StatusOK)
	if len(upcoming) == 0 {
		t.Fatal("expected upcoming recurring occurrences")
	}
	matched := false
	for _, occurrence := range upcoming {
		if occurrence.RecurringPatternID == created.ID {
			matched = true
			break
		}
	}
	if !matched {
		t.Fatalf("expected upcoming occurrences to include pattern %q", created.ID)
	}

	doRaw(t, mux, http.MethodDelete, "/api/recurring-expenses/"+created.ID, nil, http.StatusNoContent)
	pattern, err := server.store.GetRecurringPattern(created.ID)
	if err != nil {
		t.Fatalf("load recurring pattern after delete: %v", err)
	}
	if pattern.Active {
		t.Fatal("expected recurring pattern to be inactive after delete")
	}
}

func TestCreateExpenseWithRecurringPayload(t *testing.T) {
	t.Parallel()

	_, mux := newTestAPIServer(t)

	body := map[string]any{
		"amount":   12.5,
		"category": "food",
		"note":     "Weekly lunch",
		"date":     time.Now().UTC().Format("2006-01-02"),
		"recurring": map[string]any{
			"enabled":   true,
			"frequency": "weekly",
			"end_date":  time.Now().UTC().AddDate(0, 1, 0).Format("2006-01-02"),
		},
	}
	created := doJSON[Expense](t, mux, http.MethodPost, "/api/expenses", body, http.StatusCreated)
	if created.RecurringPatternID == nil || *created.RecurringPatternID == "" {
		t.Fatal("expected created expense to include recurring_pattern_id")
	}

	patterns := doJSON[[]RecurringPattern](t, mux, http.MethodGet, "/api/recurring-expenses", nil, http.StatusOK)
	if len(patterns) != 1 {
		t.Fatalf("expected 1 recurring pattern, got %d", len(patterns))
	}
	if patterns[0].ID != *created.RecurringPatternID {
		t.Fatalf("expected recurring pattern id %q, got %q", *created.RecurringPatternID, patterns[0].ID)
	}

	expenses := doJSON[[]Expense](t, mux, http.MethodGet, "/api/expenses", nil, http.StatusOK)
	if len(expenses) != 1 {
		t.Fatalf("expected 1 expense, got %d", len(expenses))
	}
}

func TestCreateExpenseWithInvalidRecurringPayloadReturnsBadRequest(t *testing.T) {
	t.Parallel()

	_, mux := newTestAPIServer(t)

	body := map[string]any{
		"amount":   22,
		"category": "food",
		"note":     "Invalid recurring",
		"recurring": map[string]any{
			"enabled":   true,
			"frequency": "daily",
		},
	}
	resp := doJSON[map[string]string](t, mux, http.MethodPost, "/api/expenses", body, http.StatusBadRequest)
	if !strings.Contains(resp["error"], "recurring.frequency") {
		t.Fatalf("expected actionable recurring frequency error, got %q", resp["error"])
	}
}

func TestUpcomingRecurringExpensesDaysValidation(t *testing.T) {
	t.Parallel()

	_, mux := newTestAPIServer(t)

	resp := doJSON[map[string]string](t, mux, http.MethodGet, "/api/recurring-expenses/upcoming?days=0", nil, http.StatusBadRequest)
	if !strings.Contains(resp["error"], "days must be a positive integer") {
		t.Fatalf("unexpected validation message: %q", resp["error"])
	}
}

func newTestAPIServer(t *testing.T) (*apiServer, *http.ServeMux) {
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
	mux.HandleFunc("/api/recurring-expenses/upcoming", server.handleUpcomingRecurringExpenses)
	mux.HandleFunc("/api/recurring-expenses", server.handleRecurringExpenses)
	mux.HandleFunc("/api/recurring-expenses/", server.handleRecurringExpenseByID)
	mux.HandleFunc("/api/categories", server.handleCategories)
	mux.HandleFunc("/api/stats", server.handleStats)
	return server, mux
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
