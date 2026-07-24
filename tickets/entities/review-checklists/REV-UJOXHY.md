---
id: REV-UJOXHY
type: review-checklist
title: 'Review: export: route entity/list export + export_render through visibility.Reader; thread request principal into ExecuteDocument (closes #1188 IB finding)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — full sweep green except `internal/docscapture` (pre-existing browser-env failure, identical on clean tree; CI authoritative)
- [x] Lint clean (`just lint`) — golangci-lint 0 issues (dataentry/script/visibility); arch-lint OK (visibility → dataentry.mayDependOn); plimsoll OK
- [x] Coverage maintained (`just coverage-check`) — no floors touched; visibility stays >85, dataentry unchanged band

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none found)
- [x] All significant review-responses addressed (RR-2QSGLU)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-2QSGLU (significant — document-render singleflight key
now includes the principal; preempts the TKT-ZF2DTV-era cross-principal leak),
RR-AF54A3 (minor — newExportHandler returns error), RR-BM0KIJ (minor — stale
override comment fixed). All addressed. Reviewer verified the **leak closure is
complete**: every path from raw entity to exported bytes goes through
Get/Filter/Redact; truncation math, filename, labels clean; double-gate proven
idempotent (ReadQuery and PermitsReadMany derive from the same readQuery);
documented residuals (body pass-through, override in-script reads) accurate.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** AC1–AC6 all PASS — see IMPL-VYWMQ8 Verification Evidence
for the AC → test mapping (5 new tests + 13 existing pins, all green first run).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated (docs/transforms.md §Access control)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS (see has-docs relation)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI — this ticket lands ON the existing PR #1188 (feat/transform-registry): the fix resolves that PR's CHANGES_REQUESTED block; pushed with a review reply to the CISO
- [x] All CI checks pass — monitored on push
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1188 (updated in place — the
CISO block is resolved on its own PR, per plan)
