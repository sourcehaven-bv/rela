# Checklist Status

Check the workflow progress for a ticket or bug.

## Usage

If $ARGUMENTS is provided, look up that specific ticket/bug ID. Otherwise, ask the user which ticket/bug to check.

## Process

1. Fetch the ticket/bug entity
2. Fetch all linked checklists:
   - `has-planning` or `has-bug-analysis`
   - `has-implementation`
   - `has-review`
   - `has-docs`

3. For each checklist, analyze the markdown body:
   - Count `- [x]` (completed items)
   - Count `- [ ]` (pending items)
   - List `- [x] ~~...~~` (skipped items with reasons)

4. Determine current workflow stage based on ticket/bug status

5. Identify what's blocking the next status transition

## Output

Display a progress summary:

```
Ticket: TKT-123 - Add dark mode toggle
Status: in-progress

Checklists:
  Planning (PLAN-45): done (9/9 items)
  Implementation (IMPL-12): in-progress (4/8 items)
    Pending:
    - [ ] Handle edge cases
    - [ ] Add error handling
    - [ ] No security issues
    - [ ] No debug code left
  Review: not created yet

Next: Complete implementation checklist, then transition to review
```
