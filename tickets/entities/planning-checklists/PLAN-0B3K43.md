---
id: PLAN-0B3K43
type: planning-checklist
title: 'Planning: visibility: name the ungated read path (visibility.Unrestricted) so every bypass is greppable'
status: done
---

## Understanding

- [x] Problem understood — `lua.EntityReader` is satisfied structurally by `store.Store`, so `VisibleReader: st` (ungated) and `VisibleReader: gatedReader` look equally deliberate. Three sites were silently ungated through that blind spot (RR-R0G3DF).
- [x] Scope clear — naming/legibility, not enforcement or behavior.

## Research

- [x] All wiring sites enumerated via `grep -rn "VisibleReader:"` rather than trusting the ticket's count.
- [x] Existing idiom reviewed (`visibility.AllowAllReader`) for naming consistency.
- [x] Import constraints checked BEFORE designing the signature: `internal/visibility` must not import `internal/lua` (`.go-arch-lint.yml`), which invalidates the ticket's original `func Unrestricted(st) lua.EntityReader` sketch.

## Approach

- [x] Return a concrete `*UnrestrictedReader` whose method set matches `lua.EntityReader`; structural satisfaction does the rest.
- [x] Narrow deliberately to the three read methods so "ungated" cannot drift into "ungated and writable".
- [x] Convert every ungated site — including both NopACL branches — so the grep claim is actually true.

## Security

- [x] No behavior change intended; pass-through pinned by test.
- [x] Nil handling analyzed: returning a typed nil would bypass lua's `VisibleReader == nil` deny guard. Panic at construction instead (RR-5QQL1Z).
- [x] Confirmed no converted site is on an identity-bearing path that should be gated (verified via review + independent check).

## Test plan

- [x] Pass-through, method-set narrowing, nil panic, non-nil interface.
- [x] Every test mutation-verified rather than merely passing.
