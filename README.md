# BudgetApp

BudgetApp is a simple full-stack expense tracker with a Go backend and React frontend.

## Project Structure

- `backend/` — Go API with JSON-file persistence
- `frontend/` — React + Vite + Tailwind UI
- `.cc/tasks/` — Implementation task manifests

## Run Backend

```bash
cd backend
go run .
```

API server starts on `http://localhost:8080`.

## Run Frontend

```bash
cd frontend
npm install
npm run dev
```

Frontend starts on `http://localhost:5173` and calls backend at `http://localhost:8080` by default.

## Build Checks

```bash
cd backend && go build .
cd ../frontend && npm run build
```
