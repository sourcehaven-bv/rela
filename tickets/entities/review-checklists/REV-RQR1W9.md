---
id: REV-RQR1W9
type: review-checklist
title: 'Review: Relation field-level ACL redaction (visible:) — currently absent for relations, live and history'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just ci` green — both default and `-tags postgres` for the touched packages)
- [x] Lint clean (`just lint` 0 issues; `just arch-lint` OK; `just plimsoll` OK)
- [x] Coverage maintained (`just coverage-check` PASS, 77.1% total)

## Code Review

- [x] Ran cranky-code-reviewer (initial + re-review after fixes)
- [x] All critical review-responses addressed (RR-FOD7IB — sync leak, resolved doc-and-defer per the read-feeds-write constraint; re-review withdrew must-fix-in-code)
- [x] All significant review-responses addressed (RR-AO1RFG restore-raw test, RR-JZ7VDI deleted-endpoint fail-closed test)
- [x] Self-reviewed the diff for unrelated changes (none — only ACL/affordances/dataentry/docs)

**Review Responses:** RR-FOD7IB (critical), RR-AO1RFG (significant), RR-JZ7VDI
(significant), RR-KTBK9G (minor), RR-UIX3UP (minor), RR-XCAI6J (nit)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. Live GET selective strip (all 4 shapes): PASS — TestRelationRedaction_LiveGet_SelectiveStrip / _LiveIncoming_UsesSourceGrant / _NoBlock_Permissive
2. History fails closed (subject-conditional): PASS — TestRelationHistoryRedaction_SubjectConditional_FailsClosed
3. history:read-redacted reveal: PASS — TestRelationHistoryRedaction_RevealPermission_ShowsFrozenMeta
4. Free-form key closed-world: PASS — TestRelationVisible_FreeFormKey_ClosedWorld
Plus fail-closed edges: empty role set, deleted-endpoint+spoofed fromType,
incoming-source-gone, restore-preserves-hidden — all PASS.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs` (DOCS-IWOJC6)
- [x] User-facing documentation updated (docs/acl-security.md + docs-project mirror)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-IWOJC6

## Final Checks

- [x] Commit messages explain the why (fail-closed rationale, plimsoll, review fixes)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass (all code checks green: Test, Postgres Backend, E2E, Lint, God-object, CodeQL, Fuzz, Analyze, cross-compiles; the "Rela Tickets" gate clears once this ticket reaches done)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1254
