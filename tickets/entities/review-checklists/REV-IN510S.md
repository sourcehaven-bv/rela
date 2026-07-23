---
id: REV-IN510S
type: review-checklist
title: 'Review: Transform registry + view export (pdf/docx via external tools)'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — full suite green, no FAIL/panic
- [x] Lint clean (`just lint`) — golangci-lint 0 issues
- [x] Coverage maintained (`just coverage-check`) — package floor (50%) + total (65%) PASS; total 76.3%

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed — (design-review: RR-8C23IL addressed; code-review found no new critical)
- [x] All significant review-responses addressed — RR-1N142S (entityID path validation), RR-A9U1NQ (batched relation resolution); design-review RR-T3PDHN/C3M3BR/VYTL35/6ZDPTQ
- [x] Self-reviewed the diff for unrelated changes — none; all changes scoped to the feature

**Review Responses:**

Design-review (7): RR-8C23IL (critical, addressed), RR-T3PDHN, RR-C3M3BR,
RR-VYTL35, RR-6ZDPTQ (significant, addressed), RR-PBTUK5 (minor, addressed),
RR-UGKOI5 (minor, deferred). Code-review (5): RR-1N142S, RR-A9U1NQ (significant,
addressed), RR-PY4912 (minor, addressed), RR-YSTW0X (minor, deferred — inherited
entityReader error-swallow), RR-HDYCJH (nit, addressed). **No open critical or
significant.**

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** (full mapping in IMPL-1O8TLF)
- AC1 metamodel parse/validate incl. CRLF — PASS (metamodel/transforms_test.go)
- AC2 GET /_transforms — PASS (TestExport_TransformsList)
- AC3 visible entity export + hardened headers — PASS (TestExport_Entity_VisibleReturnsBytesAndHeaders)
- AC4 denied entity export 404 no bytes — PASS (TestExport_Entity_DeniedReturns404NoBytes)
- AC5 list whole ACL set + hidden-neighbor exclusion — PASS (TestExport_List_WholeScopedSetAsTable, _HiddenNeighborExcluded, _SharedNeighborResolvedOnce)
- AC6 cap + truncation notice boundary — PASS (TestExport_List_TruncationNotice)
- AC7 missing binary clear error — PASS (transform TestEngine_Probe_MissingBinary, cmdexec Probe)
- AC8 render override used + denied 404 — PASS (TestExport_Entity_DocumentOverride, _DeniedIs404, _UnknownDocument)

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs` — DOCS-FIY2UG
- [x] User-facing documentation updated — docs/transforms.md + CLAUDE.md note
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-FIY2UG

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- pending /pr -->
