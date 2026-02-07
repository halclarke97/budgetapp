import { Navigate, createBrowserRouter } from 'react-router-dom';

import { AppShell } from './shell';
import { CategoriesBudgetsView } from '../views/categories-budgets-view';
import { DashboardView } from '../views/dashboard-view';
import { ExpensesView } from '../views/expenses-view';
import { NotFoundView } from '../views/not-found-view';

export const appRouter = createBrowserRouter([
  {
    path: '/',
    element: <AppShell />,
    children: [
      {
        index: true,
        element: <Navigate to="/dashboard" replace />
      },
      {
        path: 'dashboard',
        element: <DashboardView />
      },
      {
        path: 'expenses',
        element: <ExpensesView />
      },
      {
        path: 'categories-budgets',
        element: <CategoriesBudgetsView />
      },
      {
        path: '*',
        element: <NotFoundView />
      }
    ]
  }
]);
