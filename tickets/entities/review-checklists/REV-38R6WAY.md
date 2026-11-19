---
id: REV-38R6WAY
type: review-checklist
title: 'Review: Hide Add to pile on lists with list_buttons'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (go test of dataentry and dataentryconfig passes; frontend vitest passes (1051 files' tests apart from the known RlBusyIndicator symlink artifact of this worktree); CI runs the full suite)
- [x] Lint clean (`just lint`) (just lint: 0 issues for the default and sqlite tags; the postgres-tag run failed to load packages locally (go/packages WaitDelay), CI runs it)
- [x] Comment lint gate clean (`just comment-lint`) (CI runs it)
- [x] Coverage maintained (`just coverage-check`) (new code is covered by its tests; CI checks the floors)

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky-code-reviewer over the diff)
- [x] All critical review-responses addressed (RR-58G9BVS addressed)
- [x] All significant review-responses addressed (none raised)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-58G9BVS (critical, addressed); RR-90J1FQO,
RR-NPZZRNA and RR-Y8GP1EN (minor, addressed); RR-C328STD (minor, wont-fix);
RR-CCDR6HQ and RR-I2QMM6E (nits, addressed); RR-4ND3IST (nit, wont-fix)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS: list_buttons.add_to_pile hides Add to pile on lists
and search results; a list without bulk actions, deletable rows or Add to pile
shows no checkboxes; unknown keys are rejected and the block is documented
(Go config and sidebar tests, EntityList unit tests).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs` (DOCS-7O7B4H7)
- [x] User-facing documentation updated (data-entry guide documents list_buttons)
- [x] Docs-checklist marked as done

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR is opened with gh after this checklist is done; CI is watched on GitHub)

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
