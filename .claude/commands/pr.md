<!-- @managed: claude-workflow v1 -->
# Create PR and Monitor CI

Create a pull request and monitor CI checks until they pass, fixing any issues found.

## Workflow

### 0. Ticket Gate (done-before-PR)

A PR represents finished work, so its ticket/bug must already be `done` **and**
validate-clean *before* the PR is opened. Do this first; do not proceed if it
fails.

1. Identify the ticket/bug this PR is for (from `$ARGUMENTS`, the branch name, or
   the commit messages — they reference an ID like `TKT-XXXX` / `BUG-XXXX`).
2. Check its status with `show_entity` (rela-issues-and-design-tickets). It
   **must** be `status=done`. If it is still `planning` / `in-progress` /
   `review`, the work isn't finished per the workflow — finish it and run
   `/verify <ID>` to transition to `done` first.
3. Confirm the ticket validates clean:

   ```bash
   ./bin/rela validate --project tickets --check cardinality --check properties --check validations
   ```

   This exercises the workflow gates (completed review checklist, no open
   critical/significant review-responses, docs checklist for enhancement/docs
   tickets). Exit code must be 0.

**If the ticket is not `done` or validation fails: STOP.** Report exactly which
gate failed and what's missing (e.g. "review checklist REV-xxxx is not `status:
done`", "TKT-xxxx has an open critical review-response"). Do not push or open the
PR until the ticket is `done` and validation is green.

### 1. Pre-flight Checks

Run local CI checks before creating the PR:

```bash
just ci
```

If local checks fail, fix the issues before proceeding. Common fixes:
- `just lint-fix` for lint errors
- `just fmt` for formatting issues
- Run failing tests and fix the code

### 2. Create Pull Request

Once local checks pass:

1. Check if branch is pushed: `git status`
2. Push if needed: `git push -u origin HEAD`
3. Create PR: `gh pr create --fill` (or with custom title/body)
4. Note the PR URL for monitoring

### 3. Monitor CI Loop

Enter a monitoring loop:

```
while CI checks are pending or failing:
    1. Wait 30 seconds
    2. Check status: gh pr checks
    3. If all passed → report success and exit
    4. If failed → attempt to fix and push
    5. Repeat
```

### 4. Fixing CI Failures

When CI fails, investigate and fix:

1. Get failure details: `gh pr checks --json name,state,description`
2. For lint failures: `just lint-fix && just fmt`
3. For test failures: Run the specific test, fix the code
4. For coverage failures: Add tests to improve coverage
5. Commit fixes and push: `git add -A && git commit -m "fix: CI issues" && git push`

### 5. Report Success

Once all checks pass, report:
- PR URL
- All checks that passed
- Summary of any fixes made

## Notes

- Maximum iterations: 10 (to prevent infinite loops)
- Sleep interval: 30 seconds between checks
- If unable to fix after 3 attempts on same issue, ask user for help
