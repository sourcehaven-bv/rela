---
id: REV-31ZMSJ3
type: review-checklist
title: 'Review: Point-in-time (as-of) reads of a world'
started: "2026-11-18"
completed: "2026-11-18"
status: done
---

---
id: REV-31ZMSJ3 type: review-checklist title: 'Review: Point-in-time (as-of)
reads of a world' started: "2026-11-18" status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): internal/... default, sqlite-tagged dataentry/store/affordances/lua/search/visibility, pg conformance, frontend typecheck and unit tests
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

**Review Responses:** RR-8624OST, RR-913TVW0, RR-B7IKG5Y, RR-CGXYD28,
RR-DBNAHIM, RR-E9N18SO, RR-KSJRUY2, RR-O83GIMI, RR-LJKSOC1 (all addressed)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- PASS: pg and sqlite answer at an instant; fs and memory refuse (`storetest.RunInstantTests`, `RunInstantUnsupportedTests`).
- PASS: writes refused under an instant (store `WritesAreRefused`, `TestAsOf_SQLite_WritesAreRefused`).
- PASS: created, deleted, renamed after; face published after (`RunInstantTests` subtests).
- PASS: batched, scale-free page (`HeaderPageIsScaleFree`).
- PASS: today's ACL; access lost since fails closed (`ACLGateReadsTodaysEdges`, `TestAsOf_SQLite_AccessLostSinceFailsClosed`); relation-based field grants fail closed (`TestAsOf_SQLite_RelationConditionedFieldFailsClosed`).
- PASS: a gate folded onto a `related(...)` traversal reads today, the traversal reads the instant (`GateFoldedIntoATraversalReadsToday`, both backends and the Go fallback); `?as_of=` needs `history:read` (`TestAsOf_SQLite_NeedsHistoryRead`); no leases or OAuth at an instant (`TestAsOf_SQLite_NoLeasesInThePast`, `TestOAuth_RefusedAtAPastInstant`).
- PASS: lists, detail, views, dashboard cards, search, documents, gantt (`TestAsOf_SQLite_*`); views follow worlds (`TestViews_FollowTheWorld`, `TestGantt_FollowsTheWorld`).
- PASS: guide with an SoA audit example (GUIDE-point-in-time-reads, `just docs`).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-0VWT6WQ

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
