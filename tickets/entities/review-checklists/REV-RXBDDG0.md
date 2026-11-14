---
id: REV-RXBDDG0
type: review-checklist
title: 'Review: Gated paged reads for MCP list_entities and search with no text'
started: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A locally: the full suite is not run on this machine by agreement; every touched package passes, with `-race`, plus pgstore conformance and the dataentry goldens on default, sqlite and postgres. CI runs the full suite)
- [x] Lint clean (golangci-lint on every changed package, default and postgres tags; `just arch-lint`, `just plimsoll` clean)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: needs the full suite; CI enforces the floors, and every new function has tests)

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

- [x] Run `/code-review` command (cranky-code-reviewer and rela-security-reviewer on the diff)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed (the body-read finding is deferred to TKT-Q918DFR with a reason)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-2RHGT02 RR-39LZJBB RR-41XU345 RR-BQK4ILQ RR-D49HNSQ RR-DI26JHJ RR-DL63RJ1 RR-P0AVMPR RR-RRRW3K0 RR-W1H1HAJ RR-X3NDTPT RR-XC3DP2C RR-YEX0H73 RR-ZAQCITV

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: TestHandleListEntities_ScaleFree.
2. PASS: TestScriptReader_ListEntityPage (memstore and postgres).
3. PASS: TestSearch_NoTextIsScaleFree, TestSearchVisible_NoTextIsScaleFree.
4. PASS: allowlist entries removed; archguard UnpagedRead tests pass.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated (GUIDE-mcp-server)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-0EPYYG4

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR is opened with `gh pr create` against the stacked base; CI is not watched, by request)

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
