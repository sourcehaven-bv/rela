<!-- @managed: claude-workflow v1 -->
# Code Review

Perform a thorough review of recent changes using **two agents in parallel**:

- **cranky-code-reviewer** — general quality: architecture, edge cases, idioms, tests.
- **rela-security-reviewer** — security only, against rela's own invariants
  (ACL ceiling, visibility wrappers, `PatchEntity`, cmdexec, mailrender order,
  job/Tx deferral) framed by the OWASP web application checklist.

They are complementary and deliberately non-overlapping: cranky treats security
as one concern among six, which is why a dedicated reviewer carries the
project-specific rules that a generalist skims past.

## Instructions

1. **Identify the scope**: Use `git diff` or `git log` to identify what code was
   changed for the current work item. Determine the merge base against `develop`
   so the diff is the branch's work, not unrelated history.

2. **Invoke both agents in a single message** so they run concurrently. Give each
   the same scope (changed files + the diff).

   - `cranky-code-reviewer`: architecture, edge cases, error handling, tests,
     performance, maintainability.
   - `rela-security-reviewer`: security only. It scopes itself by which surfaces
     the diff touches (ACL, visibility, entitymanager, dataentry handlers,
     cmdexec/attachments, mail, jobs, pgstore, lua/predicate, state, `frontend/`)
     and reports which categories it skipped.

   If the diff touches **no** security surface (docs-only, test-only), the
   security agent will say so in one line — that is a valid result, not a
   failure. Run it anyway; deciding "this isn't security-relevant" without
   looking is exactly how a surface gets missed.

3. **Merge the findings.** Both agents may flag the same line from different
   angles — keep the security framing when they overlap, and de-duplicate.
   On severity disagreement, take the higher.

4. **For each finding**, create a `review-response` entity with:
   - `title`: Brief description of the finding
   - `finding`: Detailed explanation of the issue
   - `severity`: `critical` | `significant` | `minor` | `nit`
   - `status`: `open`

   Link each to the ticket/bug via `has-review-response`. Prefix the `finding`
   of a security item with `[security]` so it is identifiable when triaging.

5. **Summarize findings** by severity for the user, security items first.

## Severity Guide

| Severity | Criteria | Must Fix? |
|----------|----------|-----------|
| critical | Security vulnerabilities, data loss risk, crashes | Yes |
| significant | Bugs, missing error handling, architectural issues | Yes |
| minor | Code quality, missing tests, minor edge cases | Should fix |
| nit | Style, naming, documentation | Optional |

The security agent maps onto this same scale. An invariant violation is at least
`significant` even without a demonstrated exploit — those rules encode incidents
that already happened.

## After Review

- Address critical and significant findings before completing work
- Minor/nit findings can be deferred with documented reason
- The ticket cannot reach `done` with open critical/significant review-responses
