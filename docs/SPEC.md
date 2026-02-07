# BudgetApp Specification

> Simple expense tracker with Go backend and React frontend.

## 1. Overview

BudgetApp is a lightweight personal expense tracker focused on fast entry, clear category grouping, and simple monthly spending insights.

## 2. Scope

### 2.1 In Scope (Initial Implementation)
- Local single-user expense tracking.
- Expense CRUD operations.
- Category management (list/create/update/delete).
- Basic spending statistics (monthly total + category breakdown).
- File-based persistence with JSON.
- React single-page UI for all core flows.

### 2.2 Out of Scope (Initial Implementation)
- Authentication and multi-user support.
- Cloud sync or external database.
- Recurring expense automation.
- Budget goals/alerts.

## 3. System Architecture

- Backend: Go HTTP REST API.
- Frontend: React + Vite + Tailwind CSS single-page app.
- Storage: Local JSON file managed by backend.
- Communication: Frontend consumes backend REST JSON endpoints.

## 4. Backend Requirements (Go)

### 4.1 Data Models

#### Category
- `id` (integer, server-generated, unique)
- `name` (string, required, 1-40 chars, case-insensitive unique)
- `color` (string, optional, hex color format `#RRGGBB`)
- `created_at` (RFC3339 timestamp, server-generated)
- `updated_at` (RFC3339 timestamp, server-generated)

#### Expense
- `id` (integer, server-generated, unique)
- `title` (string, required, 1-120 chars)
- `amount` (number, required, > 0, stored with 2 decimal precision)
- `category_id` (integer, required, must reference existing category)
- `date` (string, required, ISO date `YYYY-MM-DD`)
- `notes` (string, optional, max 500 chars)
- `created_at` (RFC3339 timestamp, server-generated)
- `updated_at` (RFC3339 timestamp, server-generated)

### 4.2 API Contract

All endpoints return JSON and use `Content-Type: application/json`.

#### Health
- `GET /healthz`
- Response `200`: `{ "status": "ok" }`

#### Categories
- `GET /api/categories`
  - Returns categories sorted by `name` ascending.
- `POST /api/categories`
  - Request body: `{ "name": string, "color"?: string }`
  - Response `201` with created category.
- `PUT /api/categories/:id`
  - Updates category name/color.
  - Response `200` with updated category.
- `DELETE /api/categories/:id`
  - Deletes category only if no expenses reference it.
  - Response `204` on success.
  - Response `409` if category has related expenses.

#### Expenses
- `GET /api/expenses`
  - Supports optional query params:
    - `from` (`YYYY-MM-DD`)
    - `to` (`YYYY-MM-DD`)
    - `category_id` (integer)
  - Returns expenses sorted by `date` desc, then `id` desc.
- `POST /api/expenses`
  - Request body: `{ "title": string, "amount": number, "category_id": number, "date": "YYYY-MM-DD", "notes"?: string }`
  - Response `201` with created expense.
- `PUT /api/expenses/:id`
  - Updates any mutable expense fields.
  - Response `200` with updated expense.
- `DELETE /api/expenses/:id`
  - Response `204` on success.

#### Stats
- `GET /api/stats/monthly?month=YYYY-MM`
  - Response `200`:
    - `month`: requested month
    - `total_amount`: total spending for month
    - `by_category`: array of `{ category_id, category_name, total_amount }`, sorted by amount desc
    - `expense_count`: number of expenses in month

### 4.3 Validation and Error Handling

- Invalid payload/params return `400`.
- Unknown resource ids return `404`.
- Conflicts (e.g., deleting in-use category, duplicate category name) return `409`.
- Unexpected failures return `500`.
- Error response shape:
  - `{ "error": { "code": string, "message": string } }`

### 4.4 JSON Storage Requirements

- Storage file path is configurable via environment variable, with a local default (`./data/budgetapp.json`).
- Data file contains:
  - `categories` array
  - `expenses` array
  - `next_category_id` integer
  - `next_expense_id` integer
- Backend loads data at startup.
- Writes are atomic (write temp file then rename).
- Create data directory/file if missing.
- If file is malformed JSON, startup fails with clear log error.

### 4.5 Operational Requirements

- Backend runs on configurable port (default `8080`).
- CORS enabled for frontend dev origin.
- Request logging for method, path, status, duration.

## 5. Frontend Requirements (React + Vite + Tailwind)

### 5.1 User Experience

- Main page presents:
  - Expense entry form
  - Expense list with filters
  - Monthly summary cards/chart-like breakdown
  - Category manager section
- Primary workflows:
  - Add expense in under 3 interactions.
  - Filter expenses by date range and category.
  - Create/edit/delete categories.
  - Edit/delete existing expenses.

### 5.2 Component Structure

- `AppShell` (page layout + shared header)
- `ExpenseForm` (create/edit expense)
- `ExpenseFilters` (date range + category filter)
- `ExpenseTable` or `ExpenseList` (list rows + actions)
- `CategoryManager` (category CRUD UI)
- `MonthlyStatsPanel` (monthly total + category breakdown)
- Shared primitives: buttons, inputs, select, modal/confirm dialog, inline alerts

### 5.3 API Integration

- Frontend consumes backend endpoints in section 4.2.
- Central API client module handles:
  - Base URL configuration
  - JSON parse/serialization
  - Uniform error mapping
- UI refresh behavior:
  - After create/update/delete mutations, refresh impacted data views.

### 5.4 State and Interaction Requirements

- Keep local UI state for forms and filters.
- Persist no data in browser local storage for initial release.
- Show loading indicators during async requests.
- Show actionable error messages on failed requests.
- Confirm destructive actions (delete expense/category).

### 5.5 Accessibility and Responsiveness

- Mobile-first responsive layout (usable at 360px width and above).
- Form controls have labels and visible focus states.
- Color is not the sole indicator for category identification.

## 6. Non-Functional Requirements

- Performance target: standard list views should remain responsive for at least 1,000 stored expenses.
- Code should be organized into small modules for maintainability.
- Backend and frontend should support local development with a single command each.

## 7. Verification Requirements

Implementation is complete when:
- All required endpoints are implemented and manually verifiable.
- Frontend supports all specified CRUD/filter/stats workflows.
- Data persists across backend restarts using JSON storage.
- Error and loading states are visible and usable in UI.
- Health endpoint returns success while server is running.
