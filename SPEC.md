# BudgetApp Specification

> Simple expense tracker with Go backend and React frontend.

---

## User Story

Alex wants to track their daily expenses without the complexity of full budgeting apps. They need:

1. Quick expense entry (amount, category, note)
2. See where money is going (by category)
3. Track spending over time (weekly, monthly views)
4. Simple dashboard showing totals and trends

---

## Architecture

### Backend (Go)

REST API serving expense data.

**Endpoints:**
- `GET /api/expenses` — List expenses (with filters)
- `POST /api/expenses` — Create expense
- `GET /api/expenses/:id` — Get single expense
- `PUT /api/expenses/:id` — Update expense
- `DELETE /api/expenses/:id` — Delete expense
- `GET /api/categories` — List categories
- `GET /api/stats` — Get summary stats

**Data Model:**
```go
type Expense struct {
    ID        string    `json:"id"`
    Amount    float64   `json:"amount"`
    Category  string    `json:"category"`
    Note      string    `json:"note"`
    Date      time.Time `json:"date"`
    CreatedAt time.Time `json:"created_at"`
}

type Category struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
    Color string `json:"color"`
}
```

**Storage:** JSON file (`data/expenses.json`) for simplicity.

### Frontend (React)

Single-page dashboard for expense management.

**Views:**
- Dashboard — totals, recent expenses, category breakdown chart
- Expense List — filterable table with edit/delete
- Add/Edit Form — modal for expense entry

**Tech:** React 18, Vite, Tailwind CSS, Recharts for charts.

---

## Categories

Default categories:
- 🍔 Food & Dining
- 🚗 Transportation
- 🏠 Housing
- 🎮 Entertainment
- 🛒 Shopping
- 💊 Health
- 📚 Education
- 💼 Other

---

## API Examples

```bash
# Add expense
curl -X POST http://localhost:8080/api/expenses \
  -H "Content-Type: application/json" \
  -d '{"amount": 12.50, "category": "food", "note": "Lunch"}'

# List expenses
curl http://localhost:8080/api/expenses?category=food&from=2026-02-01

# Get stats
curl http://localhost:8080/api/stats?period=month
```

---

## Verification

The test harness validates:
1. Backend builds and starts
2. API endpoints respond correctly
3. Frontend builds and loads
4. Add expense → appears in list
5. Stats reflect correct totals

---

*This spec will be expanded by design tasks.*

---

## Release Design: BudgetApp Enhancements

### Release Summary

Release: `v1.1` (Recurring Expenses)

Goal: add first-class recurring expense support without breaking existing expense CRUD and stats behavior.

Included in scope:
- Recurring patterns (`weekly`, `monthly`) that auto-generate expense entries
- CRUD for recurring patterns
- Dashboard section for upcoming recurring expenses
- Editing and deleting recurring patterns

Out of scope for this release:
- Daily or custom interval recurrences
- Notifications/email reminders
- Multi-currency handling

### Functional Requirements

1. Users can create a recurring pattern from the expense entry flow.
2. Recurring patterns support `weekly` and `monthly` frequencies.
3. The backend auto-generates due expenses from active patterns.
4. Dashboard shows upcoming recurring occurrences (next 30 days).
5. Users can edit and deactivate recurring patterns.

### Data Model Changes

Existing `Expense` is extended and a new `RecurringPattern` model is introduced.

```go
type Expense struct {
    ID                 string     `json:"id"`
    Amount             float64    `json:"amount"`
    Category           string     `json:"category"`
    Note               string     `json:"note"`
    Date               time.Time  `json:"date"`
    CreatedAt          time.Time  `json:"created_at"`
    RecurringPatternID *string    `json:"recurring_pattern_id,omitempty"`
}

type RecurringPattern struct {
    ID          string     `json:"id"`
    Amount      float64    `json:"amount"`
    Category    string     `json:"category"`
    Note        string     `json:"note"`
    Frequency   string     `json:"frequency"` // "weekly" | "monthly"
    StartDate   time.Time  `json:"start_date"`
    NextRunDate time.Time  `json:"next_run_date"`
    EndDate     *time.Time `json:"end_date,omitempty"`
    Active      bool       `json:"active"`
    CreatedAt   time.Time  `json:"created_at"`
    UpdatedAt   time.Time  `json:"updated_at"`
}
```

