---
id: IMPL-8811OPK
type: implementation-checklist
title: 'Implementation: Regex search on an enum property finds nothing'
started: "2026-10-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: the data-entry test schema has no enum property on the search path; the end-to-end search was verified manually (evidence below))
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: extends the existing table-driven filter tests)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolated values)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: assertions compare match booleans only)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
rela-server from develop and from this branch, run on a copy of the GRC demo
project (fs backend), `_search` as the CISO:

| query | develop | fix |
| --- | --- | --- |
| `type:systeem prop:beschikbaarheid=hoog` | 4 | 4 |
| `type:systeem prop:beschikbaarheid=~hoog` | 0 | 4 |
| `type:systeem prop:beschikbaarheid=~^(hoog\|midden)$` | 0 | 8 |
| `type:systeem` | 9 | 9 |

Filter unit tests cover inline enum and custom `types:` regex.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
