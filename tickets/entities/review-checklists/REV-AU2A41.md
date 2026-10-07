---
id: REV-AU2A41
type: review-checklist
title: 'Review: Relation-conferred type@face grant denies explicitly addressed faces at the row gate'
started: "2026-10-07"
completed: "2026-10-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): `go test ./internal/dataentry/` ok; the diff is one test in that package. Locally `just test` fails only on cmd/rela-desktop TestChromeStyle_TargetsShippedClasses (stale SPA build in internal/dataentry/static; unrelated, passes in CI)
- [x] Lint clean (`just lint`): golangci-lint 0 issues on internal/dataentry
- [x] Comment lint gate clean (`just comment-lint`): no unresolvable doc links
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: test-only change; no production code changed)

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

- [x] Run `/code-review` command (cranky-code-reviewer + rela-security-reviewer)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (RR-348WSJ, RR-POSC97, RR-FE83RQ)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-348WSJ, RR-POSC97, RR-FE83RQ (significant, addressed);
RR-HBQ4HC, RR-XJY53H, RR-GI8YWD (minor, addressed); RR-I19I38 (minor, deferred
with reason); RR-9H2TJ9 (nit, addressed)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist (IMPL-8DXF8U)

**Acceptance Status:**

- PASS: alice reaches TKT-001@draft on the entity and comments routes (200) via the owned-by conferred ticket@draft grant. TestComments_FaceLimitedConferredGrant; fails on cf0fe87e^ (pre-#1753) with 404, passes on develop.
- PASS: no widening. TKT-002@draft (no edge) and bob (no relation) get a 404 whose body equals a real not-found.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug)
- [x] ~~User-facing documentation updated~~ (N/A: bug)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A here: /pr runs after done, see TKT-UFV01M)

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
