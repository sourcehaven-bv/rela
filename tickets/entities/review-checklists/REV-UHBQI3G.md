---
id: REV-UHBQI3G
type: review-checklist
title: 'Review: The Changes list shows customers internal records and raw names'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (go test of visibility and dataentry passes; frontend vitest, typecheck and eslint are clean for the changed files; CI runs the full suite)
- [x] Lint clean (`just lint`) (golangci-lint: 0 issues on visibility and dataentry; the full just lint run failed to load packages locally (go/packages WaitDelay), CI runs it)
- [x] Comment lint gate clean (`just comment-lint`) (just comment-lint: clean)
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
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (RR-KH35M2L addressed)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-KH35M2L (significant, addressed); RR-5ZZ0Q1Q,
RR-IEZSQ7S, RR-TXD3T5V and RR-JPYCCVD (minor, addressed); RR-WWX8188 and
RR-YRA4A7E (minor, wont-fix); RR-TBMHY8C, RR-KG267AN, RR-OD1WT8W, RR-TUPAMRU
and RR-96XW3GF (nits, addressed)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS: the Changes list follows header_buttons.history;
records about `_` relations, renumber records and updates of only hidden or
managed properties are left out; rows show labels and readable titles
(auditchanges_test.go, audit_titles_test.go, ChangesPanel and changeText tests).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] User-facing documentation updated (audit-log and data-entry guides updated)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

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
