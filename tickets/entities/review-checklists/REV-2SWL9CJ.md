---
id: REV-2SWL9CJ
type: review-checklist
title: 'Review: Content-free keyset pages for search with no text'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A: repo-wide run excluded for this task; touched packages pass with -race instead: store, storetest, memstore, fsstore, search/..., archguard, visibility, mcp on the default build; pgstore and search/... with -tags postgres; sqlitestore with -tags sqlite. All re-run after the last code commit.)
- [x] Lint clean (golangci-lint serially on the changed packages, default build plus sqlite and postgres tags: 0 issues; `just arch-lint` and `just plimsoll` clean)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: needs the repo-wide run excluded for this task; CI enforces the floors. The change adds tests to every package it touches.)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (none raised)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-7YENYHZ, RR-3UM75Y9, RR-ZD7JAYO, RR-3OWEXRR,
RR-MQC850P, RR-5DST37L, RR-5KAPRJC addressed; RR-P4CW4HL deferred to TKT-5OWNZAU
(pre-existing gap: BodyWatch does not count bodies the GraphQueryHeaders
fallback reads).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: `TestSearch_NoTextReadsNoBodies` and `TestSearchVisible_NoTextReadsNoBodies` report zero bodies; with `typePages` reverted to `ListEntitiesPage` both fail.
2. PASS: `TestSearch_NoTextHeaderPagesMatchEntityPages`, `TestSearch_NoTextIsScaleFree` and `TestSearch_NoTextAdmitsWholeFamilies` pass.
3. PASS: `storetest.RunHeaderPageTests` passes on memstore, fsstore (fallback), pgstore and sqlitestore.
4. PASS: header-page cases in both `TestFaceReadExplain` suites select no content column and use the type/id/face index.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated (no user-facing change; `.claude/rules/collection-reads.md` names the new read)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-TQA86WC

## Final Checks

- [x] Commit message explains the why, not just what (minimal subjects with the ticket id, per the user's instructions; the why is in TKT-Q918DFR)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (PR opened with `gh pr create` against the stacked base; CI monitoring is out of scope for this task)
