---
id: IMPL-4GYCPP
type: implementation-checklist
title: 'Implementation: Data classification overlay: labels, combination rules, subject inference, ACL audit (slice 1)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Unit tests: `internal/classification` (parse, lint, sync, derive, subjects,
profiles, report, exposures, minimum removals), `internal/datamigration`
renames, `internal/affordances` static grants. Integration tests in
`internal/cli`: sync then lint, validate, report text and JSON, and `rela acl
audit` on a project with roles, a client baseline, a scope, the `everyone` join,
a relation `visible:` grant, JSON `label`/`fields`, `--fail-on=any`,
`--no-classification` and a broken file.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Fixtures: `workedShape`/`workedFile`, `classificationProjectDir`,
`classificationTestFile`, `findingsFor`. Expected finding texts are literal on
purpose: they pin the wording users read.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- `/tmp/clsw` (person / employment / sick-leave worked example):
  - `rela classification sync`, `lint`, `report` and `rela validate` behave as
specified (earlier session): new fields added as `needs-review`, lint exits 1
until reviewed, report shows subjects, hops and derived labels.
  - `rela acl audit` with roles `everyone`, `hr`, `manager` (visible:
postcode) and an `apps` baseline redacting `person.email`: 13 low findings.
`manager` reads only `postcode` of employment. `hr` gets C2 `identified-person`
(breakers birth_date, gender, postcode) and C3 `identified-health` (breaker
`sick-leave.diagnosis`). `hr as app` gets a C3 without email (breakers title,
diagnosis). `--fail-on=any` exits 0; `--no-classification` reports no findings.
- AC1: existing `acl audit` and validate tests pass unchanged without the file.
- AC2-AC4: covered by the package and CLI tests above.
- AC5: covered by `TestClassificationFindings*`, `TestClassificationViewFor`,
`TestACLAudit_Classification*`, `TestExposures_*`. Deviation: findings are `low`
but never count toward `--fail-on`/`--exit-code` at all, not only at the default
threshold. Reason: they list access an operator may intend and cannot be
suppressed, so `--fail-on=any` (the documented production gate) would fail every
build.
- AC6: `just arch-lint` passes. `cli` now also depends on `affordances`
(near-leaf, cycle-free); `classification` has no internal imports.
- AC7: `rela classification *` loads schema and files only, no store.
Deviation: `rela acl audit` still binds `readServices` and so opens the store,
as it did before this ticket; the classification part adds no store access.
Moving the audit off the store is out of scope.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Security: all commands are operator-shell; the file size is checked before
reading; YAML anchors/aliases are refused; the audit evaluates a copy of the
policy through the real ACL code with a synthetic principal and changes nothing.
A classification file that does not parse is reported as a warning in the audit,
not swallowed.
