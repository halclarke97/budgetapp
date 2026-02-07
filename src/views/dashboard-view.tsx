import { useMemo, useState } from 'react';

import { EmptyState } from '../components/async-state/empty-state';
import { ErrorState } from '../components/async-state/error-state';
import { LoadingState } from '../components/async-state/loading-state';
import { useAsyncResource } from '../hooks/use-async-resource';
import { getDashboardData } from '../lib/api/budget-api';

const usdCurrency = new Intl.NumberFormat('en-US', {
  style: 'currency',
  currency: 'USD'
});

function currentMonthValue() {
  return new Date().toISOString().slice(0, 7);
}

export function DashboardView() {
  const [month, setMonth] = useState(currentMonthValue());

  const resource = useAsyncResource((signal) => getDashboardData(month, signal), [month]);

  const hasAnyData = useMemo(() => {
    if (!resource.data) {
      return false;
    }

    return (
      resource.data.totalSpend > 0 ||
      resource.data.categoryBreakdown.length > 0 ||
      resource.data.budgetVariance.length > 0 ||
      resource.data.recentExpenses.length > 0
    );
  }, [resource.data]);

  return (
    <section className="page">
      <header className="page-header">
        <h1>Dashboard</h1>
        <label className="field-inline">
          <span>Month</span>
          <input type="month" value={month} onChange={(event) => setMonth(event.target.value)} />
        </label>
      </header>

      {resource.status === 'loading' && <LoadingState label="Loading dashboard..." />}

      {resource.status === 'error' && resource.error && (
        <ErrorState error={resource.error} onRetry={() => void resource.reload()} />
      )}

      {resource.status === 'success' && resource.data && !hasAnyData && (
        <EmptyState
          title="No insights yet"
          description="Add categories, budgets, and expenses to populate your dashboard."
        />
      )}

      {resource.status === 'success' && resource.data && hasAnyData && (
        <div className="dashboard-grid">
          <article className="panel stat-panel">
            <h2>Total spend</h2>
            <p className="metric">{usdCurrency.format(resource.data.totalSpend)}</p>
            <p className="muted">For {resource.data.month}</p>
          </article>

          <article className="panel">
            <h2>Category breakdown</h2>
            <ul className="list-reset data-list">
              {resource.data.categoryBreakdown.map((item) => (
                <li key={item.categoryId}>
                  <span>{item.categoryName}</span>
                  <strong>{usdCurrency.format(item.amount)}</strong>
                </li>
              ))}
            </ul>
          </article>

          <article className="panel">
            <h2>Budget variance</h2>
            <ul className="list-reset data-list">
              {resource.data.budgetVariance.map((item) => (
                <li key={item.categoryId}>
                  <span>{item.categoryName}</span>
                  <strong className={item.variance < 0 ? 'danger' : 'success'}>
                    {usdCurrency.format(item.variance)}
                  </strong>
                </li>
              ))}
            </ul>
          </article>

          <article className="panel full-width">
            <h2>Recent expenses</h2>
            <ul className="list-reset data-list">
              {resource.data.recentExpenses.map((expense) => (
                <li key={expense.id}>
                  <span>{expense.note || expense.date}</span>
                  <strong>{usdCurrency.format(expense.amount)}</strong>
                </li>
              ))}
            </ul>
          </article>
        </div>
      )}
    </section>
  );
}
