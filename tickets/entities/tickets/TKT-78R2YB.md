---
id: TKT-78R2YB
type: ticket
title: 'Sync 3/5: public ApplyEntity/ApplyRelation (id-preserving upsert, automation-suppressed)'
kind: enhancement
priority: medium
effort: m
status: done
---

Sub-ticket of TKT-WE01O5 / FEAT-NJ9FEN. Addresses design-review RR-L1MY0N (crit)
and RR-AZMA7T (sig).

## Problem

Sync must re-create a record on the peer WITH ITS EXISTING ID. But
`CreateEntity` rejects an explicit id for non-manual id_type and `short` is the
default (`manager.go:334-343`, `core.go:45-53`, `metamodel/types.go:200-205`).
There is NO public upsert-with-id on the EntityManager interface. The internal
`upsertEntity` (`core.go:268-280`) preserves the id but bypasses ACL/audit
framing. Separately, `CreateEntity`/`UpdateEntity` run automation + cascade
(`manager.go:45-63`), so applying a pulled change could fire automations that
mutate other entities → sync loop / double side-effects.

## Scope

- NEW public `entitymanager` methods `ApplyEntity(ctx, e, opts)` /
`ApplyRelation(ctx, r, opts)`:
  - preserve the supplied id (no regeneration, no id_type rejection on apply),
  - create-or-update (upsert) semantics,
  - keep ACL + validation + audit (the project's "all writes through
entitymanager" rule must hold; ticket acceptance #5),
  - **suppress automation + cascade** on apply (context flag or apply-mode arg) —
the origin already ran them; derived changes sync as their own records.
- Model on internal `upsertEntity`/`upsertRelation` but with full public framing.

## Acceptance

- `ApplyEntity` creates a `short`-id entity on the peer with the given id (no
rejection), and updates it on a second call (idempotent).
- An audit record IS written for an applied write; ACL still gates it; invalid
content is rejected (validation error surfaced, mapped to 422 by the API layer).
- **Automation suppression**: applying a status-change does NOT auto-create the
status's checklist locally (assert no derived writes occur on apply).
- Existing CreateEntity/UpdateEntity behavior unchanged.

## Notes

This is consumed by both the server push-apply (sub-ticket 4) and the local
pull-apply (sub-ticket 5). HIGH risk: must keep the audit/ACL framing while
suppressing automation.
