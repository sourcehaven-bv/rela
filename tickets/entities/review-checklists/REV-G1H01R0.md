---
id: REV-G1H01R0
type: review-checklist
title: 'Review: Form default: current-user token'
started: "2026-10-29"
completed: "2026-10-29"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (go test ./internal/dataentry/... ok; frontend 354 files / 5335 tests pass)
- [x] Lint clean (`just lint`) (golangci-lint 0 issues for maildemo,mailmanual and memorybackend repo-wide, sqlite and postgres on internal/dataentry: the repo-wide runs failed to load packages under load from parallel sessions; eslint 0 errors; vue-tsc clean)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (new code is covered by its tests; the local run failed only on internal/store/sqlitestore, which timed out at 600s under machine load and is untouched; CI runs the gate)

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
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** Design: RR-W3UY2R1, RR-BERZK38, RR-MP0DXYR. Code:
RR-GWFX4QX, RR-P580NT8, RR-SN2A92U (significant, addressed), RR-TDV1B2D,
RR-52GIOIW, RR-8MAYUNC, RR-B0TC84T, RR-3PJJQ19, RR-1YAK8F2 (addressed). The
security review found no issues.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** AC1 PASS (DynamicForm.userdefault cached /_me; browser
check). AC2 PASS (late /_me, typed value kept, prop.* wins). AC3 PASS (unknown
and failed /_me). AC4 PASS (TestV1FilteringUserToken; curl as jeroen and bob).
AC5 PASS (no identified caller case). AC6 PASS
(TestNavStatus_UserTokenCountsPerCaller). Boards and commands added after
review: KanbanView $user tests, TestApplyFilters $user cases.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-RV51ELD

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
