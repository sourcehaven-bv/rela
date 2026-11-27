---
id: IMPL-C9B29C7
type: implementation-checklist
title: 'Implementation: Flows M1f: predicate starts and change events from every write'
started: "2026-11-25"
completed: "2026-11-25"
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

- `internal/flowengine/onchange_test.go`: change event with and without
the role, guard false, no-op write, branch without `when:`, machine write, step
write skip, claimed step, event kept while settling and taken on settle, forked
run and join, predicate edge, already matching, chain bound, busy subject,
predicate on create.
- `internal/flowrun`: `Pending.Change` validation and round trip in the
conformance suite (kv, sql, sqlite, postgres).
- `internal/appbuild/flowwriteevents_test.go`: manager PATCH moves the
run as its author (hr moves, clerk does not), a Lua `rela.update_entity` moves
it, a predicate start runs once and settles. Passed on fs, `-tags sqlite` and
`-tags postgres`.
- The tests found a deadlock: a write nested in a transaction took
`writeChecked`'s fast path to the base store. Fixed: the fast path applies only
outside a transaction.
- Manual CLI run on a scratch project: a name-only update started no
run; `status=hired` started RUN in `collect`; `documents=true` moved it to
`finished` / `completed`.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
