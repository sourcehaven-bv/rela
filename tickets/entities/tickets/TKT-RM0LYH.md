---
id: TKT-RM0LYH
type: ticket
title: rela import bypasses transition guards on create and update
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

`rela import` writes entities straight to `store.Store` via
`CreateEntity`/`UpdateEntity` without calling `Transitions.EnforceCreate` or
`EnforceUpdate`. The guard / entry-constraint that rela#1154 just introduced
structurally for the regular write path can therefore be bypassed via import —
on both create and update.

GitHub issue #1155 (IB-review rela#1154), CONTROL-5-15 + POLICY-015 §3.
Severity: medium. Not a regression from #1154; an existing gap that PR did not
cover.

## Impact

An entity can enter a guard-protected state via `rela import` — including
against a production database through `RELA_DATABASE_URL` — without passing the
guard the operator configured.

## The decision this needs

The issue offers two paths, and they are genuinely different:

1. **Enforce.** Call the transition guards from `importEntity`, so import
obeys the same constraints as every other write.
2. **Document the exemption**, as was already accepted for `rela renumber`
(POLICY-015 §4).

Option 2 has a real argument: import is a bulk operator tool, and a guard that
rejects half a dataset mid-import leaves a partially-loaded project. Option 1
has the stronger one: a guard that any operator can bypass by reaching for a
different command is not an access-control measure, it is a suggestion.
