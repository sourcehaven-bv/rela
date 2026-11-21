---
id: REV-AEFV81D
type: review-checklist
title: 'Review: The Changes list shows raw actors and relation names; History opens empty'
started: "2026-11-20"
completed: "2026-11-20"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (internal/dataentry and the full frontend suite (444 files) pass locally; CI runs the rest)
- [x] Lint clean (`just lint`) (golangci-lint on internal/dataentry: 0 issues; ESLint and vue-tsc clean)
- [x] Comment lint gate clean (`just comment-lint`) (no unresolvable doc links)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: the CI coverage job gates it; every new function has tests)

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (critical review of the diff done inline in the worker session, which may not start agents; four findings recorded)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (none)
- [x] Self-reviewed the diff for unrelated changes (Prettier reformatting of unrelated files reverted; only the touched test file keeps its new line breaks)

**Review Responses:** RR-FSSDLLW (deferred), RR-06QZPK2 (wont-fix), RR-FYVHJYT (addressed), RR-C2LIL6B (wont-fix)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist) (see the implementation checklist IMPL-C66G7EP)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
Actor titles PASS (TestChangeTitles_NamesActorsByTitle, ChangesPanel). Identity labels PASS (changeText cases). Incoming relations PASS (changeText cases). History hidden PASS (EntityDetail.world, useHasHistory). Title on history page PASS (HistoryView subject tests). Docs PASS (audit-log, data-entry, api-reference).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug)
- [x] User-facing documentation updated (docs/audit-log.md, docs/data-entry.md, docs/data-entry/api-reference.md and the guide mirrors)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what (minimal commit message by the owner's rule; the bug carries the why)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A here: the PR follows done, see TKT-UFV01M)

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
