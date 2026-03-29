<!-- @managed: claude-workflow v1 -->
# Code Review

Perform a thorough code review of recent changes using the cranky-code-reviewer agent.

## Instructions

1. **Identify the scope**: Use `git diff` or `git log` to identify what code was changed for the current work item.

2. **Invoke the cranky-code-reviewer agent** to review the changes, focusing on:
   - Security vulnerabilities (injection, path traversal, auth bypass, etc.)
   - Edge cases and error handling gaps
   - Missing or insufficient tests
   - Architectural concerns and code smells
   - Performance issues
   - Code quality and maintainability

3. **For each finding**, document with:
   - `title`: Brief description of the finding
   - `finding`: Detailed explanation of the issue
   - `severity`: `critical` | `significant` | `minor` | `nit`
   - `status`: `open`

4. **Summarize findings** by severity for the user.

## Severity Guide

| Severity | Criteria | Must Fix? |
|----------|----------|-----------|
| critical | Security vulnerabilities, data loss risk, crashes | Yes |
| significant | Bugs, missing error handling, architectural issues | Yes |
| minor | Code quality, missing tests, minor edge cases | Should fix |
| nit | Style, naming, documentation | Optional |

## After Review

- Address critical and significant findings before completing work
- Minor/nit findings can be deferred with documented reason
