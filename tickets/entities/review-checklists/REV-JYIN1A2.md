---
id: REV-JYIN1A2
type: review-checklist
title: 'Review: Link style, phone layout, sidebar title and empty list text'
started: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test` on openapi, apiwire, dataentry, dataentryconfig and configedit; vitest on lists, configure, widgets and dashboard tests; vue-tsc; the full suite runs in CI)
- [x] Lint clean (eslint on the changed files; golangci-lint runs in CI)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained~~ (N/A locally: new code has unit tests; CI checks the floors)

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
- [x] All significant review-responses addressed (RR-6L0U840, RR-DQ19IN7, RR-0YGJO8E, RR-51GFZWP)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-6L0U840, RR-DQ19IN7, RR-0YGJO8E, RR-51GFZWP, RR-6ZFKMAG, RR-4VCONG4, RR-Q2BQSSW, RR-UYR1JEQ, RR-MK1DKEG, RR-QFS9OGO

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- Link style and name: PASS (UrlWidget.test.ts, browser check)
- List tools on a phone, no band, no shadow: PASS (mobile-layout.spec.ts)
- Dashboard table cards stacked, short values on one line: PASS (DashboardView.test.ts, dashboardCards.test.ts)
- Menu button without a box: PASS (browser check at 375px)
- Workspace name wraps: PASS (browser check)
- Plural label in empty and search text: PASS (EntityList.empty.test.ts, TestToV1EntityType_LabelPlural)
- Per-list empty_message: PASS (EntityList.empty.test.ts, TestList_EmptyMessage, screenEdits.test.ts)
- Documented: PASS (data-entry.md, metamodel.md and their guides)

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug)
- [x] User-facing documentation updated (docs/data-entry.md, docs/metamodel.md and their guides)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] PR created with `gh pr create` after the bug is done; CI watched there

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
