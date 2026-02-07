# Design: BudgetApp Initial Implementation

Create the design specification and implementation task plan for a simple expense tracker.

## Objective

Update docs/SPEC.md with complete requirements and create implementation task manifests.

**This is a DESIGN task only — do NOT write implementation code.**

## Deliverables

### 1. Update docs/SPEC.md

Define the full specification including:

**Backend (Go):**
- REST API endpoints (expenses CRUD, categories, stats, healthz)
- Data models (Expense, Category)
- JSON file storage approach

**Frontend (React + Vite + Tailwind):**
- Component structure
- UI/UX requirements
- API integration

### 2. Create Implementation Task Manifests

Create `.cc/tasks/*.json` files that define implementation tasks:

Each manifest should include:
- `id`: unique task identifier
- `title`: descriptive title
- `type`: "implementation"
- `description`: what to build
- `paths`: files this task will create/modify
- `depends_on`: task dependencies (if any)
- `acceptance_criteria`: how to verify completion

**Important:** Set `status: "todo"` — these tasks will be executed after this design merges.

## Acceptance Criteria

- [ ] docs/SPEC.md contains complete backend and frontend requirements
- [ ] .cc/tasks/*.json contains implementation task manifests
- [ ] All manifests have status: "todo"
- [ ] No implementation code (no backend/, frontend/, etc.)
