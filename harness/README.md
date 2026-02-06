# BudgetApp Integration Test Harness

Tests the CC/OPERATIONS pipeline end-to-end using budgetapp as a guinea pig.

## Quick Start

```bash
# Run full integration test
./harness/run-full-test.sh

# Or step by step:
./harness/reset.sh                              # Reset to blank slate
./harness/submit-design.sh design-tasks/01-initial.md  # Submit design task
./harness/wait-complete.sh                      # Wait for all tasks
./harness/verify.sh                             # Run verification
./harness/report.sh                             # Generate report
```

## Files

- `design-tasks/01-initial.md` — First design task (implement budgetapp)
- `design-tasks/02-enhancement.md` — Enhancement (recurring expenses)
- `reset.sh` — Reset repo to initial state
- `submit-design.sh` — Submit a design task to CC
- `wait-complete.sh` — Poll until all tasks done/failed
- `verify.sh` — Run verification suite
- `run-full-test.sh` — Full orchestrator
- `report.sh` — Generate timestamped report

## Expected Flow

1. Reset → clean slate
2. Submit initial design → Codex updates SPEC.md, creates implementation tasks
3. Opus reviews design PR
4. Implementation tasks execute in parallel (backend + frontend)
5. Sonnet reviews implementation PRs
6. Verification runs (build, API tests)
7. Submit enhancement → repeat cycle
8. Final report generated

## Reports

Reports saved to `harness/reports/YYYY-MM-DD-HHMMSS.md`
