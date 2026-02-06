# BudgetApp

Simple expense tracker with Go backend and React frontend.

## Purpose

This project serves as an integration test for the [Command Center](https://github.com/halclarke97/command-center) autonomous development pipeline.

## Features

- Add/edit/delete expenses
- Categories (food, transport, entertainment, etc.)
- Date filtering (this week, this month, custom range)
- Dashboard with totals and charts
- Simple reports by category

## Architecture

```
budgetapp/
├── backend/          # Go API server
│   ├── main.go
│   ├── handlers/
│   ├── models/
│   └── store/
├── frontend/         # React dashboard
│   ├── src/
│   └── package.json
├── harness/          # Integration test harness
│   ├── run-test.sh
│   └── verify.sh
└── SPEC.md
```

## License

MIT
