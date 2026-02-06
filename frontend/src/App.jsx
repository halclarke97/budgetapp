import { useState, useEffect, useCallback } from 'react'
import { PieChart, Pie, Cell, ResponsiveContainer, BarChart, Bar, XAxis, YAxis, Tooltip } from 'recharts'

const CATEGORY_COLORS = {
  food: '#F97316',
  transportation: '#0EA5E9',
  housing: '#22C55E',
  entertainment: '#EC4899',
  shopping: '#8B5CF6',
  health: '#EF4444',
  education: '#FACC15',
  other: '#64748B',
}

export default function App() {
  const [expenses, setExpenses] = useState([])
  const [categories, setCategories] = useState([])
  const [stats, setStats] = useState(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [showModal, setShowModal] = useState(false)
  const [editingExpense, setEditingExpense] = useState(null)
  const [filter, setFilter] = useState({ category: '', period: 'month' })

  const fetchData = useCallback(async () => {
    try {
      setLoading(true)
      const [expensesRes, categoriesRes, statsRes] = await Promise.all([
        apiRequest('/api/expenses'),
        apiRequest('/api/categories'),
        apiRequest(`/api/stats?period=${filter.period}`),
      ])
      setExpenses(expensesRes || [])
      setCategories(categoriesRes || [])
      setStats(statsRes)
      setError(null)
    } catch (err) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }, [filter.period])

  useEffect(() => {
    fetchData()
  }, [fetchData])

  const handleAddExpense = () => {
    setEditingExpense(null)
    setShowModal(true)
  }

  const handleEditExpense = (expense) => {
    setEditingExpense(expense)
    setShowModal(true)
  }

  const handleDeleteExpense = async (id) => {
    if (!confirm('Delete this expense?')) return
    try {
      await apiRequest(`/api/expenses/${id}`, { method: 'DELETE' })
      fetchData()
    } catch (err) {
      alert(err.message)
    }
  }

  const handleSaveExpense = async (data) => {
    try {
      if (editingExpense) {
        await apiRequest(`/api/expenses/${editingExpense.id}`, {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(data),
        })
      } else {
        await apiRequest('/api/expenses', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(data),
        })
      }
      setShowModal(false)
      fetchData()
    } catch (err) {
      throw err
    }
  }

  const filteredExpenses = expenses.filter((exp) => {
    if (filter.category && exp.category !== filter.category) return false
    return true
  })

  if (loading && expenses.length === 0) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p className="text-slate-500">Loading...</p>
      </div>
    )
  }

  return (
    <div className="mx-auto max-w-6xl px-4 py-8">
      <header className="mb-8 flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">BudgetApp</h1>
          <p className="text-slate-500">Track your expenses</p>
        </div>
        <button className="btn-primary" onClick={handleAddExpense}>
          + Add Expense
        </button>
      </header>

      {error && (
        <div className="mb-4 rounded-xl bg-red-100 p-4 text-red-700">
          {error}
        </div>
      )}

      {/* Stats Cards */}
      <div className="mb-8 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <div className="card">
          <p className="text-sm text-slate-500">Total Expenses</p>
          <p className="text-2xl font-bold">{stats?.total_expenses || 0}</p>
        </div>
        <div className="card">
          <p className="text-sm text-slate-500">Total Amount</p>
          <p className="text-2xl font-bold">${(stats?.total_amount || 0).toFixed(2)}</p>
        </div>
        <div className="card">
          <p className="text-sm text-slate-500">This {filter.period}</p>
          <p className="text-2xl font-bold">${(stats?.period_total || 0).toFixed(2)}</p>
        </div>
        <div className="card">
          <select
            className="input w-full"
            value={filter.period}
            onChange={(e) => setFilter((f) => ({ ...f, period: e.target.value }))}
          >
            <option value="week">This Week</option>
            <option value="month">This Month</option>
          </select>
        </div>
      </div>

      {/* Charts */}
      <div className="mb-8 grid gap-4 lg:grid-cols-2">
        <div className="card">
          <h2 className="mb-4 font-semibold">By Category</h2>
          {stats?.by_category?.length > 0 ? (
            <ResponsiveContainer width="100%" height={200}>
              <PieChart>
                <Pie
                  data={stats.by_category}
                  dataKey="total"
                  nameKey="category"
                  cx="50%"
                  cy="50%"
                  outerRadius={80}
                  label={({ category, total }) => `${category}: $${total.toFixed(0)}`}
                >
                  {stats.by_category.map((entry) => (
                    <Cell key={entry.category} fill={CATEGORY_COLORS[entry.category] || '#64748B'} />
                  ))}
                </Pie>
                <Tooltip formatter={(value) => `$${value.toFixed(2)}`} />
              </PieChart>
            </ResponsiveContainer>
          ) : (
            <p className="text-center text-slate-400">No data</p>
          )}
        </div>
        <div className="card">
          <h2 className="mb-4 font-semibold">Spending Trend</h2>
          {stats?.trend?.length > 0 ? (
            <ResponsiveContainer width="100%" height={200}>
              <BarChart data={stats.trend}>
                <XAxis dataKey="date" tick={{ fontSize: 10 }} />
                <YAxis tick={{ fontSize: 10 }} />
                <Tooltip formatter={(value) => `$${value.toFixed(2)}`} />
                <Bar dataKey="total" fill="#0EA5E9" radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          ) : (
            <p className="text-center text-slate-400">No data</p>
          )}
        </div>
      </div>

      {/* Filter */}
      <div className="mb-4 flex gap-2">
        <select
          className="input"
          value={filter.category}
          onChange={(e) => setFilter((f) => ({ ...f, category: e.target.value }))}
        >
          <option value="">All Categories</option>
          {categories.map((cat) => (
            <option key={cat.id} value={cat.id}>
              {cat.name}
            </option>
          ))}
        </select>
      </div>

      {/* Expense List */}
      <div className="card">
        <h2 className="mb-4 font-semibold">Expenses</h2>
        {filteredExpenses.length === 0 ? (
          <p className="text-center text-slate-400">No expenses yet</p>
        ) : (
          <div className="space-y-2">
            {filteredExpenses.map((expense) => (
              <div
                key={expense.id}
                className="flex items-center justify-between rounded-xl bg-slate-50 p-3"
              >
                <div className="flex items-center gap-3">
                  <div
                    className="h-3 w-3 rounded-full"
                    style={{ backgroundColor: CATEGORY_COLORS[expense.category] || '#64748B' }}
                  />
                  <div>
                    <p className="font-medium">{expense.note || expense.category}</p>
                    <p className="text-sm text-slate-500">
                      {new Date(expense.date).toLocaleDateString()}
                    </p>
                  </div>
                </div>
                <div className="flex items-center gap-3">
                  <span className="font-semibold">${expense.amount.toFixed(2)}</span>
                  <button
                    className="text-slate-400 hover:text-slate-600"
                    onClick={() => handleEditExpense(expense)}
                  >
                    Edit
                  </button>
                  <button
                    className="text-red-400 hover:text-red-600"
                    onClick={() => handleDeleteExpense(expense.id)}
                  >
                    Delete
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {showModal && (
        <ExpenseModal
          expense={editingExpense}
          categories={categories}
          onSave={handleSaveExpense}
          onClose={() => setShowModal(false)}
        />
      )}
    </div>
  )
}

function ExpenseModal({ expense, categories, onSave, onClose }) {
  const [form, setForm] = useState({
    amount: expense?.amount?.toString() || '',
    category: expense?.category || 'other',
    note: expense?.note || '',
    date: expense?.date ? new Date(expense.date).toISOString().split('T')[0] : new Date().toISOString().split('T')[0],
  })
  const [error, setError] = useState(null)
  const [submitting, setSubmitting] = useState(false)

  const handleSubmit = async (e) => {
    e.preventDefault()
    setSubmitting(true)
    setError(null)
    try {
      await onSave({
        amount: parseFloat(form.amount),
        category: form.category,
        note: form.note,
        date: form.date,
      })
    } catch (err) {
      setError(err.message)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div className="card w-full max-w-md">
        <h2 className="mb-4 text-xl font-bold">{expense ? 'Edit Expense' : 'Add Expense'}</h2>
        <form onSubmit={handleSubmit} className="space-y-4">
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
            onChange={(e) => setForm((prev) => ({ ...prev, amount: e.target.value }))}
            required
          />
          <label className="block text-sm font-semibold text-slate-600" htmlFor="category">
            Category
          </label>
          <select
            id="category"
            className="input w-full"
            value={form.category}
            onChange={(e) => setForm((prev) => ({ ...prev, category: e.target.value }))}
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
            onChange={(e) => setForm((prev) => ({ ...prev, note: e.target.value }))}
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
            onChange={(e) => setForm((prev) => ({ ...prev, date: e.target.value }))}
          />
          {error && <p className="text-sm text-red-600">{error}</p>}
          <div className="mt-6 flex justify-end gap-3">
            <button type="button" className="btn-secondary" onClick={onClose} disabled={submitting}>
              Cancel
            </button>
            <button type="submit" className="btn-primary" disabled={submitting}>
              {submitting ? 'Saving...' : 'Save'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
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
