---
id: REV-73C6B2
type: review-checklist
title: 'Review: historical redaction fails closed'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `just lint` clean; `just arch-lint` clean
- [x] `just coverage-check` PASS (76.9% total)
- [x] Default + `-tags postgres` builds green; full default suite green; touched packages + DB-gated pgstore suite green under postgres tag

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer)
- [x] All critical review-responses addressed (RR-73CA — role-resolution leak: globals-only historical role resolution + type-level closed-world)
- [x] All significant review-responses addressed (RR-73CB docs over-promise; RR-73CC missing role-relation negative test)
- [x] Minor/nit addressed or dispositioned (RR-73CD _actions comment fixed; RR-73CE bool-payload wont-fix with reason)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-73CA (critical, addressed), RR-73CB (significant, addressed), RR-73CC (significant, addressed), RR-73CD (minor, addressed), RR-73CE (nit, wont-fix)

## Acceptance Verification

- [x] Subject-conditional (has_relation/count_relations) grant fails closed in history — PASS (TestHistoryRedaction_SubjectConditional_FailsClosed, TestHistorical_HasRelationVisible/CountRelations)
- [x] Local/ancestor-role-conferred visibility fails closed in history — PASS (TestHistoryRedaction_LocalRoleConferred_FailsClosed, TestHistorical_TypeLevelClosedWorld_EmptyRoleSet)
- [x] Reader-side inputs stay live; per-reader + reader-promoted correct — PASS (TestHistorical_ReaderSideGrant_Unaffected)
- [x] `history:read-redacted` holder sees all frozen fields (OVERRIDE) — PASS (TestHistoryRedaction_RevealPermission_ShowsFrozenField)
- [x] Since-removed referenced property fails closed — PASS (TestHistorical_MissingReferencedProperty_FailsClosed)
- [x] Marker inert for a type with no visible: policy — PASS (TestHistorical_NoVisiblePolicyForType_MarkerInert)
- [x] ~~Relation-history scenario 6~~ (N/A: deferred to TKT-B1F5Q1; seam comment left in relation_history_handler.go)

## Documentation (enhancements only)

- [x] `history:read-redacted` documented in GUIDE-acl-security (regenerated to docs/acl-security.md); code comments describe the complete neutering
- [x] ~~Docs-checklist~~ (N/A: doc source updated inline; no separate external-docs surface)

## Final Checks

- [x] Commit message explains the why
- [x] No TODOs/FIXMEs left
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass (monitored post-creation)
- [x] PR URL documented below

**PR:** (filled after creation)
