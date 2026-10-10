---
id: REV-LLLH2G
type: review-checklist
title: 'Review: Gantt: horizontal scroll with Now and arrow navigation'
started: "2026-10-09"
completed: "2026-10-10"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

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

**Review Responses:** cranky-code-reviewer found 0 critical, 4 significant (all
addressed: RR-DCTA9Q stale fetch, RR-43FNTF inclusive end, RR-NJWUC5 timeline
cap, RR-YYO7M2 chart height), 10 minor (7 addressed, RR-RLNOI7 and RR-CV05JL
deferred, RR-ZQLQ1N wont-fix) and one grouped nit (RR-1WW5OB, addressed).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** (scroll-and-navigation half; drag criteria moved to
TKT-7W9LKE)

- Wider-than-screen span scrolls sideways, tree column and axis fixed: PASS
(e2e sticky tree-cell position, screenshots).
- Now brings today into view: PASS (e2e centred within 15px; unit centring
with stubbed width).
- Arrows scroll one unit: PASS (e2e ±360px at Month).
- Scroll position survives zoom change: PASS (e2e 1200 → 4000px; unit).
- Docs: PASS (GUIDE-data-entry "Scrolling through time").

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-T5N0IB

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: runs after done, see note below)

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
