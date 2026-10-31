---
id: IMPL-XN8UPE4
type: implementation-checklist
title: 'Implementation: _nav_status and _nav_items probe local roles per row (slow on atlas postgres)'
started: "2026-10-31"
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
Local copy of the atlas Postgres database, atlas main config, rela-server
built from develop and from this branch, same principal. Interleaved runs on
a loaded laptop (GOMAXPROCS=2, pool 4, like atlas):

| | develop | branch |
|---|---|---|
| `_nav_status` statements | 891 | 10 |
| `_nav_items?space=projecten` statements | 184 | 36 |
| `_nav_status` single request, mean | 48-59 ms | 10-16 ms |
| `_nav_status` at 20 req/s, p50 / p99 | 47-72 / 227-512 ms | 13-16 / 45-64 ms |

The `_nav_status` body is byte-identical between the builds and the
`_nav_items` body has the same hash. Earlier open-loop runs with `/taken`
and `/_analyze` alongside are in the ticket. The budget tests fail without
the fix (664, 503, 504 and 168 reads at 50 rows) and pass with it (8, 7, 8
and 12 at both sizes).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
