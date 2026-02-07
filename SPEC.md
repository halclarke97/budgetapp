# BudgetApp Specification

> Simple expense tracker with Go backend and React frontend.

## Overview

BudgetApp is a single-user expense tracker for fast daily entry, filtering by category, and lightweight spending insights. The system uses a Go REST API with JSON-file persistence and a React + Vite + Tailwind frontend.

## Goals

- Support full CRUD for expenses.
- Show category and total spending insights.
- Keep local development and deployment simple (single backend binary + static frontend build).

## Non-Goals

- Multi-user auth/permissions.
- Database integrations beyond local JSON file storage.
- Advanced budgeting workflows (targets, alerts, forecasting).

## Functional Requirements

- Add an expense with `amount`, `category`, and optional `note`.
- List expenses with optional category filter and deterministic sorting by date (newest first).
- View, update, and delete a single expense.
- Return category catalog for frontend selector/filter usage.
- Return stats including total spend, count, average spend, and per-category totals.
- Expose `GET /healthz` for readiness checks.

## Backend Architecture

### Stack

- Go 1.22+
- Standard `net/http` server and JSON handlers
- In-process store backed by a JSON file and guarded by mutex for concurrent access

### API Contract

- `GET /healthz` -> `{ "status": "ok" }`
- `GET /api/expenses` -> `200` with `[]Expense` (optional `?category=<value>` filter)
- `POST /api/expenses` -> `201` with created `Expense`
- `GET /api/expenses/:id` -> `200` with `Expense`, `404` if missing
- `PUT /api/expenses/:id` -> `200` with updated `Expense`, `404` if missing
- `DELETE /api/expenses/:id` -> `204` on success, `404` if missing
- `GET /api/categories` -> `200` with `[]string`
- `GET /api/stats` -> `200` with `StatsResponse`

Validation rules:
- `amount` must be `> 0`
- `category` is required and normalized to lowercase slugs for consistency
- unknown fields in write payloads return `400`

### Data Model

```go
type Expense struct {
    ID        string    `json:"id"`
    Amount    float64   `json:"amount"`
    Category  string    `json:"category"`
    Note      string    `json:"note,omitempty"`
    Date      time.Time `json:"date"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

type StatsResponse struct {
    TotalAmount     float64            `json:"total_amount"`
    ExpenseCount    int                `json:"expense_count"`
    AverageAmount   float64            `json:"average_amount"`
    CategoryTotals  map[string]float64 `json:"category_totals"`
}

type StoreEnvelope struct {
    Version  int       `json:"version"`
    Expenses []Expense `json:"expenses"`
}
```

### Persistence Model

- Store file path is configurable via env var `BUDGETAPP_DATA_FILE` and defaults to `backend/data/expenses.json`.
- Writes are atomic (`tmp` file + rename).
- JSON envelope format allows forward-compatible migrations.
- Bootstraps empty store if file is missing.

## Frontend Architecture

### Stack

- React 18 + Vite
- Tailwind CSS
- Fetch-based API client to Go backend

### UI Scope

- Expense entry form (amount, category, note, date).
- Expense list with category filter and inline delete/edit triggers.
- Stats cards for totals/count/average.
- Category breakdown chart (bar chart or proportional list) from `/api/stats`.
- Graceful loading, empty, and error states.

### UX Rules

- Optimistic local refresh after successful create/update/delete.
- User-visible validation errors for bad inputs.
- Responsive layout for desktop and mobile.

## Verification Strategy

- Backend:
  - `go test ./...` for store and handler logic.
  - Contract tests for all required endpoints and validation paths.
- Frontend:
  - `npm run build` must succeed.
  - Component/state tests for form validation and list rendering.
- Integration:
  - Run backend and frontend together with frontend pointing to backend API.
  - Manual smoke: create, update, filter, delete, and verify stats/category updates.

## Release Plan

1. Implement backend project scaffolding and persistent store layer.
2. Implement expense CRUD endpoints and input validation.
3. Implement categories + stats endpoints and regression tests.
4. Implement frontend scaffold and shared API client.
5. Implement expense workflows, filtering, and category breakdown UI.
6. Perform end-to-end verification and document run instructions.
