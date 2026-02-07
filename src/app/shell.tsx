import { NavLink, Outlet } from 'react-router-dom';

const navItems = [
  { to: '/dashboard', label: 'Dashboard' },
  { to: '/expenses', label: 'Expenses' },
  { to: '/categories-budgets', label: 'Categories & Budgets' }
];

export function AppShell() {
  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand">BudgetApp</div>
        <nav className="nav-list" aria-label="Primary navigation">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              className={({ isActive }) => (isActive ? 'nav-link active' : 'nav-link')}
            >
              {item.label}
            </NavLink>
          ))}
        </nav>
      </aside>

      <main className="main-content">
        <Outlet />
      </main>
    </div>
  );
}
