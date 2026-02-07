import { apiClient } from './client';

export interface Expense {
  id: string;
  amount: number;
  date: string;
  categoryId: string;
  note?: string;
}

export interface Category {
  id: string;
  name: string;
  color: string;
  isDefault: boolean;
}

export interface Budget {
  id: string;
  categoryId: string;
  month: string;
  amount: number;
}

export interface BudgetVarianceItem {
  categoryId: string;
  categoryName: string;
  budget: number;
  actual: number;
  variance: number;
}

export interface CategoryBreakdownItem {
  categoryId: string;
  categoryName: string;
  amount: number;
  color?: string;
}

interface MonthlyInsightsResponse {
  month: string;
  totalSpend: number;
  categoryBreakdown: CategoryBreakdownItem[];
  budgetVariance: BudgetVarianceItem[];
  recentExpenses?: Expense[];
}

export interface DashboardData {
  month: string;
  totalSpend: number;
  categoryBreakdown: CategoryBreakdownItem[];
  budgetVariance: BudgetVarianceItem[];
  recentExpenses: Expense[];
}

export interface ExpenseFilters {
  from?: string;
  to?: string;
  categoryId?: string;
  limit?: number;
}

export async function getExpenses(filters: ExpenseFilters = {}, signal?: AbortSignal): Promise<Expense[]> {
  return apiClient.get<Expense[]>('/expenses', {
    query: filters,
    signal
  });
}

export async function getCategories(signal?: AbortSignal): Promise<Category[]> {
  return apiClient.get<Category[]>('/categories', { signal });
}

export async function getBudgets(month: string, signal?: AbortSignal): Promise<Budget[]> {
  return apiClient.get<Budget[]>('/budgets', {
    query: { month },
    signal
  });
}

export async function getMonthlyInsights(month: string, signal?: AbortSignal): Promise<MonthlyInsightsResponse> {
  return apiClient.get<MonthlyInsightsResponse>('/insights/monthly', {
    query: { month },
    signal
  });
}

export async function getDashboardData(month: string, signal?: AbortSignal): Promise<DashboardData> {
  const [insights, recentExpenses] = await Promise.all([
    getMonthlyInsights(month, signal),
    getExpenses({ limit: 10 }, signal)
  ]);

  return {
    month: insights.month,
    totalSpend: insights.totalSpend,
    categoryBreakdown: insights.categoryBreakdown ?? [],
    budgetVariance: insights.budgetVariance ?? [],
    recentExpenses: insights.recentExpenses?.length ? insights.recentExpenses : recentExpenses.slice(0, 10)
  };
}
