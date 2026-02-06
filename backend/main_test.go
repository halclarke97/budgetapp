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

	mux := setupTestMux(t)

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

func TestPostExpenseWithRecurringCreatesPattern(t *testing.T) {
	t.Parallel()

	mux := setupTestMux(t)

	payload := map[string]any{
		"amount":   42.75,
		"category": "housing",
		"note":     "Gym membership",
		"date":     time.Now().UTC().Format("2006-01-02"),
		"recurring": map[string]any{
			"enabled":   true,
			"frequency": "weekly",
			"end_date":  time.Now().UTC().AddDate(0, 1, 0).Format("2006-01-02"),
		},
	}

	createdExpense := doJSON[Expense](t, mux, http.MethodPost, "/api/expenses", payload, http.StatusCreated)
	if createdExpense.RecurringPatternID == nil || strings.TrimSpace(*createdExpense.RecurringPatternID) == "" {
		t.Fatalf("expected expense recurring pattern id, got: %+v", createdExpense)
	}

	patterns := doJSON[[]RecurringPattern](t, mux, http.MethodGet, "/api/recurring-expenses", nil, http.StatusOK)
	if len(patterns) != 1 {
		t.Fatalf("expected 1 recurring pattern, got %d", len(patterns))
	}
	if patterns[0].ID != *createdExpense.RecurringPatternID {
		t.Fatalf("expected recurring pattern id %s, got %s", *createdExpense.RecurringPatternID, patterns[0].ID)
	}

	expenses := doJSON[[]Expense](t, mux, http.MethodGet, "/api/expenses", nil, http.StatusOK)
	if len(expenses) != 1 {
		t.Fatalf("expected exactly one initial expense, got %d", len(expenses))
	}
}

func TestRecurringPatternCRUDAndUpcoming(t *testing.T) {
	t.Parallel()

	mux := setupTestMux(t)
	startDate := time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02")

	createPayload := map[string]any{
		"amount":     75,
		"category":   "transportation",
		"note":       "Transit pass",
		"frequency":  "weekly",
		"start_date": startDate,
		"active":     true,
	}
	createdPattern := doJSON[RecurringPattern](t, mux, http.MethodPost, "/api/recurring-expenses", createPayload, http.StatusCreated)
	if createdPattern.ID == "" {
		t.Fatalf("expected recurring pattern id, got %+v", createdPattern)
	}

	allPatterns := doJSON[[]RecurringPattern](t, mux, http.MethodGet, "/api/recurring-expenses", nil, http.StatusOK)
	if len(allPatterns) != 1 {
		t.Fatalf("expected 1 recurring pattern, got %d", len(allPatterns))
	}

	updatePayload := map[string]any{
		"amount":     85,
		"category":   "transportation",
		"note":       "Updated transit pass",
		"frequency":  "weekly",
		"start_date": startDate,
		"active":     true,
	}
	updatedPattern := doJSON[RecurringPattern](t, mux, http.MethodPut, "/api/recurring-expenses/"+createdPattern.ID, updatePayload, http.StatusOK)
	if updatedPattern.Amount != 85 || updatedPattern.Note != "Updated transit pass" {
		t.Fatalf("unexpected recurring update result: %+v", updatedPattern)
	}

	upcoming := doJSON[[]UpcomingOccurrence](t, mux, http.MethodGet, "/api/recurring-expenses/upcoming?days=30", nil, http.StatusOK)
	if len(upcoming) == 0 {
		t.Fatal("expected upcoming recurring occurrences")
	}
	if upcoming[0].RecurringPatternID != createdPattern.ID {
		t.Fatalf("expected upcoming occurrence to reference pattern %s, got %s", createdPattern.ID, upcoming[0].RecurringPatternID)
	}

	doRaw(t, mux, http.MethodDelete, "/api/recurring-expenses/"+createdPattern.ID, nil, http.StatusNoContent)
	patternsAfterDelete := doJSON[[]RecurringPattern](t, mux, http.MethodGet, "/api/recurring-expenses", nil, http.StatusOK)
	if len(patternsAfterDelete) != 1 {
		t.Fatalf("expected 1 recurring pattern after deactivation, got %d", len(patternsAfterDelete))
	}
	if patternsAfterDelete[0].Active {
		t.Fatalf("expected recurring pattern to be inactive after delete, got %+v", patternsAfterDelete[0])
	}
}

func TestRecurringValidationErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		payload         map[string]any
		expectedMessage string
	}{
		{
			name: "missing recurring frequency",
			payload: map[string]any{
				"amount":   10,
				"category": "food",
				"note":     "coffee",
				"date":     "2026-02-01",
				"recurring": map[string]any{
					"enabled": true,
				},
			},
			expectedMessage: "recurring.frequency",
		},
		{
			name: "invalid recurring end date format",
			payload: map[string]any{
				"amount":   10,
				"category": "food",
				"note":     "coffee",
				"date":     "2026-02-01",
				"recurring": map[string]any{
					"enabled":   true,
					"frequency": "weekly",
					"end_date":  "2026-99-99",
				},
			},
			expectedMessage: "recurring.end_date",
		},
		{
			name: "recurring end date before expense date",
			payload: map[string]any{
				"amount":   10,
				"category": "food",
				"note":     "coffee",
				"date":     "2026-02-20",
				"recurring": map[string]any{
					"enabled":   true,
					"frequency": "weekly",
					"end_date":  "2026-02-01",
				},
			},
			expectedMessage: "on or after expense date",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mux := setupTestMux(t)
			respBody := doRaw(t, mux, http.MethodPost, "/api/expenses", tc.payload, http.StatusBadRequest)
			var errResp map[string]string
			if err := json.Unmarshal(respBody, &errResp); err != nil {
				t.Fatalf("decode error response: %v", err)
			}
			if !strings.Contains(errResp["error"], tc.expectedMessage) {
				t.Fatalf("expected error to contain %q, got %q", tc.expectedMessage, errResp["error"])
			}
		})
	}
}

func setupTestMux(t *testing.T) *http.ServeMux {
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
	mux.HandleFunc("/api/recurring-expenses/upcoming", server.handleRecurringUpcoming)
	mux.HandleFunc("/api/recurring-expenses", server.handleRecurringExpenses)
	mux.HandleFunc("/api/recurring-expenses/", server.handleRecurringExpenseByID)
	mux.HandleFunc("/api/categories", server.handleCategories)
	mux.HandleFunc("/api/stats", server.handleStats)
	return mux
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
