---
id: REV-4YW9T95
type: review-checklist
title: 'Review: A body line starting with a conflict marker makes the file unreadable'
started: "2026-11-05"
completed: "2026-11-05"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (go test on every touched package and its consumers: affordances, entitymanager, dataentry, store, markdown, conflict, mcp, appbuild, cli, lua, archguard)
- [x] Lint clean (`just lint`) (golangci-lint on the changed packages: 0 issues; just arch-lint: OK)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: CI enforces the package floors; every new function has tests)

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky-code-reviewer and rela-security-reviewer)
- [x] All critical review-responses addressed (none found)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-9QVT0LM, RR-TRNN2CC (significant, addressed);
RR-ZKGIR6J (minor, addressed); RR-NCBMCTK, RR-OPB6N22 (nit, addressed);
RR-H5GSRPG (nit, wont-fix)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist) (bug-analysis checklist fix plan)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS: a lone marker line and an indented example
round-trip on every backend (storetest
RoundTripsBodyDocumentingConflictMarkers); a complete block is still a conflict,
fenced or not (TestConflictBlocks).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] User-facing documentation updated (GUIDE-acl-overview)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (PR opened after done; CI not polled by request)

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
