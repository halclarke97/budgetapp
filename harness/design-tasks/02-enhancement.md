# Design: Recurring Expenses Enhancement

Add recurring expense patterns to the existing BudgetApp.

## Objective

Update docs/SPEC.md with recurring expense requirements and create implementation task manifests.

**This is a DESIGN task only — do NOT write implementation code.**

## Deliverables

### 1. Update docs/SPEC.md

Add recurring expense specification including:

**Backend:**
- RecurringPattern data model (frequency, start/end dates, next run)
- CRUD endpoints for recurring patterns
- Upcoming preview endpoint
- Sweep function for generating due expenses
- Monthly edge-case handling (month-end dates)

**Frontend:**
- Recurring pattern management UI
- Controls for pause/resume/delete
- Upcoming expenses preview panel

### 2. Create Implementation Task Manifests

Create `.cc/tasks/*.json` files for the implementation:

Each manifest should include:
- `id`: unique task identifier
- `title`: descriptive title  
- `type`: "implementation"
- `description`: what to build
- `paths`: files this task will create/modify
- `depends_on`: dependencies on other tasks
- `acceptance_criteria`: verification steps

**Important:** Set `status: "todo"` — tasks execute after design merges.

## Acceptance Criteria

- [ ] docs/SPEC.md updated with recurring expense requirements
- [ ] .cc/tasks/*.json contains implementation task manifests
- [ ] All manifests have status: "todo"
- [ ] No implementation code (no backend/, frontend/, etc.)
