import { useState } from 'react';

import { EmptyState } from '../components/async-state/empty-state';
import { ErrorState } from '../components/async-state/error-state';
import { LoadingState } from '../components/async-state/loading-state';
import { useAsyncResource } from '../hooks/use-async-resource';
import { Expense, getExpenses } from '../lib/api/budget-api';

const usdCurrency = new Intl.NumberFormat('en-US', {
  style: 'currency',
  currency: 'USD'
});

function ExpenseList({ expenses }: { expenses: Expense[] }) {
  return (
    <ul className="list-reset data-list">
      {expenses.map((expense) => (
        <li key={expense.id}>
          <div>
            <strong>{expense.note || 'No note'}</strong>
            <p className="muted">{expense.date}</p>
          </div>
          <strong>{usdCurrency.format(expense.amount)}</strong>
        </li>
      ))}
    </ul>
  );
}

export function ExpensesView() {
  const [from, setFrom] = useState('');
  const [to, setTo] = useState('');
  const [categoryId, setCategoryId] = useState('');

  const resource = useAsyncResource(
    (signal) =>
      getExpenses(
        {
          from: from || undefined,
          to: to || undefined,
          categoryId: categoryId || undefined
        },
        signal
      ),
    [from, to, categoryId]
  );

  return (
    <section className="page">
      <header className="page-header">
        <h1>Expenses</h1>
      </header>

      <div className="panel filters">
        <label>
          <span>From</span>
          <input type="date" value={from} onChange={(event) => setFrom(event.target.value)} />
        </label>

        <label>
          <span>To</span>
          <input type="date" value={to} onChange={(event) => setTo(event.target.value)} />
        </label>

        <label>
          <span>Category ID</span>
          <input
            type="text"
            value={categoryId}
            placeholder="Optional"
            onChange={(event) => setCategoryId(event.target.value)}
          />
        </label>
      </div>

      {resource.status === 'loading' && <LoadingState label="Loading expenses..." />}

      {resource.status === 'error' && resource.error && (
        <ErrorState error={resource.error} onRetry={() => void resource.reload()} />
      )}

      {resource.status === 'success' && resource.data && resource.data.length === 0 && (
        <EmptyState
          title="No expenses found"
          description="Try adjusting filters or add your first expense."
        />
      )}

      {resource.status === 'success' && resource.data && resource.data.length > 0 && (
        <article className="panel">
          <h2>Expense list</h2>
          <ExpenseList expenses={resource.data} />
        </article>
      )}
    </section>
  );
}
