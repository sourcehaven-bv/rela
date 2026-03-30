---
id: REV-325e
status: done
title: 'Review: Define YAML schema types for Query-as-Output-Structure views'
type: review-checklist
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-f6nc (critical - validation), RR-zjlv (significant - child validation), RR-ygyq (significant - ViewNames ordering), RR-qnl5 (minor - edge case tests)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- PASS: QueryNode struct with all fields - verified by TestQueryNode_* tests
- PASS: ViewDefV2 struct with Description - verified by TestViewDefV2_Unmarshal
- PASS: FileV2 struct with Views map - verified by TestFileV2_Unmarshal
- PASS: Helper methods (IncludeContent, IncludeProps, IsRecursive, HasChildren, IsRoot) - verified by dedicated tests
- PASS: YAML unmarshaling works correctly - verified by all unmarshal tests
- PASS: Validation against metamodel - verified by TestViewDefV2_Validate_* and TestQueryNode_ValidateAsChild_* tests

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass (except Rela Tickets which requires ticket files in branch)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/248
