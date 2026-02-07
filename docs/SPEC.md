# BudgetApp Specification

> Simple expense tracker with Go backend and React frontend.

## Overview

BudgetApp helps users track daily expenses with quick entry, category-based organization, and spending insights.

This document defines the **initial implementation (MVP)** requirements. It is intended to be actionable for separate implementation tasks and excludes stretch features.

## Product Goals

1. Let users record expenses in under 15 seconds.
2. Provide clear spend visibility by category and time period.
3. Support lightweight monthly budgeting to surface over/under-spend.
4. Keep the system simple enough for a single-user first release.

## Scope

### In Scope (MVP)

1. Create, read, update, and delete (CRUD) expenses.
2. Assign each expense to a category.
3. Manage categories (CRUD) with defaults on first start.
4. Monthly budget targets per category.
5. Summary insights for current month and selected date ranges.
6. Go REST API backend.
7. React single-page frontend.

### Out of Scope (MVP)

1. Multi-user accounts and authentication.
2. Bank sync/import.
3. Recurring expense automation.
4. Export/import files.
5. Native mobile apps.

## Core User Flows

1. Add expense: user enters amount, date, category, and optional note; expense appears in list immediately.
2. Edit/delete expense: user can correct mistakes from the expense list.
3. Manage categories: user can add/rename/delete categories and select category color.
4. Set budgets: user defines monthly budget per category.
5. Review insights: user sees totals by month, category breakdown, and budget variance.

## Functional Requirements

### Expense Management

1. Expense fields:
   - `id` (system-generated)
   - `amount` (decimal, positive, required)
   - `date` (ISO date, required)
   - `categoryId` (required)
   - `note` (optional, max 280 chars)
   - `createdAt` / `updatedAt` (system-generated)
2. Amount precision supports two decimal places.
3. Date cannot be in an invalid format.
4. List view supports sorting by date descending by default.
5. List view supports filtering by date range and category.
6. Delete requires confirmation in UI.

### Category Management

1. Category fields:
   - `id` (system-generated)
   - `name` (required, unique case-insensitive)
   - `color` (hex string, required)
   - `isDefault` (boolean)
2. System initializes with default categories (Food, Transport, Housing, Utilities, Health, Entertainment, Other).
3. Deleting a category with existing expenses is blocked unless expenses are reassigned first.

### Budget Management

1. Budget is scoped to category + month (`YYYY-MM`).
2. Budget fields:
   - `id` (system-generated)
   - `categoryId` (required)
   - `month` (required)
   - `amount` (positive, required)
3. Upsert behavior: one budget record per category per month.
4. Variance calculation: `variance = budget - actual`.

### Insights

1. Monthly total spend for selected month.
2. Spend by category for selected period.
3. Budget vs actual per category for selected month.
4. Recent expenses widget showing last 10 records.

## API Requirements (Go Backend)

### General

1. JSON request/response format.
2. Versioned base path: `/api/v1`.
3. Standard error envelope:
   - `error.code`
   - `error.message`
   - `error.details` (optional)
4. Validation failures return HTTP `400`.
5. Not found returns HTTP `404`.
6. Conflict (e.g., duplicate category name) returns HTTP `409`.

### Required Endpoints

1. Expenses:
   - `GET /expenses`
   - `POST /expenses`
   - `GET /expenses/{id}`
   - `PUT /expenses/{id}`
   - `DELETE /expenses/{id}`
2. Categories:
   - `GET /categories`
   - `POST /categories`
   - `PUT /categories/{id}`
   - `DELETE /categories/{id}`
3. Budgets:
   - `GET /budgets?month=YYYY-MM`
   - `PUT /budgets/{categoryId}?month=YYYY-MM` (upsert)
4. Insights:
   - `GET /insights/summary?from=YYYY-MM-DD&to=YYYY-MM-DD`
   - `GET /insights/monthly?month=YYYY-MM`

## Frontend Requirements (React)

### Pages / Views

1. Dashboard:
   - Monthly total spend
   - Category breakdown visualization
   - Budget variance summary
   - Recent expenses list
2. Expenses:
   - Expense list with filters
   - Add/Edit expense form
3. Categories & Budgets:
   - Category management table/form
   - Monthly budget editor

### UX Requirements

1. Form validation with inline error messaging.
2. Loading, empty, and error states for all data views.
3. Optimistic or immediate refresh behavior after successful create/update/delete.
4. Amounts displayed in USD format.
5. Responsive layout for desktop and mobile widths.

## Data & Persistence

1. Persistence layer must survive server restart.
2. Data model must include expenses, categories, and budgets.
3. Timestamps stored in UTC.
4. Decimal-safe handling for monetary amounts (no floating-point rounding bugs in business logic).

## Non-Functional Requirements

1. API p95 response time under 300ms for list/read operations with up to 10,000 expenses.
2. Basic structured logging for API requests and errors.
3. CORS configured for frontend origin in development.
4. Deterministic seed/init behavior for default categories.

## Testing Requirements

1. Backend unit tests for validation and business rules.
2. Backend API integration tests for all required endpoints.
3. Frontend component/integration tests for:
   - Expense form validation
   - Expense list filtering
   - Budget variance rendering
4. End-to-end happy-path scenario:
   - create category
   - set monthly budget
   - add expense
   - verify dashboard summary updates

## Acceptance Criteria (MVP)

1. User can fully CRUD expenses from UI and data persists after restart.
2. User can manage categories with uniqueness and safe-delete constraints enforced.
3. User can define monthly category budgets and view variance.
4. Dashboard reflects accurate totals and category breakdown for selected period.
5. API and frontend validations prevent invalid amount/date/category inputs.
6. Required automated tests pass in CI.
