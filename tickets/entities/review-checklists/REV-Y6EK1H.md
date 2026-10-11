---
id: REV-Y6EK1H
type: review-checklist
title: 'Review: Entity references to faced types never resolve as mentions'
started: "2026-10-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): exit 0; frontend `npm run test:run` 4044 passed
- [x] Lint clean (`just lint`): exit 0; `just arch-lint` OK; frontend lint 0 errors (warnings only in untouched files)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): the run under concurrent load timed out at 10m in `dataentry` and `sqlitestore` (no assertion failures); rerun alone with `-race`: dataentry 86.2% (floor 55), sqlitestore 85.5%

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
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed: RR-2KZCA3 addressed; RR-QD5JRS deferred with reason (editor links, out of agreed scope)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-2KZCA3, RR-QD5JRS, RR-I1TV5H, RR-2ZQLU8, RR-ES999F,
RR-89X726, RR-TZOWT1, RR-J691YT, RR-3FJYMV, RR-8W0UVY

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. Faced reference resolves with the served face's title: PASS
   (`TestMentions_FacedTargetsResolveInTheRequestWorld`, manual Atlas config).
2. Draft-only entity resolves to its draft: PASS (same test, manual
   `PROCEDURE-WCTK`).
3. Denied prime falls through; nothing readable → no mention: PASS (same
   test; `TestMentions_DeniedWorldServesNoMention` for a denied world).
4. Unfaced references unchanged: PASS (existing mention tests).
5. Redacted title → id: PASS (`TestMentions_FacedTargetRedactsHiddenTitle`,
   both routes).
6. Store fault drops mentions, response 200: PASS
   (`TestCollectMentions_StoreErrorDegradesToNoMentions`).
7. World on the link: PASS (`markdown.test.ts`, `EntityDetail.world.test.ts`,
   headless Chromium on the Atlas config); explicit `?world=` resolution:
   `TestMentions_ResolveInTheWorldTheRequestNames`.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-AJ6F6F

## Final Checks

- [x] Commit message explains the why, not just what
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
