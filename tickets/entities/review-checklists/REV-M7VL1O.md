---
id: REV-M7VL1O
type: review-checklist
title: 'Review: Atlas taken list costs thousands of queries per request and the SPA refetches it in bursts'
started: "2026-10-15"
completed: "2026-10-16"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (go test ./... clean; vitest composables 713 passed; storetest PeerIDs case passes on mem, fs, sqlite, postgres)
- [x] Lint clean (`just lint`) (0 issues on every build tag)
- [x] Comment lint gate clean (`just comment-lint`) (no unresolvable doc links)
- [x] Coverage maintained (`just coverage-check`) (total 84.2%, all floors pass)

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky + rela-security-reviewer)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (RR-AZAUXS, RR-NANY6J)
- [x] Self-reviewed the diff for unrelated changes (135 golden diffs touch only reads/bodies counters)

**Review Responses:** RR-26YRHK, RR-AZAUXS, RR-NANY6J, RR-S7N1IU, RR-9WI0W4,
RR-AWR6K8, RR-E8L9OJ, RR-ESHR4I, RR-2CG2U7, RR-98FO5N, RR-MVLVXE, RR-JCT7N0,
RR-6P77J5, RR-60LHXC, RR-6E7TIU, RR-ZVRX1D, RR-HSAKHS, RR-K5X56N

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist (IMPL-DU3G6Y)

**Acceptance Status:**
- PASS: list reads do not grow with rows under a role-relation policy (8 budget tests compare 10 and 50 rows under realisticBudgetPolicy; with the prime disabled they fail at 1,003 to 2,202 reads).
- PASS: Atlas on postgres: plain list 15 queries, mijn_taken include=* 21, all_taken include=* world=actueel 20, detail include=* 22 (was 1,814 / 4,026 / 7,416). 70 concurrent all_taken: all 200, max 0.46 s.
- PASS: PrimeLocalRoles matches the per-entity walk (differential over random graphs).
- PASS: hidden tab defers refetches; visible tab refetches once after jitter (visibleRefetch.test.ts, useLiveSubscription.test.ts).
- PASS: access log names the variant as a query shape with no values (TestQueryShape).
- PASS: GET over --query-ceiling logs a warning naming the top statement; e2e fails on it (TestRequestStats_QueryCeiling, e2e fixtures).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug, not an enhancement)
- [x] User-facing documentation updated (docs/server-security.md: query shape and --query-ceiling; docs/acl-security.md: conferring relation direction)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug, no docs-checklist)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what (minimal message per user preference; the ticket carries the why)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (done after the ticket reaches `done`; see TKT-UFV01M)

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
