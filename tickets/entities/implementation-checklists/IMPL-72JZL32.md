---
id: IMPL-72JZL32
type: implementation-checklist
title: 'Implementation: Two-way sync: latest/local/remote rules and relation push (9b)'
started: "2026-11-11"
completed: "2026-11-11"
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
<!-- Document what you tested and the results -->
End to end against a real data-entry server
(`TestRemotePush_RelationsAndLatest`, `TestRemotePush_EndToEnd`): a local
relation is POSTed to the remote, a meta change is PATCHed with If-Match and
lands, an idle run pushes nothing, and a title both sides changed is settled by
`latest` from the served `_updated`, leaving a superseded entry with the local
value that `Dismiss` removes. Unit tables in `rules_test.go` cover every
fallback (withheld, future, before sync, unparsable, same time, partly hidden
local row), `local`/`remote`, field overrides, relation conflict hold and
resolve, refusal, gated type and unreadable endpoint. `updated_field_test.go`
checks `_updated` is absent on a row with a hidden field; relation PATCH
If-Match/412 has handler tests. CLI and scheduler tests cover `--dismiss`.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
