---
id: DEC-EJ00DNZ
type: decision
title: A headless flow does not read an unset property as its initial state
context: M5c (TKT-ZRQSGCM) folds enum transitions into flowcfg as headless flows and asks whether an unset property reads as the initial state. A run-based flow reads an unset instance as its initial state.
consequences: Every move from an unset machine property is refused, including the move to the entry value. Entities that predate a machine get their value from a data migration, which writes the store directly. No behaviour changes.
date: "2026-11-13"
status: accepted
---

## Decision

A headless flow, the flow of an enum property with `transitions:`, does not read
an unset property as its initial state. A write that moves the property from
unset to any value is refused with `ErrIllegalTransition`, also when the value
is the entry value. A create still enters at the entry value or leaves the
property unset.

## Why

- Reading unset as the initial state would let an update take the guarded
edges out of the initial state without the entity ever having been in it. The
`initial` of a machine pins creates; it does not describe entities that predate
the machine.
- A run-based flow reads an unset instance as its initial state because the
engine created the instance. For a headless flow nothing created the value, so
an unset value is data the machine has not seen.
- It is the behaviour of the enforcer before and after M5a (TKT-12RC7JT), so
no project changes behaviour.

## Recovery

An entity that predates the machine, or was created without the property, gets
its value from a data migration. Data migrations write the store directly, so
the transition enforcer does not apply.

Pinned by `TestEnforceUpdate_UnsetIsNotInitial` in `internal/flowcfg`.
