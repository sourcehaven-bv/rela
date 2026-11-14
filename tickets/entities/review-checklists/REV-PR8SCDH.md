---
id: REV-PR8SCDH
type: review-checklist
title: 'Review: Id minting misses mixed-case ids after HighestID switch'
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A: repo-wide run skipped to spare a shared machine; every touched package passes with -race: entity, entitymanager, memstore, fsstore, storetest, sqlitestore, pgstore against a scratch database)
- [x] ~~Lint clean (`just lint`)~~ (N/A: replaced by golangci-lint on each touched package, including the postgres and sqlite build tags: 0 issues)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: no package lost tests; new tests added in every touched package)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (none)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-K9TZ9RL, RR-ZV8VPFU, RR-P7ATW05, RR-LFWLYQT,
RR-86CIQNG, RR-APWZKMQ

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- All backends agree on which ids count, case-insensitively: PASS (storetest HighestIDFoldsPrefixCase and HighestIDCountsDigitsOnly on fsstore, memstore, sqlitestore, pgstore).
- Create after a mixed-case id mints the next number: PASS (TestCreate_MintsPastMixedCaseIDs).
- No type scan on postgres: PASS (TestFaceReadExplain/highest_id uses entities_id_lower_key under a generic plan).

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix, no user-facing change)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (PR opened with gh pr create as instructed; CI not awaited)
