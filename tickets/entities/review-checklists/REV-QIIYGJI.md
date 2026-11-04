---
id: REV-QIIYGJI
type: review-checklist
title: 'Review: Finish next actions: every page surface, context-aware sources, per-view and global placement config'
started: "2026-11-03"
completed: "2026-11-03"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): the full run's one failure (archguard ParseRef rule) is fixed. sqlitestore timed out twice under `-race` on a loaded machine and passes alone in 35s; the coverage run below used `-timeout 30m`
- [x] Lint clean: golangci-lint (serial) on the changed packages, frontend eslint and typecheck, arch-lint, plimsoll
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained: `go-test-coverage` floors pass on the full `-race -shuffle` profile

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

- [x] Run `/code-review` command: cranky-code-reviewer and rela-security-reviewer
- [x] ~~All critical review-responses addressed~~ (N/A: none were critical)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-24AKOWM, RR-3ZR3OQT, RR-46X1JXM, RR-6NFB3C, RR-7E4S6YW,
RR-896QQSV, RR-8EOIXR2, RR-8P25KJ, RR-AT917U, RR-B3LV0BI, RR-BZZ0FV, RR-C9R3KO,
RR-CNTE7L, RR-DZH7SDW, RR-E2GQGQV, RR-I5MW0B2, RR-JAZT7D, RR-JSNHRW0, RR-K07C6P,
RR-KDVGIMQ, RR-KVX38T3, RR-N24USY, RR-NGL8PU, RR-OMC3F8T, RR-QQ3QIX, RR-QSKDTL,
RR-RWF2IR, RR-S3UEQKB, RR-TE8C7F, RR-TN7V5R, RR-UVT5IK, RR-VPMT3PV, RR-YH4PFK4,
RR-YPX50WD, RR-ZKSM81R. One is deferred (the page-watcher closure) and one is
wont-fix (the plimsoll directives), both with reasons.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist (IMPL-YIXKCDP)

**Acceptance Status:**

1. PASS. The card is mounted at the App root. e2e shows it on a list, above a board and on an entity page; the route-to-placement mapping for documents is unit-tested against the real router.
2. PASS. Handler tests: the context source fires on the entity page for that entity with the real condition matcher, and not on a list or another type's page.
3. PASS. A hidden entity's answer is byte-identical to a nonexistent id's; field redaction is pinned too.
4. PASS. `TestNextAction_PlacementSwitches` and the config tests; the chip and card share the singleton, so both go quiet.
5. PASS. `TestNextAction_VisibleOnPlacesASource`, `TestNextActionSource_VisibleOnSurface`.
6. PASS, with one change. Load rejects unknown kinds, views and an empty `visible_on`; an unknown surface is a 400. An unknown view answers empty and logs at Debug, not Warn, after the security review (RR-JSNHRW0).
7. PASS. Engine `WithCurrent` tests, composable tests and the e2e navigation test; pages now also keep their own last suggestion.
8. PASS. Composable test with an out-of-order slow answer.
9. PASS. Defaults tested in `TestNextActionsEnabled` and the handler switch table.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-ZLZEO6N

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the request was to commit; a PR is opened separately)

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
