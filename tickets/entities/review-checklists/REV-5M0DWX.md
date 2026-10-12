---
id: REV-5M0DWX
type: review-checklist
title: 'Review: Template picker as dropdown with readable variant labels'
started: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (pre-commit `go test` passed; `npm run test:run` 4146 passed; e2e templates spec 2 passed)
- [x] Lint clean (`just lint`) (golangci-lint 0 issues; eslint 0 errors; vue-tsc clean)
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (N/A: no Go changes)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: no Go changes; frontend has no coverage gate)

**Comment findings.** `just comment-report` lists the advisory rules
(duplication, nil-contract, param-contract, restatement). They are not a merge
gate, but a finding your diff *introduces* should be fixed or suppressed — don't
grow the backlog.

Every rule is a heuristic over prose, so false positives are expected. To
suppress one, prefer the inline form on the declaration line, which travels with
the code and is reviewed in this diff:

```go
func f(p string) {} //commentlint:ignore param-contract  p is contained by Clone
```

Use `.commentlint.yml` (`ignore:` path globs, `allow-phrases:`) only when the
same prose recurs across many sites. A reason is required either way — an
unexplained suppression is a finding nobody can re-evaluate later.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** No RR entities; findings handled inline. Significant:
a native select fires `change` on arrow keys in some browsers, wiping entered
values. Fixed: switching asks first when the form is dirty and restores the
select on cancel. Test gaps for `?template=`, unknown-name fallback and the
confirm path are now covered in `DynamicForm.templates.test.ts`. Re-choosing
the current template no longer resets the form; accepted. Minor: a
separator-only name now shows as written; docs say "Default" only if a base
template exists; stale "pill choice" comment fixed. Label collisions
(`follow-up` vs `follow_up`) are left to the authorable-label follow-up
(DEC-6C1NAA).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: dropdown present only with more than one template (unit + e2e).
2. PASS: `embeddedTemplate` and `?template=` preselect (unit).
3. PASS: choosing applies the template body (unit + e2e).
4. PASS: "Default", "Corrective action", "Follow up" (unit + e2e).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-RKL2ON

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

<!--
Deliberately NOT tracked here: the PR URL and whether CI passed.

Both post-date this checklist. `/pr` requires the ticket to be `done` and
validating clean before it opens the PR, and a `done` review-checklist may have
no unchecked items — so an item asking for the PR URL can only be satisfied by a
PR that does not exist yet. Checking it early would mean asserting "CI passed"
before CI ran, which turns the checklist from evidence into a formality.

GitHub records both authoritatively, and the branch and commit messages carry
the ticket ID, so the ticket-to-PR link is recoverable without duplicating it
here. See TKT-UFV01M. -->
