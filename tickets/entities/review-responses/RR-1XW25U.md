---
id: RR-1XW25U
type: review-response
title: appEntityWriter's floored-by-lua.Mutator claim is vacuous and misleading
finding: Every lua.Mutator method is already in App's direct-call set and lua.Mutator lacks UpdateRelation
severity: significant
resolution: Dropped the sentence. It was doubly wrong - every lua.Mutator method was already in App's direct-call set so the floor constrained nothing; and lua.Mutator lacks UpdateRelation.
status: addressed
---

`appEntityWriter`'s doc claims the set *"is also floored by lua.Mutator's six
methods, since App passes this value into lua.WriteDeps.EntityManager."*

`lua.Mutator` is CreateEntity, UpdateEntity, PatchEntity, DeleteEntity,
CreateRelation, DeleteRelation. Every one of those is **already** in App's own
7-method direct-call set, so the floor constrains nothing — the sentence implies
it is load-bearing when it is not. And `lua.Mutator` does **not** contain
`UpdateRelation`, so a reader trying to work out why `UpdateRelation` is present
will look there, not find it, and be misled.

Either drop the sentence or state it accurately.
