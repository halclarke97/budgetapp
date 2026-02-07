# BudgetApp Specification

> Simple expense tracker with Go backend and React frontend.

## Overview

BudgetApp helps users track daily expenses with quick entry, category-based organization, and spending insights.

## Goals

1. Let users record expenses in under 10 seconds per entry.
2. Keep expense data organized by category and date.
3. Provide actionable spending insights (totals and category breakdowns) for a selected time period.

## Scope

This initial implementation covers a single-user local deployment with a Go HTTP API backend and a React SPA frontend.

## User Stories

1. As a user, I can create and manage categories so my expenses are organized.
2. As a user, I can add, edit, and delete expenses so my records stay accurate.
3. As a user, I can filter expenses by date range and category so I can review specific periods.
4. As a user, I can view total spend and category-level totals so I can understand spending patterns.

## Functional Requirements

### Expense Management

1. The system must support creating an expense with:
   - `amount` (positive decimal with 2-digit precision)
   - `date` (ISO date string, `YYYY-MM-DD`)
   - `category_id`
   - `description` (optional, max 200 chars)
2. The system must support listing expenses with filters:
   - `from` date (inclusive)
   - `to` date (inclusive)
   - `category_id`
   - sort by `date desc` by default
3. The system must support updating an existing expense by id.
4. The system must support deleting an expense by id.
5. The system must reject invalid payloads with field-level validation errors.

### Category Management

1. The system must support creating categories with unique names (case-insensitive).
2. The system must support listing all categories sorted alphabetically.
3. The system must support updating category name.
4. The system must support deleting a category only when it is not referenced by any expense.
5. The system must include seed/default categories on first run (Food, Transport, Housing, Utilities, Entertainment, Health, Other).

### Insights

1. The system must provide a summary endpoint for a date range returning:
   - total spend
   - expense count
   - spend grouped by category
2. Insights calculations must be based on persisted expense data and selected filters.

## API Requirements

Base path: `/api/v1`

1. `GET /categories`
2. `POST /categories`
3. `PUT /categories/:id`
4. `DELETE /categories/:id`
5. `GET /expenses`
6. `POST /expenses`
7. `PUT /expenses/:id`
8. `DELETE /expenses/:id`
9. `GET /insights/summary`

Response rules:

1. Success responses return JSON.
2. Validation failures return `400` with structured error details.
3. Not-found resources return `404`.
4. Business-rule conflicts (for example deleting category in use) return `409`.

## Data Requirements

### Entities

1. `Category`
   - `id` (stable identifier)
   - `name` (unique, 1-50 chars)
   - `created_at`
   - `updated_at`
2. `Expense`
   - `id` (stable identifier)
   - `amount` (decimal(12,2) equivalent)
   - `date`
   - `category_id` (foreign key to category)
   - `description` (nullable, max 200 chars)
   - `created_at`
   - `updated_at`

### Validation and Integrity

1. Amount must be greater than `0`.
2. Date must be a valid calendar date.
3. Category foreign key must reference an existing category.
4. Deleting a category in use must be blocked.

## Frontend Requirements

### Application Views

1. Expense List view:
   - displays expenses in reverse-chronological order
   - includes filter controls (date range + category)
2. Add/Edit Expense form:
   - supports create and update flows
   - uses inline validation error display
3. Category Management view:
   - list/create/update/delete categories
4. Insights view:
   - total spend card
   - category breakdown list for selected range

### UX Expectations

1. Primary flows (add expense, filter list) must be usable on desktop and mobile widths.
2. UI must show loading, empty, and error states for each data view.
3. UI must use optimistic disabling/prevent double-submit behavior while requests are in-flight.

## Non-Functional Requirements

1. Time handling:
   - store and compare expense date as date-only (no timezone conversion).
2. Performance:
   - list and summary requests should complete within 500ms for 10,000 expenses on local development hardware.
3. Accessibility:
   - form inputs must have labels
   - keyboard navigation must work for all interactive controls
4. Reliability:
   - backend must return deterministic error formats for client handling.

## Out Of Scope (Initial Release)

1. Multi-user authentication and authorization.
2. Recurring expenses.
3. Budget limits/alerts.
4. Data import/export.
5. Multi-currency support.

## Success Criteria

1. A user can create categories and expenses through the UI with validation feedback.
2. A user can filter expenses by date and category and see matching results.
3. A user can view summary totals and per-category breakdown for a selected range.
4. The implementation tasks in `.cc/tasks/` trace back to this specification.
