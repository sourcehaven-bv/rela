---
id: IMPL-A1RH8P
type: implementation-checklist
title: 'Implementation: CalDAV: declarative caldav: config — symmetrical single-type mapping + fail-fast validation'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

`internal/dataentryconfig/validate_caldav.go` with 9 table-driven test
functions; package coverage 89.3%. The full flow (config → served collection →
inbound write) is covered by the adapter tests in `internal/dataentry` and by
the live demo, since config alone has no runtime behaviour to integrate.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Table-driven throughout, in the `validate_feeds_test.go` style the ticket asked
for. Each case names only the field under test and asserts on the returned
message, so a rule change fails the one case that owns it.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

The validation earned its keep during the live demo: `validateCalDAVCompletionReachable`
was added after a real footgun bit me. My own demo config used
`where: ["status != done"]`, which made the Reminders checkbox tick and then
silently revert — rela recorded the completion, the entity stopped matching the
filter, the resource vanished from the collection, and the client restored its
local copy unchecked. The check then immediately caught the same mistake in a
test fixture I had written.

Per AC:

| AC | Status | Evidence |
|----|--------|----------|
| 1. Valid block loads; each invalid case gives a node-identifying error, collected in one pass | PASS | Table-driven tests per rule; messages prefixed with the collection name |
| 2. `ValidateConfig` fails startup on an invalid block | PASS | Wired into the same path as `feeds:` |
| 3. Declarative and Lua mutually exclusive, distinct messages | PASS | Two-arm switch, both arms tested |
| 4. Unsatisfiable-from-`SUMMARY` type rejected at config load | PASS | `validateCalDAVConstructible`, the deliberate DEC-HWZHA departure |
| 5. Multiple collections over different types coexist | PASS | Covered in tests |
| 6. Table-driven tests in the `validate_feeds_test.go` style | PASS | 9 test functions |

**Known limitation, as the ticket predicted:** `rebuildState` never calls
`ValidateConfig`, so these guarantees hold at **startup only** — a hot-reload
catches YAML syntax errors and nothing else. Accepted and documented rather than
fixed here.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Follows `validate_feeds.go`: flat linear checks returning `[]string`, early
return once `entity_type` fails to resolve. `golangci-lint` 0 issues.
