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
