<!-- @managed: claude-workflow v1 -->
# Create PR and Monitor CI

Create a pull request and monitor CI checks until they pass, fixing any issues found.

## Workflow

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
