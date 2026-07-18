---
id: IMPL-30XA8
type: implementation-checklist
title: 'Implementation: State machine: create with no initial/default lets an entity enter any state (incl. guarded), unconstrained'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Fix implemented (compile.go: a machine with transitions but no resolved entry value → boot error; EnforceCreate doc/invariant tightened)
- [x] Root cause addressed, not symptom (the hole was `m.entry == ""` → unconstrained create; fixed structurally at Compile so every machine has an entry and EnforceCreate's existing no-deviation check always applies)
- [x] Unit test for the fix (`TestCompile_RejectsTransitionsWithoutEntry`)
- [x] Regression coverage of the no-deviation enforcement (existing `TestTransition_IllegalEntryOnCreateIs422` + `TestTransition_IllegalEntry_DoesNotPersist`, now guaranteed to apply to all machines)
- [x] Edge cases (initial-set, default-only, neither → boot error; non-breaking for existing valid fixtures which all declare an entry)
- [x] Error handling (clear boot error naming the type + required field)

## Manual Verification

- [x] ~~Running-app manual test~~ (N/A: compile-time rule + write-path check, exercised by unit + entitymanager integration tests; no UI in scope)
- [x] Fix verified: a transitions-without-entry metamodel now fails `Compile`; a create with a non-initial value is rejected 422 (ErrIllegalEntry) and never persists

**Verification Evidence:**
- `TestCompile_RejectsTransitionsWithoutEntry` — transitions + no initial/default → boot error "must declare an `initial`"
- `TestTransition_IllegalEntryOnCreateIs422` — non-initial create value → 422
- `TestTransition_IllegalEntry_DoesNotPersist` — rejected create leaves no row
- Non-breaking confirmed: full `go test ./...` green — every in-tree transition fixture already declares an entry value.

CI: `go test ./...`, `golangci-lint ./...` (0), `just arch-lint`, `just
plimsoll`, `just coverage-check` (statemachine 85.6%) — all green.

## Quality

- [x] Follows project patterns (fail-fast at Compile, matching the existing entry/list/dangling validations)
- [x] No silent failures (boot error, not a silent unconstrained create)
- [x] No security issue introduced; the change closes one (create can no longer enter a guarded state unconstrained)
- [x] No debug code

**5-Whys → prevention:** root cause was that create was mentally excluded from
the machine's constraint model ("entry, not a transition"), so "which states may
a create enter" was left implicit and defaulted to "any" when no entry value was
set. Prevention: Compile makes an entry value mandatory for any machine, so the
constraint is always explicit and creates are always pinned. (Full chain in the
bug's why1–why5.)
