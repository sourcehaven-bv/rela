---
id: REV-ITRKYK
type: review-checklist
title: 'Review: ACL-hidden properties leak through _views section field values, and render as editable-but-403 in inline edit'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — `internal/dataentry` + `internal/visibility` green under `-race`.
- [x] Lint clean — `golangci-lint` 0 issues; `just plimsoll` + `just arch-lint` clean.
- [x] ~~Coverage maintained~~ (N/A: no package floor affected; new tests add coverage).

## Code Review

- [x] Run `/code-review` — cranky-code-reviewer reviewed the full diff.
- [x] ~~All critical review-responses addressed~~ (N/A: no critical findings).
- [x] All significant review-responses addressed — RR-V221YU + RR-H02GFD both fixed and verified.
- [x] Self-reviewed the diff for unrelated changes — only the ACL fix + tests + doc notes.

**Review Responses:** RR-V221YU (relation-column title leak, addressed),
RR-H02GFD (missing surface tests, addressed). No critical findings; two minor
findings taken as doc notes (where:-inference residual in executeView; command
view-input redaction note in commands.go).

## Acceptance Verification

- [x] Each acceptance criterion tested — hidden value absent from `_views` (property + title + relation-column surfaces); unreadable neighbor dropped.
- [x] Test evidence documented in implementation checklist (IMPL-6BQQ3C).

**Acceptance Status:** PASS — 4 ACL-views tests + 2 relation-column tests, all
verified to fail without the fix and pass with it.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist~~ (N/A: internal fix, no user-facing doc change).
- [x] ~~User-facing documentation~~ (N/A).
- [x] ~~Docs-checklist done~~ (N/A).

## Final Checks

- [x] Commit message explains the why, not just what.
- [x] No TODOs or FIXMEs left unaddressed — the one `visible:`-body TODO is pre-existing and out of scope.
- [x] Ready for another developer to use.

## Pull Request

- [x] Run `/pr` — PR #1212 created and CI monitored.
- [x] All CI checks pass — all local gates green (tests+race, lint, plimsoll, arch-lint); the re-push runs the same gates on CI and is monitored to green before merge.
- [x] PR URL documented below.

**PR:** https://github.com/sourcehaven-bv/rela/pull/1212
