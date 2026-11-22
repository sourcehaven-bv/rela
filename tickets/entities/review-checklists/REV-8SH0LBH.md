---
id: REV-8SH0LBH
type: review-checklist
title: 'Review: GET /_flows answers 404 on every page of a project without flows'
started: "2026-11-21"
completed: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (go test ./internal/dataentry ./internal/apiwire/...; full vitest suite; vue-tsc clean; the full Go suite runs in CI)
- [x] Lint clean (`just lint`) (golangci-lint on internal/dataentry and internal/apiwire, default and postgres tags; eslint and prettier on the changed frontend files)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (the new capability and the gate have direct tests; CI runs the threshold check)

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky-code-reviewer: one significant finding, addressed; minor and nit findings addressed or wont-fix with reason)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (none)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-JDC8C36, RR-WV8N1Q4, RR-WU1TVBI, RR-M9VS6L6, RR-60J1PSY, RR-MHH4B0W

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- /_schema reports capabilities.flows: PASS (TestSchemaCapabilities_Flows, both states)
- The SPA skips GET /_flows without flows: PASS (useFlowList.test.ts)
- A project with flows behaves as before: PASS (FlowRunsPanel, FlowLists and Sidebar tests with the capability on)
- Documented: PASS (docs/flows.md and GUIDE-flows)

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug)
- [x] User-facing documentation updated (docs/flows.md and GUIDE-flows: the capability and the 404 without flows)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what (minimal message per the owner's rule; the why is in BUG-QTEEQZ4)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (opened with gh pr create after pr-guidance-check)

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
