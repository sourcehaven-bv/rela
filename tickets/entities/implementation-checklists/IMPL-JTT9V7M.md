---
id: IMPL-JTT9V7M
type: implementation-checklist
title: 'Implementation: Flows M3: policy for edits outside a flow'
started: "2026-11-10"
completed: "2026-11-10"
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

End to end through the real composition root (appbuild over a project on disk,
onboarding sample plus `outside_edits:` and a second `hold` flow), in
`internal/appbuild/flowedits_test.go`:

- TestOutsideEdits_GatedWrites: in `collect`, `deny` refuses a clerk and
HR with `field-affordance:read-only:<prop>`; a `{permission:}` rule refuses a
caller without it and lets a holder write; an unruled property stays writable.
- TestOutsideEdits_RunOwnWrites: the run's own step and `set` outputs are
not refused by its own rules.
- TestOutsideEdits_OtherRunStillNarrows: a step of one run is refused by
another run's rules, and the error maps to 403 (`flowview.ErrForbidden`).
- TestOutsideEdits_UngatedWritesUnchanged: CLI-shaped (ungated) writes
are not narrowed.
- TestOutsideEdits_RenameKeepsRules: after a rename the rules hold under
the new id; the test fails when the rekey subscriber is disabled.

Data-entry surface (`internal/dataentry/flowedits_test.go`,
`transitions_wire_test.go`): `_fields` marks the property read-only, PATCH
answers 403 without the attribution, `_transitions` reports `read-only`, the
affordance profile override is refused, a list page does zero run lookups, and
an editable section does one lookup for 5 and 25 rows.

Unit: flowcfg table (valid, invalid, warnings, empty block, stored declaration
ignored), flowedits (precedence, initial state of a new run, lookup failure
locks every property), flowrun conformance for `SubjectIDs` (kv, sqlite,
postgres), archguard pins the two `WithinRun` callers. All run with `-race`.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
