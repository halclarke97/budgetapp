# Add Recurring Expenses

Add support for recurring expense patterns that automatically generate expenses.

## Overview

Users can create recurring expense patterns (weekly, monthly) that automatically generate expense entries when due.

## Backend Requirements

**New data model:**
```go
type RecurringPattern struct {
    ID          string    `json:"id"`
    Amount      float64   `json:"amount"`
    Category    string    `json:"category"`
    Note        string    `json:"note"`
    Frequency   string    `json:"frequency"` // "weekly" or "monthly"
    StartDate   time.Time `json:"start_date"`
    EndDate     *time.Time `json:"end_date,omitempty"`
    NextRunDate time.Time `json:"next_run_date"`
    Active      bool      `json:"active"`
}
```

**New endpoints:**
- `GET /api/recurring-expenses` — List patterns
- `POST /api/recurring-expenses` — Create pattern
- `PUT /api/recurring-expenses/:id` — Update pattern
- `DELETE /api/recurring-expenses/:id` — Delete pattern
- `GET /api/recurring-expenses/upcoming` — Preview upcoming

**Sweep function:** Generate due expenses and advance next_run_date.

## Frontend Requirements

- Recurring expense management UI
- Upcoming expenses preview
- Integration with existing expense views

## Acceptance Criteria

- Recurring patterns CRUD works
- Sweep generates expenses correctly
- Monthly patterns handle month-end edge cases
- Frontend displays recurring expense controls
