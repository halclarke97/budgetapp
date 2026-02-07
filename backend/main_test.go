package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func newTestApp(t *testing.T) *app {
	t.Helper()
	dataFile := filepath.Join(t.TempDir(), "store.json")
	a, err := newApp(dataFile)
	if err != nil {
		t.Fatalf("newApp() error = %v", err)
	}
	return a
}

func doJSONRequest(t *testing.T, handler http.Handler, method, path string, payload any) *httptest.ResponseRecorder {
	t.Helper()

	var body bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			t.Fatalf("encode payload: %v", err)
		}
	}

	req := httptest.NewRequest(method, path, &body)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	return res
}

func decodeBody[T any](t *testing.T, res *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(res.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return out
}

func TestExpenseCRUDAndStats(t *testing.T) {
	a := newTestApp(t)

	health := doJSONRequest(t, a, http.MethodGet, "/healthz", nil)
	if health.Code != http.StatusOK {
		t.Fatalf("health status = %d, want 200", health.Code)
	}

	create := doJSONRequest(t, a, http.MethodPost, "/api/expenses", map[string]any{
		"amount":   12.5,
		"category": "food",
		"note":     "lunch",
		"date":     "2026-02-01",
	})
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201, body=%s", create.Code, create.Body.String())
	}
	createdExpense := decodeBody[Expense](t, create)
	if createdExpense.ID == "" {
		t.Fatal("created expense ID is empty")
	}

	list := doJSONRequest(t, a, http.MethodGet, "/api/expenses", nil)
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200", list.Code)
	}
	expenses := decodeBody[[]Expense](t, list)
	if len(expenses) != 1 {
		t.Fatalf("expenses count = %d, want 1", len(expenses))
	}

	getOne := doJSONRequest(t, a, http.MethodGet, "/api/expenses/"+createdExpense.ID, nil)
	if getOne.Code != http.StatusOK {
		t.Fatalf("get one status = %d, want 200", getOne.Code)
	}

	update := doJSONRequest(t, a, http.MethodPut, "/api/expenses/"+createdExpense.ID, map[string]any{
		"amount":   20.0,
		"category": "transport",
		"note":     "train",
		"date":     "2026-02-02",
	})
	if update.Code != http.StatusOK {
		t.Fatalf("update status = %d, want 200, body=%s", update.Code, update.Body.String())
	}

	statsRes := doJSONRequest(t, a, http.MethodGet, "/api/stats", nil)
	if statsRes.Code != http.StatusOK {
		t.Fatalf("stats status = %d, want 200", statsRes.Code)
	}
	stats := decodeBody[statsResponse](t, statsRes)
	if stats.ExpenseCount != 1 {
		t.Fatalf("stats expense_count = %d, want 1", stats.ExpenseCount)
	}
	if stats.ByCategory["transport"] != 20.0 {
		t.Fatalf("stats transport total = %f, want 20.0", stats.ByCategory["transport"])
	}

	deleteRes := doJSONRequest(t, a, http.MethodDelete, "/api/expenses/"+createdExpense.ID, nil)
	if deleteRes.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", deleteRes.Code)
	}
}

func TestRecurringSweepAndUpcoming(t *testing.T) {
	a := newTestApp(t)
	today := time.Now().UTC().Format("2006-01-02")
	start := time.Now().UTC().AddDate(0, 0, -14).Format("2006-01-02")

	createPattern := doJSONRequest(t, a, http.MethodPost, "/api/recurring-expenses", map[string]any{
		"amount":     15.0,
		"category":   "utilities",
		"note":       "subscription",
		"frequency":  "weekly",
		"start_date": start,
		"end_date":   today,
		"active":     true,
	})
	if createPattern.Code != http.StatusCreated {
		t.Fatalf("create recurring status = %d, want 201, body=%s", createPattern.Code, createPattern.Body.String())
	}

	listExpenses := doJSONRequest(t, a, http.MethodGet, "/api/expenses", nil)
	if listExpenses.Code != http.StatusOK {
		t.Fatalf("list expenses status = %d, want 200", listExpenses.Code)
	}
	expenses := decodeBody[[]Expense](t, listExpenses)
	if len(expenses) < 2 {
		t.Fatalf("expected at least 2 generated expenses, got %d", len(expenses))
	}

	futurePattern := doJSONRequest(t, a, http.MethodPost, "/api/recurring-expenses", map[string]any{
		"amount":     9.5,
		"category":   "health",
		"note":       "weekly class",
		"frequency":  "weekly",
		"start_date": time.Now().UTC().AddDate(0, 0, 7).Format("2006-01-02"),
		"active":     true,
	})
	if futurePattern.Code != http.StatusCreated {
		t.Fatalf("create future recurring status = %d, want 201", futurePattern.Code)
	}

	upcoming := doJSONRequest(t, a, http.MethodGet, "/api/recurring-expenses/upcoming?days=30", nil)
	if upcoming.Code != http.StatusOK {
		t.Fatalf("upcoming status = %d, want 200", upcoming.Code)
	}
	items := decodeBody[[]upcomingOccurrence](t, upcoming)
	if len(items) == 0 {
		t.Fatal("expected upcoming recurring occurrences, got 0")
	}
}

func TestAddMonthWithAnchorMonthEnd(t *testing.T) {
	current := time.Date(2024, time.January, 31, 0, 0, 0, 0, time.UTC)
	next := addMonthWithAnchor(current, 31)
	if next.Day() != 29 || next.Month() != time.February {
		t.Fatalf("expected Feb 29, got %s", next.Format("2006-01-02"))
	}

	next = addMonthWithAnchor(next, 31)
	if next.Day() != 31 || next.Month() != time.March {
		t.Fatalf("expected Mar 31, got %s", next.Format("2006-01-02"))
	}
}
