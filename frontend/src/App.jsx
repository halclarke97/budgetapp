import { useEffect, useMemo, useState } from 'react'
import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Legend,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'

const money = new Intl.NumberFormat('en-US', {
  style: 'currency',
  currency: 'USD',
})

const dateFormatter = new Intl.DateTimeFormat('en-US', {
  month: 'short',
  day: 'numeric',
  year: 'numeric',
  timeZone: 'UTC',
})

const baseForm = {
  amount: '',
  category: 'other',
  note: '',
  date: new Date().toISOString().slice(0, 10),
}

const recurringBaseForm = {
  amount: '',
  category: 'other',
  note: '',
  frequency: 'monthly',
  startDate: new Date().toISOString().slice(0, 10),
  endDate: '',
}

export default function App() {
  const [expenses, setExpenses] = useState([])
  const [categories, setCategories] = useState([])
  const [stats, setStats] = useState(null)
  const [recurringPatterns, setRecurringPatterns] = useState([])
  const [upcomingRecurring, setUpcomingRecurring] = useState([])
  const [period, setPeriod] = useState('month')
  const [filters, setFilters] = useState({ category: '', from: '', to: '' })
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState('')
  const [modalOpen, setModalOpen] = useState(false)
  const [editingExpense, setEditingExpense] = useState(null)
  const [recurringModalOpen, setRecurringModalOpen] = useState(false)
  const [editingRecurring, setEditingRecurring] = useState(null)

  const categoryMap = useMemo(() => {
    return categories.reduce((acc, category) => {
      acc[category.id] = category
      return acc
    }, {})
  }, [categories])

  useEffect(() => {
    void refreshAll()
  }, [period, filters.category, filters.from, filters.to])

  async function refreshAll() {
    try {
      setIsLoading(true)
      setError('')

      const query = new URLSearchParams()
      if (filters.category) query.set('category', filters.category)
      if (filters.from) query.set('from', filters.from)
      if (filters.to) query.set('to', filters.to)

      const [categoriesData, expensesData, statsData, patternsData, upcomingData] = await Promise.all([
        apiRequest('/api/categories'),
        apiRequest(`/api/expenses${query.toString() ? `?${query.toString()}` : ''}`),
        apiRequest(`/api/stats?period=${period}`),
        apiRequest('/api/recurring-expenses'),
        apiRequest('/api/recurring-expenses/upcoming?days=30'),
      ])

      setCategories(categoriesData)
      setExpenses(expensesData)
      setStats(statsData)
      setRecurringPatterns(patternsData)
      setUpcomingRecurring(upcomingData)
    } catch (err) {
      setError(err.message)
    } finally {
      setIsLoading(false)
    }
  }

  function openCreateModal() {
    setEditingExpense(null)
    setModalOpen(true)
  }

  function openEditModal(expense) {
    setEditingExpense(expense)
    setModalOpen(true)
  }

  function openEditRecurringModal(pattern) {
    setEditingRecurring(pattern)
    setRecurringModalOpen(true)
  }

  async function saveExpense(formValues) {
    const payload = {
      amount: Number(formValues.amount),
      category: formValues.category,
      note: formValues.note,
      date: formValues.date,
    }

    if (editingExpense) {
      await apiRequest(`/api/expenses/${editingExpense.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      })
    } else {
      await apiRequest('/api/expenses', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      })
    }

    setModalOpen(false)
    setEditingExpense(null)
    await refreshAll()
  }

  async function saveRecurringPattern(formValues) {
    const payload = {
      amount: Number(formValues.amount),
      category: formValues.category,
      note: formValues.note,
      frequency: formValues.frequency,
      start_date: formValues.startDate,
      end_date: formValues.endDate || '',
    }

    await apiRequest(`/api/recurring-expenses/${editingRecurring.id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    })

    setRecurringModalOpen(false)
    setEditingRecurring(null)
    await refreshAll()
  }

  async function deleteExpense(id) {
    if (!window.confirm('Delete this expense?')) {
      return
    }

    try {
      await apiRequest(`/api/expenses/${id}`, { method: 'DELETE' })
      await refreshAll()
    } catch (err) {
      setError(err.message)
    }
  }

  async function deactivateRecurringPattern(id) {
    if (!window.confirm('Deactivate this recurring pattern?')) {
      return
    }

    try {
      await apiRequest(`/api/recurring-expenses/${id}`, { method: 'DELETE' })
      await refreshAll()
    } catch (err) {
      setError(err.message)
    }
  }

  const pieData = useMemo(() => {
    if (!stats) return []
    return stats.by_category.map((item) => {
      const category = categoryMap[item.category]
      return {
        name: category?.name ?? item.category,
        value: item.total,
        color: category?.color ?? '#64748B',
      }
    })
  }, [stats, categoryMap])

  const trendData = stats?.trend ?? []

  return (
    <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
      <header className="mb-6 flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
        <div>
          <p className="text-sm uppercase tracking-[0.2em] text-slate-500">BudgetApp</p>
          <h1 className="text-4xl font-bold text-ink">Expense Dashboard</h1>
          <p className="mt-2 text-slate-600">Track spending, spot trends, and keep daily costs under control.</p>
        </div>
        <div className="flex flex-wrap gap-3">
          <select className="input" value={period} onChange={(event) => setPeriod(event.target.value)}>
            <option value="week">This week</option>
            <option value="month">This month</option>
          </select>
          <button type="button" className="btn-primary" onClick={openCreateModal}>
            Add expense
          </button>
        </div>
      </header>

      {error ? (
        <div className="mb-6 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-red-700">{error}</div>
      ) : null}

      <section className="mb-6 grid gap-4 md:grid-cols-3">
        <SummaryCard
          label="Total spending"
          value={money.format(stats?.total_amount ?? 0)}
          hint={`${stats?.total_expenses ?? 0} expenses logged`}
        />
        <SummaryCard
          label="Current period"
          value={money.format(stats?.period_total ?? 0)}
          hint={period === 'week' ? 'Monday to today' : 'Month to date'}
        />
        <SummaryCard
          label="Average expense"
          value={money.format(
            stats?.total_expenses ? (stats.total_amount ?? 0) / stats.total_expenses : 0,
          )}
          hint="Per transaction"
        />
      </section>

      <section className="mb-6 grid gap-6 xl:grid-cols-[1.3fr_1fr]">
        <div className="card">
          <div className="mb-4 flex items-center justify-between">
            <h2 className="text-xl font-semibold">Upcoming recurring (30 days)</h2>
            <span className="text-xs uppercase tracking-[0.15em] text-slate-500">
              {upcomingRecurring.length} occurrence{upcomingRecurring.length === 1 ? '' : 's'}
            </span>
          </div>
          {isLoading ? (
            <p className="text-slate-500">Loading upcoming occurrences...</p>
          ) : recurringPatterns.length === 0 ? (
            <p className="text-slate-500">No recurring patterns yet. Upcoming occurrences will appear here.</p>
          ) : upcomingRecurring.length === 0 ? (
            <p className="text-slate-500">No upcoming recurring expenses in the next 30 days.</p>
          ) : (
            <div className="overflow-auto">
              <table className="min-w-full border-collapse text-sm">
                <thead>
                  <tr className="border-b border-slate-200 text-left text-slate-600">
                    <th className="px-3 py-2">Date</th>
                    <th className="px-3 py-2">Category</th>
                    <th className="px-3 py-2">Note</th>
                    <th className="px-3 py-2 text-right">Amount</th>
                  </tr>
                </thead>
                <tbody>
                  {upcomingRecurring.map((occurrence) => (
                    <tr
                      key={`${occurrence.pattern_id}-${occurrence.date}`}
                      className="border-b border-slate-100 last:border-b-0"
                    >
                      <td className="px-3 py-3">{formatCalendarDate(occurrence.date)}</td>
                      <td className="px-3 py-3">
                        {categoryMap[occurrence.category]?.name ?? occurrence.category}
                      </td>
                      <td className="px-3 py-3">{occurrence.note || '-'}</td>
                      <td className="px-3 py-3 text-right font-semibold">{money.format(occurrence.amount)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
        <div className="card">
          <div className="mb-4 flex items-center justify-between">
            <h2 className="text-xl font-semibold">Recurring patterns</h2>
            <span className="text-xs uppercase tracking-[0.15em] text-slate-500">
              {recurringPatterns.length} total
            </span>
          </div>
          {isLoading ? (
            <p className="text-slate-500">Loading recurring patterns...</p>
          ) : recurringPatterns.length === 0 ? (
            <p className="text-slate-500">No recurring patterns yet. Create one from the expense flow.</p>
          ) : (
            <ul className="max-h-[24rem] space-y-3 overflow-auto pr-1">
              {recurringPatterns.map((pattern) => (
                <li key={pattern.id} className="rounded-xl border border-slate-200 px-4 py-3">
                  <div className="flex items-start justify-between gap-2">
                    <div>
                      <p className="font-semibold text-ink">{pattern.note || 'Untitled recurring expense'}</p>
                      <p className="text-sm text-slate-500">
                        {categoryMap[pattern.category]?.name ?? pattern.category} • {formatFrequency(pattern.frequency)}
                      </p>
                    </div>
                    <p className="text-lg font-semibold">{money.format(pattern.amount)}</p>
                  </div>
                  <div className="mt-2 flex flex-wrap items-center justify-between gap-2">
                    <p className="text-sm text-slate-500">
                      Next run: {formatCalendarDate(pattern.next_run_date)}
                    </p>
                    <span
                      className={`rounded-full px-2 py-1 text-xs font-semibold ${
                        pattern.active ? 'bg-emerald-100 text-emerald-800' : 'bg-slate-200 text-slate-700'
                      }`}
                    >
                      {pattern.active ? 'Active' : 'Inactive'}
                    </span>
                  </div>
                  <div className="mt-3 flex justify-end gap-3">
                    <button
                      type="button"
                      className="text-sm font-semibold text-sky-700"
                      onClick={() => openEditRecurringModal(pattern)}
                    >
                      Edit
                    </button>
                    <button
                      type="button"
                      className="text-sm font-semibold text-red-600 disabled:cursor-not-allowed disabled:text-slate-400"
                      onClick={() => deactivateRecurringPattern(pattern.id)}
                      disabled={!pattern.active}
                    >
                      Deactivate
                    </button>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </div>
      </section>

      <section className="mb-6 grid gap-6 xl:grid-cols-[1.4fr_1fr]">
        <div className="card">
          <div className="mb-4 flex items-center justify-between">
            <h2 className="text-xl font-semibold">Trend</h2>
            <span className="text-xs uppercase tracking-[0.15em] text-slate-500">Daily totals</span>
          </div>
          <div className="h-72">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={trendData}>
                <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
                <XAxis dataKey="date" tick={{ fontSize: 12 }} />
                <YAxis tickFormatter={(value) => `$${value}`} tick={{ fontSize: 12 }} />
                <Tooltip formatter={(value) => money.format(Number(value))} />
                <Bar dataKey="total" radius={[8, 8, 0, 0]} fill="#0EA5E9" />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>
        <div className="card">
          <h2 className="mb-4 text-xl font-semibold">By category</h2>
          <div className="h-72">
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie data={pieData} dataKey="value" nameKey="name" outerRadius={90} innerRadius={45}>
                  {pieData.map((entry) => (
                    <Cell key={entry.name} fill={entry.color} />
                  ))}
                </Pie>
                <Legend />
                <Tooltip formatter={(value) => money.format(Number(value))} />
              </PieChart>
            </ResponsiveContainer>
          </div>
        </div>
      </section>

      <section className="mb-6 card">
        <div className="mb-4 flex flex-wrap gap-3">
          <select
            className="input"
            value={filters.category}
            onChange={(event) => setFilters((prev) => ({ ...prev, category: event.target.value }))}
          >
            <option value="">All categories</option>
            {categories.map((category) => (
              <option key={category.id} value={category.id}>
                {category.name}
              </option>
            ))}
          </select>
          <input
            className="input"
            type="date"
            value={filters.from}
            onChange={(event) => setFilters((prev) => ({ ...prev, from: event.target.value }))}
          />
          <input
            className="input"
            type="date"
            value={filters.to}
            onChange={(event) => setFilters((prev) => ({ ...prev, to: event.target.value }))}
          />
          <button
            type="button"
            className="btn-secondary"
            onClick={() => setFilters({ category: '', from: '', to: '' })}
          >
            Clear filters
          </button>
        </div>

        <h2 className="mb-3 text-xl font-semibold">Expenses</h2>

        {isLoading ? (
          <p className="text-slate-500">Loading expenses...</p>
        ) : expenses.length === 0 ? (
          <p className="text-slate-500">No expenses found for selected filters.</p>
        ) : (
          <div className="overflow-auto">
            <table className="min-w-full border-collapse text-sm">
              <thead>
                <tr className="border-b border-slate-200 text-left text-slate-600">
                  <th className="px-3 py-2">Date</th>
                  <th className="px-3 py-2">Category</th>
                  <th className="px-3 py-2">Note</th>
                  <th className="px-3 py-2 text-right">Amount</th>
                  <th className="px-3 py-2 text-right">Actions</th>
                </tr>
              </thead>
              <tbody>
                {expenses.map((expense) => (
                  <tr key={expense.id} className="border-b border-slate-100">
                    <td className="px-3 py-3">{formatCalendarDate(expense.date)}</td>
                    <td className="px-3 py-3">{categoryMap[expense.category]?.name ?? expense.category}</td>
                    <td className="px-3 py-3">{expense.note || '-'}</td>
                    <td className="px-3 py-3 text-right font-semibold">{money.format(expense.amount)}</td>
                    <td className="px-3 py-3 text-right">
                      <button
                        type="button"
                        className="mr-2 text-sm font-semibold text-sky-700"
                        onClick={() => openEditModal(expense)}
                      >
                        Edit
                      </button>
                      <button
                        type="button"
                        className="text-sm font-semibold text-red-600"
                        onClick={() => deleteExpense(expense.id)}
                      >
                        Delete
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <section className="card">
        <h2 className="mb-3 text-xl font-semibold">Recent expenses</h2>
        <ul className="space-y-3">
          {expenses.slice(0, 5).map((expense) => (
            <li
              key={`recent-${expense.id}`}
              className="flex items-center justify-between rounded-xl border border-slate-100 px-4 py-3"
            >
              <div>
                <p className="font-semibold text-ink">{expense.note || 'Untitled expense'}</p>
                <p className="text-sm text-slate-500">
                  {categoryMap[expense.category]?.name ?? expense.category} •{' '}
                  {formatCalendarDate(expense.date)}
                </p>
              </div>
              <p className="font-semibold">{money.format(expense.amount)}</p>
            </li>
          ))}
          {expenses.length === 0 ? <li className="text-slate-500">Add your first expense to start tracking.</li> : null}
        </ul>
      </section>

      {modalOpen ? (
        <ExpenseModal
          categories={categories}
          initialValues={editingExpense}
          onClose={() => {
            setModalOpen(false)
            setEditingExpense(null)
          }}
          onSave={saveExpense}
        />
      ) : null}

      {recurringModalOpen && editingRecurring ? (
        <RecurringPatternModal
          categories={categories}
          initialValues={editingRecurring}
          onClose={() => {
            setRecurringModalOpen(false)
            setEditingRecurring(null)
          }}
          onSave={saveRecurringPattern}
        />
      ) : null}
    </div>
  )
}

function SummaryCard({ label, value, hint }) {
  return (
    <div className="card">
      <p className="text-xs uppercase tracking-[0.16em] text-slate-500">{label}</p>
      <p className="mt-2 text-3xl font-bold text-ink">{value}</p>
      <p className="mt-1 text-sm text-slate-500">{hint}</p>
    </div>
  )
}

function ExpenseModal({ categories, initialValues, onClose, onSave }) {
  const [form, setForm] = useState(() => {
    if (!initialValues) {
      return baseForm
    }
    return {
      amount: String(initialValues.amount),
      category: initialValues.category,
      note: initialValues.note ?? '',
      date: toDateInput(initialValues.date),
    }
  })
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  async function handleSubmit(event) {
    event.preventDefault()

    if (!form.amount || Number(form.amount) <= 0) {
      setError('Amount must be greater than zero.')
      return
    }

    try {
      setSubmitting(true)
      setError('')
      await onSave(form)
    } catch (err) {
      setError(err.message)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="fixed inset-0 z-20 flex items-center justify-center bg-slate-900/45 p-4">
      <div className="w-full max-w-md rounded-2xl bg-white p-6 shadow-2xl">
        <h3 className="text-2xl font-bold text-ink">{initialValues ? 'Edit expense' : 'Add expense'}</h3>
        <form className="mt-4 space-y-3" onSubmit={handleSubmit}>
          <label className="block text-sm font-semibold text-slate-600" htmlFor="amount">
            Amount
          </label>
          <input
            id="amount"
            className="input w-full"
            type="number"
            min="0"
            step="0.01"
            value={form.amount}
            onChange={(event) => setForm((prev) => ({ ...prev, amount: event.target.value }))}
          />

          <label className="block text-sm font-semibold text-slate-600" htmlFor="category">
            Category
          </label>
          <select
            id="category"
            className="input w-full"
            value={form.category}
            onChange={(event) => setForm((prev) => ({ ...prev, category: event.target.value }))}
          >
            {categories.map((category) => (
              <option key={category.id} value={category.id}>
                {category.name}
              </option>
            ))}
          </select>

          <label className="block text-sm font-semibold text-slate-600" htmlFor="note">
            Note
          </label>
          <input
            id="note"
            className="input w-full"
            type="text"
            value={form.note}
            onChange={(event) => setForm((prev) => ({ ...prev, note: event.target.value }))}
            placeholder="Coffee, rent, groceries..."
          />

          <label className="block text-sm font-semibold text-slate-600" htmlFor="date">
            Date
          </label>
          <input
            id="date"
            className="input w-full"
            type="date"
            value={form.date}
            onChange={(event) => setForm((prev) => ({ ...prev, date: event.target.value }))}
          />

          {error ? <p className="text-sm text-red-600">{error}</p> : null}

          <div className="mt-6 flex justify-end gap-3">
            <button type="button" className="btn-secondary" onClick={onClose} disabled={submitting}>
              Cancel
            </button>
            <button type="submit" className="btn-primary" disabled={submitting}>
              {submitting ? 'Saving...' : 'Save expense'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

function RecurringPatternModal({ categories, initialValues, onClose, onSave }) {
  const [form, setForm] = useState(() => {
    if (!initialValues) {
      return recurringBaseForm
    }
    return {
      amount: String(initialValues.amount),
      category: initialValues.category,
      note: initialValues.note ?? '',
      frequency: initialValues.frequency ?? 'monthly',
      startDate: toDateInput(initialValues.start_date),
      endDate: initialValues.end_date ? toDateInput(initialValues.end_date) : '',
    }
  })
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  async function handleSubmit(event) {
    event.preventDefault()

    if (!form.amount || Number(form.amount) <= 0) {
      setError('Amount must be greater than zero.')
      return
    }
    if (!form.startDate) {
      setError('Start date is required.')
      return
    }
    if (form.endDate && form.endDate < form.startDate) {
      setError('End date cannot be before start date.')
      return
    }

    try {
      setSubmitting(true)
      setError('')
      await onSave(form)
    } catch (err) {
      setError(err.message)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="fixed inset-0 z-20 flex items-center justify-center bg-slate-900/45 p-4">
      <div className="w-full max-w-md rounded-2xl bg-white p-6 shadow-2xl">
        <h3 className="text-2xl font-bold text-ink">Edit recurring pattern</h3>
        <form className="mt-4 space-y-3" onSubmit={handleSubmit}>
          <label className="block text-sm font-semibold text-slate-600" htmlFor="recurring-amount">
            Amount
          </label>
          <input
            id="recurring-amount"
            className="input w-full"
            type="number"
            min="0"
            step="0.01"
            value={form.amount}
            onChange={(event) => setForm((prev) => ({ ...prev, amount: event.target.value }))}
          />

          <label className="block text-sm font-semibold text-slate-600" htmlFor="recurring-category">
            Category
          </label>
          <select
            id="recurring-category"
            className="input w-full"
            value={form.category}
            onChange={(event) => setForm((prev) => ({ ...prev, category: event.target.value }))}
          >
            {categories.map((category) => (
              <option key={category.id} value={category.id}>
                {category.name}
              </option>
            ))}
          </select>

          <label className="block text-sm font-semibold text-slate-600" htmlFor="recurring-frequency">
            Frequency
          </label>
          <select
            id="recurring-frequency"
            className="input w-full"
            value={form.frequency}
            onChange={(event) => setForm((prev) => ({ ...prev, frequency: event.target.value }))}
          >
            <option value="weekly">Weekly</option>
            <option value="monthly">Monthly</option>
          </select>

          <label className="block text-sm font-semibold text-slate-600" htmlFor="recurring-note">
            Note
          </label>
          <input
            id="recurring-note"
            className="input w-full"
            type="text"
            value={form.note}
            onChange={(event) => setForm((prev) => ({ ...prev, note: event.target.value }))}
            placeholder="Subscription, rent, membership..."
          />

          <label className="block text-sm font-semibold text-slate-600" htmlFor="recurring-start-date">
            Start date
          </label>
          <input
            id="recurring-start-date"
            className="input w-full"
            type="date"
            value={form.startDate}
            onChange={(event) => setForm((prev) => ({ ...prev, startDate: event.target.value }))}
          />

          <label className="block text-sm font-semibold text-slate-600" htmlFor="recurring-end-date">
            End date (optional)
          </label>
          <input
            id="recurring-end-date"
            className="input w-full"
            type="date"
            value={form.endDate}
            onChange={(event) => setForm((prev) => ({ ...prev, endDate: event.target.value }))}
          />

          {error ? <p className="text-sm text-red-600">{error}</p> : null}

          <div className="mt-6 flex justify-end gap-3">
            <button type="button" className="btn-secondary" onClick={onClose} disabled={submitting}>
              Cancel
            </button>
            <button type="submit" className="btn-primary" disabled={submitting}>
              {submitting ? 'Saving...' : 'Save pattern'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

function toDateInput(value) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return ''
  }
  return date.toISOString().slice(0, 10)
}

function formatCalendarDate(value) {
  const dateInput = toDateInput(value)
  if (!dateInput) {
    return '-'
  }
  return dateFormatter.format(new Date(`${dateInput}T00:00:00Z`))
}

function formatFrequency(value) {
  if (value === 'weekly') {
    return 'Weekly'
  }
  if (value === 'monthly') {
    return 'Monthly'
  }
  return value
}

async function apiRequest(path, options = {}) {
  const response = await fetch(path, options)
  if (!response.ok) {
    const body = await response.json().catch(() => ({}))
    throw new Error(body.error ?? `Request failed with status ${response.status}`)
  }
  if (response.status === 204) {
    return null
  }
  return response.json()
}
