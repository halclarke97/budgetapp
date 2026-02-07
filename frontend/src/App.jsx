import { useEffect, useMemo, useState } from 'react'

const API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:8080'

const defaultExpenseForm = {
  amount: '',
  category: 'food',
  note: '',
  date: todayValue()
}

const defaultRecurringForm = {
  amount: '',
  category: 'food',
  note: '',
  frequency: 'monthly',
  start_date: todayValue(),
  end_date: '',
  active: true
}

function todayValue() {
  return new Date().toISOString().slice(0, 10)
}

function formatCurrency(value) {
  return new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: 'USD'
  }).format(Number(value || 0))
}

function formatDate(value) {
  if (!value) {
    return '-'
  }
  return new Date(value).toLocaleDateString('en-US')
}

async function api(path, options = {}) {
  const response = await fetch(`${API_BASE}${path}`, {
    headers: {
      'Content-Type': 'application/json',
      ...(options.headers || {})
    },
    ...options
  })

  if (!response.ok) {
    let message = `Request failed (${response.status})`
    try {
      const body = await response.json()
      if (body.error) {
        message = body.error
      }
    } catch {
      // ignore non-JSON error payloads
    }
    throw new Error(message)
  }

  if (response.status === 204) {
    return null
  }

  return response.json()
}

