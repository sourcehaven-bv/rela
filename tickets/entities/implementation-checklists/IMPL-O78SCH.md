---
id: IMPL-O78SCH
type: implementation-checklist
title: Implementation
status: done
---

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

The guard is one function, `requireCopyGates`, called from `New`. Edge cases
covered: both gates nil, exactly one nil (the error must name only what is
missing, or it sends the operator to re-wire correct code), explicit opt-out,
and both no-policy ACL implementations.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects~~ (N/A: assertions are on
error text and nil-ness, no interpolated object values)
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario
- [x] Edge cases manually verified

**Verification Evidence:**

The guard was verified **non-vacuous** by reverting the fix at the site that
motivated it: removing the two `CopyReadGate` / `CopyVisibility` lines from
`appbuildtest/fixture.go` fails `./internal/appbuild/...` with

```
panic: appbuildtest.New: build entitymanager: entitymanager: New:
CopyReadGate and CopyVisibility required when ACL is a compiled policy
```

on `TestScheduledLuaWriteDeps_ReadsAreACLBound` and
`TestNew_WithDeclarative_WiresBothACLAndDeclarative`. Restoring the two lines
returns the suite to green. That is the evidence the unsafe state was reachable
rather than theoretical: those two tests pass a real declarative policy into a
fixture that left both gates nil.

Half-nil messaging was caught by its own test during development — the first cut
named both opt-outs in the hint even when only one gate was missing.

## Quality

- [x] Code follows project patterns (`AllowAllFieldGate`, the
`Automations`/`Cascade` coupling check, the `TransitionWiring` bundle)
- [x] Checked for DRY opportunities — the gates now have ONE construction
site (`CompileTransitions`) instead of an inline build plus a hand-copy
- [x] `just lint` clean (0 issues)
- [x] `just arch-lint` clean
- [x] `just comment-lint` gate clean
- [x] `just plimsoll` clean
- [x] `just coverage-check` passes (78.9% total)
- [x] `go test -race ./...` passes (enforced by the pre-commit hook)
