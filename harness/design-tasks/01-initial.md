# Implement BudgetApp

Build a simple expense tracker with Go backend and React frontend.

## Overview

Create a full-stack expense tracking application that allows users to:
- Add expenses with amount, category, and notes
- View expenses by category
- See spending statistics

## Backend Requirements

**Go REST API with these endpoints:**
- `GET /api/expenses` — List all expenses
- `POST /api/expenses` — Create expense
- `GET /api/expenses/:id` — Get single expense
- `PUT /api/expenses/:id` — Update expense  
- `DELETE /api/expenses/:id` — Delete expense
- `GET /api/categories` — List categories
- `GET /api/stats` — Get spending stats
- `GET /healthz` — Health check

**Data stored in JSON file.**

## Frontend Requirements

**React + Vite + Tailwind:**
- Expense entry form
- Expense list with filters
- Category breakdown chart
- Stats dashboard

## Acceptance Criteria

- Backend builds and passes tests
- Frontend builds successfully
- API endpoints work correctly
- Full-stack app runs together
