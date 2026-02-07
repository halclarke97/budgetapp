import { useState } from 'react';

import { EmptyState } from '../components/async-state/empty-state';
import { ErrorState } from '../components/async-state/error-state';
import { LoadingState } from '../components/async-state/loading-state';
import { useAsyncResource } from '../hooks/use-async-resource';
import { Budget, Category, getBudgets, getCategories } from '../lib/api/budget-api';

const usdCurrency = new Intl.NumberFormat('en-US', {
  style: 'currency',
  currency: 'USD'
});

function currentMonthValue() {
  return new Date().toISOString().slice(0, 7);
}

function BudgetRows({ categories, budgets }: { categories: Category[]; budgets: Budget[] }) {
  const budgetByCategory = new Map<string, Budget>();
  for (const budget of budgets) {
    budgetByCategory.set(budget.categoryId, budget);
  }

  return (
    <ul className="list-reset data-list">
      {categories.map((category) => {
        const budget = budgetByCategory.get(category.id);
        return (
          <li key={category.id}>
            <span>{category.name}</span>
            <strong>{budget ? usdCurrency.format(budget.amount) : 'Not set'}</strong>
          </li>
        );
      })}
    </ul>
  );
}

export function CategoriesBudgetsView() {
  const [month, setMonth] = useState(currentMonthValue());

  const resource = useAsyncResource(
    async (signal) => {
      const [categories, budgets] = await Promise.all([getCategories(signal), getBudgets(month, signal)]);
      return { categories, budgets };
    },
    [month]
  );

  return (
    <section className="page">
      <header className="page-header">
        <h1>Categories & Budgets</h1>
        <label className="field-inline">
          <span>Month</span>
          <input type="month" value={month} onChange={(event) => setMonth(event.target.value)} />
        </label>
      </header>

      {resource.status === 'loading' && <LoadingState label="Loading categories and budgets..." />}

      {resource.status === 'error' && resource.error && (
        <ErrorState error={resource.error} onRetry={() => void resource.reload()} />
      )}

      {resource.status === 'success' && resource.data && resource.data.categories.length === 0 && (
        <EmptyState
          title="No categories yet"
          description="Create at least one category to manage monthly budgets."
        />
      )}

      {resource.status === 'success' && resource.data && resource.data.categories.length > 0 && (
        <div className="two-column-grid">
          <article className="panel">
            <h2>Categories</h2>
            <ul className="list-reset data-list">
              {resource.data.categories.map((category) => (
                <li key={category.id}>
                  <span>
                    <span
                      className="swatch"
                      aria-hidden="true"
                      style={{ backgroundColor: category.color }}
                    />
                    {category.name}
                  </span>
                  <strong>{category.isDefault ? 'Default' : 'Custom'}</strong>
                </li>
              ))}
            </ul>
          </article>

          <article className="panel">
            <h2>Monthly budgets</h2>
            <BudgetRows categories={resource.data.categories} budgets={resource.data.budgets} />
          </article>
        </div>
      )}
    </section>
  );
}
