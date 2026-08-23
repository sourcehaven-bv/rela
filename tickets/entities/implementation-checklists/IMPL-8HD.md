---
id: IMPL-8HD
type: implementation-checklist
title: Implementation
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Blocking spike resolved before implementation: fsstore readers never take txMu (fsstore.go:134-138), and acl.StoreGraph only calls GetRelation/ListRelations — pinned empirically by TestTx_ReadsViaOuterHandleDoNotDeadlock, a property the package documented but nothing asserted. Cascade gate mutation-verified in an isolated copy: disabling it fails TestCascadeDelete_DeniedWhenRelationNotDeletable and TestCascadeDelete_DeniedLeavesEverythingIntact. The rewritten race test was mutation-verified against the naive no-Tx design and fails it, which the original version could not. go test ./... green after all code-review fixes.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
