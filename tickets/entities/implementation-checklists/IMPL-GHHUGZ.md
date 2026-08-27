---
id: IMPL-GHHUGZ
type: implementation-checklist
title: 'Implementation: Derive PostgreSQL indexes from static pushed-down query predicates'
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
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolation feature or generated expected strings)
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- `queryplan.StaticIndexSpecs` was exercised with dashboard, next-action and
`pick_one` queries. Equivalent reordered/literal-varied queries deduplicated;
unsupported and runtime-only shapes produced no specs.
- Live PostgreSQL 15 lifecycle test passed: dry-run create, create, idempotent
enforced result, orphan drop, and survival of an operator-owned index.
- Live PostgreSQL EXPLAIN over 5,000 task rows reported
`Index Scan using rela_derived_query__361f58cb59417f5a10164251fa9681a6` with an
index condition on `properties->>'status'`.
- Scalar predicate conformance covers scalar, list, absent, blank, integer and
boolean storage shapes across mem/fs/PostgreSQL implementations.
- Invalid `data-entry.yaml` test proves appbuild performs zero reconciliation
rather than passing a partial desired set; unique violation specs remain
published independently.
- `RELA_TEST_DATABASE_URL='postgres:///postgres?host=/tmp'
RELA_TEST_DATABASE_REQUIRED=1 just test-postgres`: PASS under race detector.
- `just arch-lint`: PASS. `just lint`: PASS with zero issues.
- Full `just ci`: final pass recorded after the implementation stabilized.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
