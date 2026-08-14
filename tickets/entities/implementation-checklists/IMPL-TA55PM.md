---
id: IMPL-TA55PM
type: implementation-checklist
title: 'Implementation: Cascade re-write post-automation re-checks'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Fix is in `cascadeHost.WriteEntity` (`internal/entitymanager/cascadehost.go`),
not in `autocascade`. The `Host` interface documents `WriteEntity` as "no
validation", and `autocascade` has no metamodel access — the implementation is
where `Deps` lives, so that is where the constraint knowledge belongs.

Three checks added, mirroring the top-level create path:

1. `Meta.ValidateEntity` + `partitionValidationErrors` — **hard errors only**.
Soft conditions (required-unset, out-of-enum) are tolerated on every other write
path per DEC-HWZHA; rejecting them here would make a cascade stricter than a
direct edit.
2. `checkUniqueProperties(ctx, deps, e, e.ID)` — `e.ID` excluded, since the row
is already persisted and would otherwise collide with itself.
3. `Transitions.EnforceCreate` — **not** `EnforceUpdate`. The row was created
moments ago in this same cascade, so the automation's value is still an *entry*
value: it must equal the declared entry, not merely be reachable by one legal
move. `EnforceUpdate` would wrongly accept a jump the create path forbids.

Errors surface through the existing `outcome.Errors` path in
`runner.go:187-190`, so a refused write appears in `AutomationErrors` rather
than being swallowed.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

`newRecheckManager` / `countHolders` / `duplicateEmailAutomation` are the shared
fixtures. Tests drive the **real** engine + runner over memstore — no stubbed
cascade.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Reproduced the bug first, then fixed it. Before:

```
automation errors: []
holder: PERS-3DLE      email=taken@example.com
holder: PERS-EXISTING  email=taken@example.com
=== rows holding the unique value: 2 ===
```

After: 1 holder, and the refusal is surfaced — `failed to update automation
entity TAAK-PGPQ: illegal entry state: status="done" on create; must enter at
"todo"`.

**Both re-checks mutation-tested.** Removing the transition check makes
`TestCascadeWrite_TransitionRecheckedAfterAutomation` fail; restoring it passes.
Same for the unique check. Neither assertion is vacuous.

**One false start, worth recording:** the first transition test "passed" against
unfixed code because my test metamodel declared `entry:`/`transitions:` inline
on an enum property, where they belong on a **named custom type** with
`initial:`. The state machine never compiled, so the test asserted nothing. A
green result from a test that does not exercise the path is worse than no test —
caught by reading the log rather than trusting the exit code.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Reuses the existing `checkUniqueProperties` / `partitionValidationErrors` /
`Transitions` collaborators rather than reimplementing any check.

**Deliberately NOT done:** extracting a shared apply-automation-results helper
that both paths must pass through. That is the structural fix implied by why5
and it is the right long-term answer, but it touches both write paths and
deserves its own change rather than riding along on a bug fix.

**Full gate:** `go test ./...` 0 failures · `just lint` 0 issues · `just
arch-lint` OK · `just coverage-check` PASS (77.1%) · `go test -race` on
`entitymanager` + `autocascade` clean.
