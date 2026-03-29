# Implement Ticket/Bug

Continue implementation on the current ticket or bug.

## Prerequisites

Verify the ticket/bug is in `in-progress` status with a completed planning/analysis checklist.
The implementation-checklist was created automatically when status changed to `in-progress`.

## Implementation Checklist

Work through the implementation checklist (auto-created by automation):

### Development
- [ ] Write tests first (TDD where applicable)
- [ ] Implement the happy path
- [ ] Handle edge cases identified in planning
- [ ] Add error handling

### Quality
- [ ] Code follows project style
- [ ] No security issues introduced
- [ ] Performance is acceptable
- [ ] No debug code left behind

## Process

1. For each piece of work:
   - Write the test first
   - Implement the code
   - Verify the test passes
   - Check the checklist item

2. Run `just test` periodically to ensure no regressions

3. When implementation is complete:
   - Mark implementation checklist as `done`
   - Transition to `review` status (automation creates review-checklist)

4. Notify the user that implementation is ready for review
