# BudgetApp Specification

> Simple expense tracker with Go backend and React frontend.

## Overview

BudgetApp helps users track daily expenses with quick entry, category-based organization, spending insights, and recurring pattern automation.

## Requirements

### Backend

- Go REST API on port `8080`
- Expense CRUD endpoints:
  - `GET /api/expenses`
  - `POST /api/expenses`
  - `GET /api/expenses/:id`
  - `PUT /api/expenses/:id`
  - `DELETE /api/expenses/:id`
- Supporting endpoints:
  - `GET /api/categories`
  - `GET /api/stats`
  - `GET /healthz`
- Recurring expense endpoints:
  - `GET /api/recurring-expenses`
  - `POST /api/recurring-expenses`
  - `PUT /api/recurring-expenses/:id`
  - `DELETE /api/recurring-expenses/:id`
  - `GET /api/recurring-expenses/upcoming`
- JSON-file persistence using `backend/data/store.json`
- Sweep logic generates due recurring expenses and advances `next_run_date`

### Frontend

- React + Vite + Tailwind application
- Expense entry form with amount, category, date, and note
- Expense list with category and note filters
- Spending stats dashboard
- Category breakdown chart
- Recurring expense management UI
- Upcoming recurring expense preview
