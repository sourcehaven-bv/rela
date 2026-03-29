# Pre-Completion Review

Perform self-review before marking a ticket/bug as done.

The review-checklist was created automatically when status changed to `review`.

## Automated Checks

Run and verify:

```bash
just test        # All tests pass?
just lint        # Lint clean?
just coverage-check  # Coverage maintained?
```

Mark each as done in the review checklist.

## Code Review

Run `/code-review` to invoke the cranky-code-reviewer agent. This will:
- Perform thorough code review of changes
- Create `review-response` entities for each finding
- Link findings to the current ticket

**Required**: All critical and significant findings must be addressed before completion.

## Manual Review

- [ ] Self-review the diff (`git diff`)
- [ ] Commit messages explain the why, not just the what
- [ ] No unrelated changes included
- [ ] No debug code, console.logs, or TODOs left behind

## Final Verification

- [ ] Acceptance criteria from planning checklist are met
- [ ] Works as expected when tested locally

## Documentation (for enhancements/docs tickets)

If this is an enhancement or docs ticket:

1. Create `docs-checklist` from template
2. Link via `has-docs` relation
3. Update relevant documentation:
   - [ ] Comments where logic isn't obvious
   - [ ] README if applicable
   - [ ] CLAUDE.md if new patterns
   - [ ] CLI help text if commands changed
   - [ ] Changelog entry

## Completion

Once all checks pass:

1. Mark review checklist as `done`
2. Mark docs checklist as `done` (if applicable)
3. Transition ticket/bug to `done`
4. Run `analyze_validations` to verify completion

Output a summary of what was completed and any notes for the user.
