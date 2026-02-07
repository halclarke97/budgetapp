# BudgetApp Integration Test Harness

Tests the CC pipeline end-to-end using budgetapp as a test project.

## Usage

```bash
# Run full integration test
./harness/run-full-test.sh
```

This will:
1. Reset repo to blank slate (wipes git history)
2. Submit 01-initial design task
3. Wait for all tasks to complete
4. Run verification
5. Submit 02-enhancement design task
6. Wait for all tasks to complete
7. Run final verification
8. Generate report

## Files

- `run-full-test.sh` — Full test orchestrator
- `reset.sh` — Reset repo to blank slate
- `submit-design.sh` — Submit a design task
- `wait-complete.sh` — Wait for tasks to complete
- `verify.sh` — Run verification checks
- `report.sh` — Generate test report
- `design-tasks/` — Design task definitions
