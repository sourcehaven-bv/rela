---
id: IMPL-K2V
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

Verified against a real project reproducing the outage (terugkerend --spawnt--> taak). Mutation-tested both critical guards in an isolated copy: removing the FromType guard fails TestRelationGrant_RequiresResolvedSourceType; moving the relation check above the ceiling fails TestRelationGrant_CeilingStillDenies on all three verbs. Probed six YAML shapes (null entry, scalar, sequence, null block, sequence block, null verb) — all handled, no panics. go test ./... green, golangci-lint 0 issues, arch-lint OK, plimsoll OK, coverage-check PASS (78.0% total; internal/acl 81.6%).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