Storage format moves from a bare `[]Expense` array to a versioned envelope:

```json
{
  "version": 2,
  "expenses": [],
  "recurring_patterns": []
}
```

Backward compatibility requirement:
- If file contains legacy `[]Expense`, load as expenses and initialize empty `recurring_patterns`.

### Backend API Additions and Changes

New endpoints:
- `GET /api/recurring-expenses` — list recurring patterns
- `POST /api/recurring-expenses` — create recurring pattern
- `PUT /api/recurring-expenses/:id` — update recurring pattern
- `DELETE /api/recurring-expenses/:id` — deactivate recurring pattern
- `GET /api/recurring-expenses/upcoming?days=30` — list upcoming occurrences

Updated endpoint:
- `POST /api/expenses` accepts optional recurrence payload for create-and-schedule flow:
  - `recurring: { enabled: boolean, frequency: "weekly"|"monthly", end_date?: string }`
  - when enabled, create one normal expense entry plus one recurring pattern tied to it

### Recurrence Engine Rules

Auto-generation behavior:
- Run sweep at server startup and before read endpoints that return expenses/stats/upcoming.
- For each active pattern, generate all missed occurrences where `next_run_date <= today`.
- Advance `next_run_date` by frequency after each generated occurrence.

Idempotency requirements:
- A pattern must not generate duplicate expenses for the same occurrence date.
- Generated expenses include `recurring_pattern_id` for traceability.

Scheduling rules:
- `weekly`: add 7 days.
- `monthly`: keep day-of-month when possible; clamp to month end for shorter months.
- Stop generation once pattern `end_date` is exceeded.

### Frontend Changes

Dashboard additions:
- New "Upcoming recurring" panel showing next occurrences (date, amount, category, note).

Expense modal additions:
- Toggle: "Make this recurring"
- Frequency selector (`weekly` / `monthly`)
- Optional end date

Recurring management:
- New section or modal list for recurring patterns with edit and deactivate actions.
- Editing a recurring pattern updates future generated entries only.

### Release Plan

Phase 1: Data and recurrence domain
- Add models, persistence envelope migration, and store-layer APIs for recurring patterns.
- Implement generation sweep and recurrence date math utilities.

Phase 2: API integration
- Add recurring endpoints and request validation.
- Extend `POST /api/expenses` to optionally create recurring patterns.

Phase 3: Frontend integration
- Add recurring fields to expense flow.
- Add recurring pattern management UI.
- Add upcoming recurring panel on dashboard.

Phase 4: Hardening and verification
- Add regression tests for existing expense CRUD and stats.
- Add recurrence unit/integration tests (weekly, monthly, month-end, idempotency).
- Manual verification with harness and UI smoke test.

### Verification Strategy

Backend tests:
- Recurring pattern CRUD handlers
- Generation sweep correctness
- No duplicate generated expenses across repeated sweeps
- Legacy file migration from `[]Expense` format

Frontend checks:
- Can create recurring expense from modal
- Can edit/deactivate recurring pattern
- Dashboard upcoming section renders API data and empty states correctly

Harness acceptance extension:
- Existing checks still pass
- Add a recurring flow check: create recurring -> simulate due window -> confirm generated expense appears in list/stats

### Risks and Mitigations

- Risk: duplicate generation under repeated requests
  - Mitigation: idempotency guard keyed by `(recurring_pattern_id, date)`
- Risk: date drift for monthly recurrences
  - Mitigation: explicit month-end clamp logic with tests for Feb and 30-day months
- Risk: migration issues on existing data files
  - Mitigation: tolerant loader supporting both old and new JSON shapes

### Rollback Plan

- Keep reader compatible with legacy `[]Expense` format during this release.
- If recurring rollout causes instability, disable recurrence sweep and recurring endpoints behind a feature flag while retaining baseline expense APIs.
