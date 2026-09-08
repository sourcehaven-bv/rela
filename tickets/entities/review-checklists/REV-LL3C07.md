---
id: REV-LL3C07
type: review-checklist
title: 'Review: sqlite-tagged tests run in CI'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test -tags sqlite` over the newly-gated packages is clean; default build unaffected)
- [x] Lint clean (`just lint`: 0 issues; `just lint-md`: 0 issues)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (no production code changed; no package moved)

## Code Review

- [x] ~~Run `/code-review` command~~ (N/A: the change is one build-tag constraint, two doc comments and a CI step — there is no logic to review. The substantive check is whether the new job catches the defect, done under Acceptance below.)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (none raised)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** none — no production code changed.

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] ~~Test evidence documented in implementation checklist~~ (N/A: evidence is below)

**Acceptance Status:**

- PASS — the failing test passes under `-tags sqlite`. Root cause was the
  fixture, not the warning logic: `warnUndeclaredFaces` probes the store with
  two `CountEntities` calls and is fully backend-neutral, while the fixture
  hand-writes entity markdown that only fsstore reads. The file already
  excluded `postgres` for that exact reason; `sqlite` inherited the gap when
  the backend was added under a bare `!postgres` tag.
- PASS — **the new CI job catches the original defect.** Reverted the build-tag
  fix and ran the job's exact command: `--- FAIL:
  TestBuild_StateRows_WarnAtStartup`. Restored, green again.
- PASS — full sweep of the tree under `-tags sqlite` found exactly one failing
  test, so the fix is complete rather than the first of many. That was worth
  measuring: the bug report predicted further failures, and there are none.
- PASS — `just build-check-tags` clean across all four combinations.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix, no user-facing surface)
- [x] ~~User-facing documentation updated~~ (N/A: no behaviour change for any user)
- [x] ~~Docs-checklist marked as done~~ (N/A)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
