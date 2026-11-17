---
id: IMPL-L29C3GG
type: implementation-checklist
title: 'Implementation: Conditional grant edges: when: on role_relations and membership'
started: "2026-11-16"
completed: "2026-11-17"
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

**Verification Evidence:** Scratch fs project with `auditor-of` edges carrying
`valid_until`, `when: "edge.valid_until >= today()"`:
- `rela acl can aud@example.org read TKT-1` (valid_until 2026-12-31): ALLOW via auditor-of edge.
- Same for an edge with valid_until 2026-11-15 on 2026-11-16 UTC: DENY.
- An edge with valid_until 2026-11-16 at 23:13 UTC on 2026-11-16: ALLOW (the last day counts in full, by UTC day).
- `valid_until: TBD`: DENY.
- `rela acl audit`: one `[info]` A16-conditional-grant finding.
- `when: "... or true"`: the policy load fails naming the AND-of-comparisons rule.
Automated: acl, aclstore, affordances, aclaudit, predicate, appbuild, dataentry,
scheduler, archguard, mcp packages pass; store suites pass on memstore, fsstore,
sqlite, naive and postgres (edge props + differential).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