export default function App() {
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [expenses, setExpenses] = useState([])
  const [categories, setCategories] = useState([])
  const [stats, setStats] = useState({
    total_spending: 0,
    expense_count: 0,
    monthly_total: 0,
    last_30_days: 0,
    by_category: {}
  })
  const [recurringPatterns, setRecurringPatterns] = useState([])
  const [upcomingExpenses, setUpcomingExpenses] = useState([])
  const [expenseForm, setExpenseForm] = useState(defaultExpenseForm)
  const [recurringForm, setRecurringForm] = useState(defaultRecurringForm)
  const [filters, setFilters] = useState({ category: 'all', query: '' })

  const chartRows = useMemo(() => {
    const entries = Object.entries(stats.by_category || {})
    entries.sort((a, b) => b[1] - a[1])
    const maxValue = entries.length === 0 ? 1 : entries[0][1]

    return entries.map(([category, amount]) => ({
      category,
      amount,
      width: `${Math.max(6, (amount / maxValue) * 100)}%`
    }))
  }, [stats])

  const filteredExpenses = useMemo(() => {
    const query = filters.query.trim().toLowerCase()
    return expenses.filter((expense) => {
      if (filters.category !== 'all' && expense.category !== filters.category) {
        return false
      }
      if (!query) {
        return true
      }
      return (expense.note || '').toLowerCase().includes(query)
    })
  }, [expenses, filters])

  async function loadAllData() {
    setLoading(true)
    setError('')

    try {
      const [expenseData, categoryData, statsData, recurringData, upcomingData] =
        await Promise.all([
          api('/api/expenses'),
          api('/api/categories'),
          api('/api/stats'),
          api('/api/recurring-expenses'),
          api('/api/recurring-expenses/upcoming?days=60')
        ])

      setExpenses(expenseData)
      setCategories(categoryData)
      setStats(statsData)
      setRecurringPatterns(recurringData)
      setUpcomingExpenses(upcomingData)

      if (categoryData.length > 0) {
        setExpenseForm((prev) => ({
          ...prev,
          category: categoryData.includes(prev.category)
            ? prev.category
            : categoryData[0]
        }))
        setRecurringForm((prev) => ({
          ...prev,
          category: categoryData.includes(prev.category)
            ? prev.category
            : categoryData[0]
        }))
      }
    } catch (loadError) {
      setError(loadError.message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadAllData()
  }, [])

  async function onExpenseSubmit(event) {
    event.preventDefault()
    setError('')

    try {
      await api('/api/expenses', {
        method: 'POST',
        body: JSON.stringify({
          amount: Number(expenseForm.amount),
          category: expenseForm.category,
          note: expenseForm.note,
          date: expenseForm.date
        })
      })

      setExpenseForm((prev) => ({
        ...prev,
        amount: '',
        note: ''
      }))

      await loadAllData()
    } catch (submitError) {
      setError(submitError.message)
    }
  }

  async function onDeleteExpense(expenseId) {
    setError('')
    try {
      await api(`/api/expenses/${expenseId}`, { method: 'DELETE' })
      await loadAllData()
    } catch (deleteError) {
      setError(deleteError.message)
    }
  }

  async function onRecurringSubmit(event) {
    event.preventDefault()
    setError('')

    try {
      await api('/api/recurring-expenses', {
        method: 'POST',
        body: JSON.stringify({
          amount: Number(recurringForm.amount),
          category: recurringForm.category,
          note: recurringForm.note,
          frequency: recurringForm.frequency,
          start_date: recurringForm.start_date,
          end_date: recurringForm.end_date || null,
          active: recurringForm.active
        })
      })

      setRecurringForm((prev) => ({
        ...prev,
        amount: '',
        note: ''
      }))

      await loadAllData()
    } catch (submitError) {
      setError(submitError.message)
    }
  }

  async function onToggleRecurring(pattern) {
    setError('')

    try {
      await api(`/api/recurring-expenses/${pattern.id}`, {
        method: 'PUT',
        body: JSON.stringify({ active: !pattern.active })
      })
      await loadAllData()
    } catch (toggleError) {
      setError(toggleError.message)
    }
  }

  async function onDeleteRecurring(patternId) {
    setError('')

    try {
      await api(`/api/recurring-expenses/${patternId}`, { method: 'DELETE' })
      await loadAllData()
    } catch (deleteError) {
      setError(deleteError.message)
    }
  }

  return (
    <div className="min-h-screen px-4 py-6 text-ink sm:px-8">
      <main className="mx-auto max-w-6xl space-y-6">
        <header className="rounded-2xl border border-sky/20 bg-white/80 p-6 shadow-sm backdrop-blur">
          <h1 className="text-3xl font-bold tracking-tight">BudgetApp</h1>
          <p className="mt-2 text-sm text-slate-600">
            Track expenses, analyze spending, and automate recurring entries.
          </p>
        </header>

        {error && (
          <p className="rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
            {error}
          </p>
        )}

        <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          <StatCard title="Total Spending" value={formatCurrency(stats.total_spending)} />
          <StatCard title="This Month" value={formatCurrency(stats.monthly_total)} />
          <StatCard title="Last 30 Days" value={formatCurrency(stats.last_30_days)} />
          <StatCard title="Expense Count" value={String(stats.expense_count || 0)} />
        </section>

        <section className="grid gap-6 lg:grid-cols-2">
          <Panel title="Add Expense">
            <form className="space-y-3" onSubmit={onExpenseSubmit}>
              <div className="grid gap-3 sm:grid-cols-2">
                <label className="block text-sm font-medium">
                  Amount
                  <input
                    className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
                    type="number"
                    min="0"
                    step="0.01"
                    required
                    value={expenseForm.amount}
                    onChange={(event) =>
                      setExpenseForm((prev) => ({ ...prev, amount: event.target.value }))
                    }
                  />
                </label>
                <label className="block text-sm font-medium">
                  Date
                  <input
                    className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
                    type="date"
                    value={expenseForm.date}
                    onChange={(event) =>
                      setExpenseForm((prev) => ({ ...prev, date: event.target.value }))
                    }
                  />
                </label>
              </div>

              <label className="block text-sm font-medium">
                Category
                <select
                  className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
                  value={expenseForm.category}
                  onChange={(event) =>
                    setExpenseForm((prev) => ({ ...prev, category: event.target.value }))
                  }
                >
                  {categories.map((category) => (
                    <option key={category} value={category}>
                      {category}
                    </option>
                  ))}
                </select>
              </label>

              <label className="block text-sm font-medium">
                Note
                <input
                  className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
                  type="text"
                  value={expenseForm.note}
                  onChange={(event) =>
                    setExpenseForm((prev) => ({ ...prev, note: event.target.value }))
                  }
                  placeholder="Optional details"
                />
              </label>

              <button
                className="rounded-lg bg-sky px-4 py-2 text-sm font-semibold text-white transition hover:brightness-95"
                type="submit"
              >
                Save Expense
              </button>
            </form>
          </Panel>

          <Panel title="Category Breakdown">
            {chartRows.length === 0 && (
              <p className="text-sm text-slate-500">No spending data yet.</p>
            )}
            <div className="space-y-3">
              {chartRows.map((row) => (
                <div key={row.category} className="space-y-1">
                  <div className="flex items-center justify-between text-sm">
                    <span className="font-medium capitalize">{row.category}</span>
                    <span className="text-slate-600">{formatCurrency(row.amount)}</span>
                  </div>
                  <div className="h-2 rounded-full bg-slate-100">
                    <div
                      className="h-2 rounded-full bg-gradient-to-r from-sky to-mint"
                      style={{ width: row.width }}
                    />
                  </div>
                </div>
              ))}
            </div>
          </Panel>
        </section>

        <section className="grid gap-6 lg:grid-cols-2">
          <Panel title="Expenses">
            <div className="mb-3 grid gap-3 sm:grid-cols-2">
              <select
                className="rounded-lg border border-slate-300 px-3 py-2 text-sm"
                value={filters.category}
                onChange={(event) =>
                  setFilters((prev) => ({ ...prev, category: event.target.value }))
                }
              >
                <option value="all">All categories</option>
                {categories.map((category) => (
                  <option key={category} value={category}>
                    {category}
                  </option>
                ))}
              </select>
              <input
                className="rounded-lg border border-slate-300 px-3 py-2 text-sm"
                type="search"
                value={filters.query}
                onChange={(event) =>
                  setFilters((prev) => ({ ...prev, query: event.target.value }))
                }
                placeholder="Filter by note"
              />
            </div>

            <div className="max-h-[25rem] space-y-2 overflow-y-auto pr-1">
              {!loading && filteredExpenses.length === 0 && (
                <p className="text-sm text-slate-500">No expenses match the current filters.</p>
              )}

              {filteredExpenses.map((expense) => (
                <article
                  key={expense.id}
                  className="rounded-xl border border-slate-200 bg-slate-50 px-3 py-2"
                >
                  <div className="flex items-start justify-between gap-2">
                    <div>
                      <p className="text-sm font-semibold capitalize">{expense.category}</p>
                      <p className="text-xs text-slate-500">{formatDate(expense.date)}</p>
                    </div>
                    <p className="text-sm font-semibold text-slate-900">
                      {formatCurrency(expense.amount)}
                    </p>
                  </div>
                  {expense.note && (
                    <p className="mt-1 text-sm text-slate-700">{expense.note}</p>
                  )}
                  <button
                    className="mt-2 text-xs font-medium text-rose-600 hover:text-rose-700"
                    type="button"
                    onClick={() => onDeleteExpense(expense.id)}
                  >
                    Delete
                  </button>
                </article>
              ))}
            </div>
          </Panel>

          <Panel title="Recurring Expenses">
            <form className="space-y-3" onSubmit={onRecurringSubmit}>
              <div className="grid gap-3 sm:grid-cols-2">
                <label className="block text-sm font-medium">
                  Amount
                  <input
                    className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
                    type="number"
                    min="0"
                    step="0.01"
                    required
                    value={recurringForm.amount}
                    onChange={(event) =>
                      setRecurringForm((prev) => ({ ...prev, amount: event.target.value }))
                    }
                  />
                </label>
                <label className="block text-sm font-medium">
                  Frequency
                  <select
                    className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
                    value={recurringForm.frequency}
                    onChange={(event) =>
                      setRecurringForm((prev) => ({ ...prev, frequency: event.target.value }))
                    }
                  >
                    <option value="weekly">Weekly</option>
                    <option value="monthly">Monthly</option>
                  </select>
                </label>
              </div>

              <div className="grid gap-3 sm:grid-cols-2">
                <label className="block text-sm font-medium">
                  Start Date
                  <input
                    className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
                    type="date"
                    required
                    value={recurringForm.start_date}
                    onChange={(event) =>
                      setRecurringForm((prev) => ({ ...prev, start_date: event.target.value }))
                    }
                  />
                </label>
                <label className="block text-sm font-medium">
                  End Date
                  <input
                    className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
                    type="date"
                    value={recurringForm.end_date}
                    onChange={(event) =>
                      setRecurringForm((prev) => ({ ...prev, end_date: event.target.value }))
                    }
                  />
                </label>
              </div>

              <label className="block text-sm font-medium">
                Category
                <select
                  className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
                  value={recurringForm.category}
                  onChange={(event) =>
                    setRecurringForm((prev) => ({ ...prev, category: event.target.value }))
                  }
                >
                  {categories.map((category) => (
                    <option key={category} value={category}>
                      {category}
                    </option>
                  ))}
                </select>
              </label>

              <label className="block text-sm font-medium">
                Note
                <input
                  className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
                  type="text"
                  value={recurringForm.note}
                  onChange={(event) =>
                    setRecurringForm((prev) => ({ ...prev, note: event.target.value }))
                  }
                />
              </label>

              <button
                className="rounded-lg bg-mint px-4 py-2 text-sm font-semibold text-white transition hover:brightness-95"
                type="submit"
              >
                Add Pattern
              </button>
            </form>

            <div className="mt-4 space-y-2">
              {recurringPatterns.length === 0 && (
                <p className="text-sm text-slate-500">No recurring patterns configured.</p>
              )}

              {recurringPatterns.map((pattern) => (
                <article
                  key={pattern.id}
                  className="rounded-xl border border-slate-200 bg-slate-50 px-3 py-2"
                >
                  <div className="flex items-start justify-between gap-2">
                    <div>
                      <p className="text-sm font-semibold capitalize">
                        {pattern.frequency} {pattern.category}
                      </p>
                      <p className="text-xs text-slate-500">
                        Next run: {formatDate(pattern.next_run_date)}
                      </p>
                    </div>
                    <p className="text-sm font-semibold text-slate-900">
                      {formatCurrency(pattern.amount)}
                    </p>
                  </div>
                  {pattern.note && (
                    <p className="mt-1 text-sm text-slate-700">{pattern.note}</p>
                  )}
                  <div className="mt-2 flex gap-3 text-xs font-medium">
                    <button
                      className="text-sky-700 hover:text-sky-800"
                      type="button"
                      onClick={() => onToggleRecurring(pattern)}
                    >
                      {pattern.active ? 'Pause' : 'Resume'}
                    </button>
                    <button
                      className="text-rose-600 hover:text-rose-700"
                      type="button"
                      onClick={() => onDeleteRecurring(pattern.id)}
                    >
                      Delete
                    </button>
                  </div>
                </article>
              ))}
            </div>
          </Panel>
        </section>

        <Panel title="Upcoming Recurring Preview (60 days)">
          {upcomingExpenses.length === 0 && (
            <p className="text-sm text-slate-500">No upcoming recurring expenses.</p>
          )}
          <div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
            {upcomingExpenses.map((item, index) => (
              <article
                key={`${item.pattern_id}-${item.date}-${index}`}
                className="rounded-xl border border-slate-200 bg-slate-50 p-3"
              >
                <p className="text-xs uppercase tracking-wide text-slate-500">
                  {formatDate(item.date)}
                </p>
                <p className="text-sm font-semibold capitalize">
                  {item.category} • {formatCurrency(item.amount)}
                </p>
                {item.note && <p className="mt-1 text-sm text-slate-700">{item.note}</p>}
              </article>
            ))}
          </div>
        </Panel>
      </main>
    </div>
  )
}

function Panel({ title, children }) {
  return (
    <section className="rounded-2xl border border-slate-200 bg-white/90 p-4 shadow-sm">
      <h2 className="mb-3 text-lg font-semibold">{title}</h2>
      {children}
    </section>
  )
}

function StatCard({ title, value }) {
  return (
    <article className="rounded-2xl border border-slate-200 bg-white/90 p-4 shadow-sm">
      <p className="text-xs uppercase tracking-wide text-slate-500">{title}</p>
      <p className="mt-2 text-2xl font-semibold">{value}</p>
    </article>
  )
}
