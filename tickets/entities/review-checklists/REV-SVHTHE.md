---
id: REV-SVHTHE
type: review-checklist
title: 'Review: Free-text search judges hidden fields one hit at a time'
started: "2026-10-17"
completed: "2026-10-17"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (only the known `TestChromeStyle_TargetsShippedClasses` fails; the postgres and sqlite suites pass)
- [x] Lint clean (`just lint`) (default, postgres, sqlite and memorybackend tag sets)
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
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (RR-FB98MQ, RR-ZS0CD4)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-FB98MQ (significant, addressed), RR-ZS0CD4
(significant, addressed), RR-K1JELW (minor, deferred to BUG-YG126C), RR-67SIP4
(nit, wont-fix).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Field verdicts are batched per page, not per hit: PASS.
`TestQueryBudget_ListFreeTextUnderFieldHiding` shows zero entity loads and the
same `MatchingFaces` count at 10 and 50 hits, for a match on an unconditional
grant and on a conditional one.
- Hits are loaded per page without bodies: PASS.
`TestListBodies_QueryShapesKeepTheBound` reads zero bodies for a narrow and a
broad `?q=`.
- Every backend agrees with `FieldVerdicts`: PASS.
`storetest.RunFieldScopeSearchTests` and
`TestSearchFieldScope_AgreesWithFieldVerdicts`.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix; the load-time `when:` rule is in CLAUDE.md)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the fix ships in the arch/acl-store-adapters PR, opened once for the whole branch)

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
