---
id: IMPL-NC60BV
type: implementation-checklist
title: 'Implementation: Regression test for empty FromType against a type-scoped policy'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code — this ticket IS a test; there is no
production change.
- [x] Integration tests written — the table drives the real
`Manager.CreateRelation` against a real `acl.Declarative`, not `grantsVerb` in
isolation. That matters: half the concern is how `FromType` is *derived*, which
a policy-level test would skip entirely.
- [x] Happy path implemented
- [x] Edge cases from planning handled — five grant shapes, each with and
without the source entity.
- [x] ~~Error handling in place~~ (N/A: test-only)

## Test Quality

- [x] Using fixture builders or factories — the package's `parseMeta`,
`nopTemplater` and the `acl.NewDeclarative` pattern from
`cascade_authz_test.go`.
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter — one role, one assignment, one grant
list per row.
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

*The first version of this test was wrong, and finding out why is most of the
value.* I granted `create: [addresses]` — the **relation** type — and every row
failed. Reading `authorizeRelationWrite` rather than adjusting the expectations
showed the gate keys on the **source entity's** type:

> A relation create checks the source type's `create` grant (consistent with
> entity create).

Had I "fixed" the expectations instead, the test would have passed while
asserting a model of the ACL that does not exist.

*A real finding, kept rather than smoothed over.* With `create: [""]` — a
literal empty-string grant — an **absent** source is authorized while a
**present** one is denied. That is the one configuration where the invariant
genuinely does not hold. The issue's prose named it as theoretical ("no
realistic acl.yaml configuration"); running it confirms the behaviour is real.

It is documented and exempted rather than fixed, and the exemption is asserted
in **both directions**: the test also fails if the exemption stops applying. So
it cannot decay into a permanent excuse — if the ACL ever rejects empty-string
grants, this test says to delete the exemption.

*Mutation testing established the honest scope, and it is narrower than I first
claimed.* Substituting `"*"` for an empty `FromType` inside
`authorizeRelationWrite` — the concrete bypass shape — reddens **only** the
`create: [""]` row. With a realistic `create: [decision]` grant, an empty type
fails to match either way, so those rows cannot see it.

I initially added a "discriminating case" (`update: ["*"]`, no `create` grant)
believing it would catch the substitution. It did not — `grantsVerb` reads the
verb's own list, so a wildcard on `update` is invisible to an `OpCreate` check.
**I removed it rather than keep a case whose comment claimed a property it did
not have.** Reaching that substitution on a realistic grant needs a
client-baseline ceiling, which is a different subsystem; the test's doc says so.

**Gates:** `go test ./...` exit 0; `just lint` 0 issues (it caught a De Morgan
simplification); arch-lint, comment-lint, plimsoll clean.

## Quality

- [x] Code follows project patterns — table-driven with `t.Run` subtests and
`t.Parallel()`, per CLAUDE.md.
- [x] Checked for DRY opportunities — one helper runs both source states, so
each row declares only what differs. `UpdateRelation`/`DeleteRelation` were
deliberately NOT added: they share `authorizeRelationWrite` and the same
derivation, so a per-verb copy would triple the table to re-exercise one
function.
- [x] No security issues introduced — a test.
- [x] No silent failures — the directional invariant is asserted *separately*
from the per-row expectations, so a row whose `want` values are wrong still
cannot hide a bypass.
- [x] No debug code left behind.

**Why the assertion is directional.** It checks "absent is never more permissive
than present", not specific verdicts. A test pinning exact verdicts would pass
while the policy model changed underneath it; the direction is the property the
issue actually cares about and the one that survives a `grantsVerb` refactor.
