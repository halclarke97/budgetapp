# Design: Budget Limits

## Overview

Add monthly budget limits per category. Users can set a spending limit for each category and see visual warnings when approaching or exceeding the limit.

## User Story

Alex wants to control spending in specific categories. They set a $200/month limit on dining out. When they've spent $180, they see a warning. At $200+, they see an alert.

## Requirements

### Backend

**Data Model:**
```go
type BudgetLimit struct {
    CategoryID string  `json:"category_id"`
    MonthlyLimit float64 `json:"monthly_limit"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

**Endpoints:**
- `GET /api/budget-limits` — List all budget limits
- `PUT /api/budget-limits/:category_id` — Set/update limit for category
- `DELETE /api/budget-limits/:category_id` — Remove limit

**Stats Enhancement:**
- `GET /api/stats` should include budget status per category:
  - `spent`: current month spending
  - `limit`: budget limit (null if none)
  - `percent`: percentage used
  - `status`: "ok" | "warning" (>80%) | "exceeded" (>100%)

### Frontend

**Budget Settings Panel:**
- List categories with current limits
- Input to set/edit limit per category
- Delete button to remove limit

**Dashboard Enhancements:**
- Category cards show progress bar toward limit
- Color coding: green (<80%), yellow (80-100%), red (>100%)
- Warning badge on categories approaching limit

## Acceptance Criteria

- [ ] Budget limits persist in JSON data store
- [ ] Stats endpoint returns budget status per category
- [ ] Frontend displays limit progress on category cards
- [ ] Visual warnings at 80% and 100% thresholds
- [ ] Users can set, update, and remove limits
