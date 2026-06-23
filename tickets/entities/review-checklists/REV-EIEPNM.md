---
id: REV-EIEPNM
type: review-checklist
title: 'Review: Attachment web write path: upload endpoint + drag-drop widget + default size limit + Lua file info'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — `go test ./internal/dataentry/ ./internal/store/... ./internal/attachment/ ./internal/dataentryconfig/` green; frontend `npm run test:run` 1086 passed
- [x] Lint clean — `golangci-lint run ./internal/dataentry/ ./internal/store/...` 0 issues; frontend lint 0 errors; `vue-tsc` clean; `just arch-lint` OK (incl. the dataentry→store boundary for the moved reader)
- [x] Coverage maintained — `just coverage-check` PASS (added a `store` package test for `CapAttachmentReader`, which had introduced 10 uncovered statements; store now 90%)

## Code Review

- [x] Ran code review (cranky-code-reviewer) — found a **critical data-loss bug** (C1) the first pass missed
- [x] All critical review-responses addressed — RR-EISJTE (fsstore replace+oversize data loss) fixed + test
- [x] All significant review-responses addressed — RR-81XSCY (clamp), RR-AFMZ9S (log)
- [x] Self-reviewed the diff for unrelated changes — scoped; the `CapAttachmentReader` lives in `internal/store` (not storeutil) to respect arch boundaries

**Review Responses:** RR-EISJTE (critical), RR-81XSCY (significant), RR-AFMZ9S
(significant), RR-JIRLLU (minor) — all `addressed`.

## Acceptance Verification

- [x] Each acceptance criterion tested:
  - AC1 upload/replace/view/delete e2e (fs; pg via storetest): `TestAttachmentUpload_RoundTrips`/`_Replaces`/`TestAttachmentDelete_RemovesBytesAndProperty` PASS
  - AC2 over-limit all backends: `TestAttachmentUpload_TooLarge` (413) + storetest `RejectsOversize`/`OversizeReplaceKeepsExisting` (fs/mem/pg) + `TestCapAttachmentReader` PASS
  - AC3 no-update-access denied before bytes: `TestAttachmentWrite_DeniedBeforeBytes` PASS
  - AC4 orphan window closed + no data loss on failed replace: `TestAttachmentUpload_ReplaceOversizeKeepsExisting` + storetest case PASS
  - Frontend upload/drag-drop/remove + error: FileWidget tests PASS
- [x] Test evidence documented (implementation checklist + above)

**Acceptance Status:** ALL PASS

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs` — DOCS-SXKF2O
- [x] User-facing documentation updated — `docs/data-entry/api-reference.md` upload/delete + size limit
- [x] Docs-checklist marked as done — DOCS-SXKF2O `done`

**Docs Checklist:** DOCS-SXKF2O

## Final Checks

- [x] Commit message explains the why — pending commit (user controls timing)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr`~~ (deferred: user controls commit/PR timing; changes staged)
- [x] ~~CI checks pass~~ (deferred to PR — full local CI surrogate green)
- [x] ~~PR URL~~ (deferred)

**PR:** pending — changes staged, not yet committed/pushed
