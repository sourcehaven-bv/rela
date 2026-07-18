---
id: BUG-X1C7S
type: bug
title: 'State machine: create with no initial/default lets an entity enter any state (incl. guarded), unconstrained'
description: 'A CustomType with transitions but no initial/default left EnforceCreate unconstrained, so a create could set the machine property to any value including a guard-protected state, bypassing the machine via create instead of a transition. Fix: Compile requires an entry value on any machine with transitions, pinning every create to the initial state. Reported as ISMS finding rela#1146; pre-release (feature on develop, not shipped).'
priority: medium
effort: s
why1: EnforceCreate returns early (unconstrained) when a machine has no entry value (m.entry == ""), so a create can set a machine property to any value including a guard-protected target.
why2: Compile does not require an entry value on a machine with transitions; a transitions block with no initial/default compiles fine, leaving m.entry empty.
why3: The create-path design (RR-1SMG4) only specified entry semantics for the case where initial IS set; the no-entry-value case was treated as 'unconstrained' without noticing it defeats the whole point of a lifecycle on create.
why4: The threat model (PLAN-5BYO6 Security Considerations) reasoned about the guard on transitions but not about create as an unguarded entry point that can land directly in a guarded state.
why5: A create is entry into the machine, not a transition, so it was mentally excluded from the guard/legality model — but 'which states may a create enter' is itself a constraint the machine must express, and the absence of an entry value silently means 'any'.
prevention: 'Compile now rejects a machine that declares transitions but no initial/default (entry value required to constrain creates). EnforceCreate''s existing ''must enter at entry value'' check then pins every create to the initial state. Regression tests cover: guarded machine with no entry rejected at compile; create with a non-initial value rejected 422.'
status: done
---

## Summary

Reported via IB/ISMS review of PR #1143 (GitHub issue rela#1146). A `CustomType`
with `transitions` but **no** `initial`/`default` has no entry constraint on
create, and create never evaluates a guard. So a principal with only
create-rights can create an entity **directly** in a guard-protected target
state without ever holding the guard permission — the state machine is bypassed
by routing through create instead of a transition.

Pre-release: caught in internal review before any release; the state-machine
feature is on `develop`, not shipped.

## Decision (2026-07-18)

Create may **not** deviate from the initial state. The fix is structural, not a
guard-on-create (a create has no `from`, so "the guard for this create" is
undefined — see the issue discussion):

1. **Compile-time:** any machine that declares `transitions` MUST declare an
entry value (`initial`, or `default`). `Compile` rejects one without it at boot.
This closes the `m.entry == ""` hole for guarded AND unguarded machines — a
create is always pinned to the entry value.
2. **`EnforceCreate`:** the existing "must enter at `m.entry`" check already
rejects any non-entry value (422 `ErrIllegalEntry`); with the compile rule,
every machine now has an entry value, so the check always applies.

Guards stay **transition-only** — create is entry, not a transition. Entering
directly at a guarded value is only possible if the operator explicitly sets
`initial: <that value>`, which is an allowed-by-design operator decision (same
trust already extended to `initial`).

## Scope

**In:** the compile-time entry-required rule + confirming `EnforceCreate`
enforces no-deviation + regression tests.

**Out (separate follow-up):** the create-form "entry-locked" affordance — so the
SPA renders the machine field read-only / omitted on create rather than showing
an editable status control. That reaches affordances + frontend and is filed
separately.

## Reference

- GitHub issue: rela#1146 (ISMS finding, severity Laag)
- Original create-path handling: RR-1SMG4 (only covered the `initial`-set case)
