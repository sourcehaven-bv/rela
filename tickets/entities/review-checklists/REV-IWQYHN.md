---
id: REV-IWQYHN
type: review-checklist
title: 'Review: Seed the autosave merge base in DynamicForm and delete the dead EntityCache.etag'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — frontend suite green: 123 files / 2008 tests
- [x] Lint clean — `npm run lint`: 0 errors (114 pre-existing warnings, none in
      the changed files beyond DynamicForm's pre-existing `max-lines`)
- [x] Comment lint gate clean — `just comment-lint`: no unresolvable doc links
      across 11066 comments
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: frontend-only change;
      the Go coverage floors cover `internal/**` and the frontend has no
      coverage enforcement — see CLAUDE.md)

Also run: `npm run typecheck` (vue-tsc) clean, and `go build ./...` clean.

Note on the local environment: `node_modules` was missing `lucide-vue-next`
(declared in `package.json` but not installed), which failed 5 test FILES to
import before any of this work. `npm install` resolved it; it is not related to
this change.

## Code Review

- [x] Reviewed the diff against the finding that motivated it (RR-VQQQ60)
- [x] All critical review-responses addressed — RR-VQQQ60 is `addressed`
- [x] All significant review-responses addressed — none apply to this ticket
- [x] Self-reviewed the diff for unrelated changes — two files plus one test
      file; no drive-by edits

**Review Responses:** RR-VQQQ60 (critical, addressed — the DynamicForm half of
this ticket). The remaining nine findings belong to TKT-2VDVHF.

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented

**Acceptance Status:**

- **Merge base is seeded from `loadEntity`** — PASS. `DynamicForm.test.ts`
  "suppresses a PATCH that would rewrite the loaded server value": retyping the
  value the server already holds issues no PATCH, which is only possible if
  `lastSeenServer` was populated before the edit.
- **Genuine edits still save** — PASS. "still PATCHes a value that genuinely
  differs from the server" asserts the call and its exact patch body, so the
  suppression above cannot be over-broad.
- **The base does not alias the form copy** — PASS. "does not alias the form
  copy — an edit-then-revert is suppressed" only holds if the baseline stayed
  still while `formData` moved. The snapshot clones `properties`/`relations` and
  is `Object.freeze`d so an accidental write to the baseline fails loudly.
- **Tests actually pin the behaviour** — PASS by mutation testing: deleting the
  `recordServerBaseline(entity)` call fails 2 of the 3 new tests. Without that
  check the assertions would be indistinguishable from ones that pass vacuously.
- **Dead `EntityCache.etag` removed** — PASS. No path wrote or read it; the full
  suite and `vue-tsc` are green after removal, which is the evidence that
  nothing depended on it.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: internal
      refactor + bug fix, `kind: refactor`; no user-facing surface changes)
- [x] ~~User-facing documentation updated~~ (N/A: no behaviour a user can
      observe beyond the removal of a redundant PATCH)
- [x] ~~Docs-checklist marked as done~~ (N/A: none created)

The reasoning that would otherwise be documentation lives in the code as
comments — the `EntityCache` comment states the "a cached entity has no
meaningful ETag" invariant, and `snapshotServerState` documents why the clone
and the pre-mutation ordering are load-bearing.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use — the snapshot seam
      (`recordServerBaseline`) is the same contract `SectionEditForm` and
      `EntityDetail` already use

## Pull Request

- [x] PR opened against `develop`
