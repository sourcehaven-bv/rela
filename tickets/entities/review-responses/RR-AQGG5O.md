---
id: RR-AQGG5O
type: review-response
title: delete_entity addresses the whole entity family, so a faced id there is not the per-face delete the plan implies
finding: 'The plan puts update_entity/delete_entity/delete_relation out of scope on the hypothesis that a faced id like POL-1@draft ''may already carry the face''. Verified: that is true for the update path but NOT for delete. Manager.PatchEntity resolves via getEntityByRef (internal/entitymanager/manager.go:1033), which parses the state ref and calls GetEntityState (core.go:474-484) — so a faced id does select the face. DeleteEntity resolves via anyFaceOf (manager.go:1414), whose doc comment states outright that ''a delete addresses the whole FAMILY''. So rela.delete_entity(''POL-1@draft'') most likely deletes the entire entity including the published face, not just the draft. The plan''s out-of-scope note treats the two paths as one unverified case and would leave a reader believing a faced delete is merely unconfirmed rather than probably-destructive.'
severity: significant
resolution: 'Accepted. The plan''s Scope section no longer groups the three deferred paths under one ''unverified'' note; each is now resolved and stated separately: update_entity carries the face correctly via getEntityByRef, delete_entity deletes the whole family per anyFaceOf and the explicit comment at manager.go:1403-1407, and delete_relation targets the wrong edge (spun out as BUG-YVU8CP). The family-delete semantics are now a required docs item in GUIDE-lua-scripting.md and appear in the Risks table, since this ticket is what teaches Lua authors the @face vocabulary and thereby makes the asymmetry reachable.'
status: addressed
---

## Finding

The plan's Scope section defers `update_entity` / `delete_entity` /
`delete_relation` with a single shared justification:

> They address an existing row by id, where the `POL-1@draft` form may already
> carry the face through the manager's own parsing. Unverified either way.

Verified during design review. The two paths do **not** behave the same, and the
difference is destructive rather than cosmetic.

**Update: the face IS carried.** `Manager.PatchEntity` resolves the target
through `getEntityByRef` (`internal/entitymanager/manager.go:1033`), which
parses the state ref and dispatches to `GetEntityState` for a non-default face
(`internal/entitymanager/core.go:474-484`). A comment there explains the point
explicitly — passing `POL-1@draft` straight to `GetEntity` would return
`ErrNotFound` "for a row that plainly exists". So `rela.update_entity` on a
faced id plausibly already works.

**Delete: the face is deliberately DISCARDED.** `Manager.DeleteEntity` resolves
through `anyFaceOf` (`internal/entitymanager/manager.go:1414`), and its doc
comment states the intent without ambiguity (`manager.go:1403-1407`):

> anyFaceOf, not GetEntity: a delete addresses the whole FAMILY, and
> GetEntity(id) is GetEntityState(id, zero) — a coordinate a type declaring
> faces has no row at, so every faced entity was undeletable (BUG-HC6I2T).

`anyFaceOf` itself (`core.go:388-419`) falls back to an `AllStates: true` query
and returns *any* row. So `rela.delete_entity("POL-1@draft")` does not delete
the draft face; it resolves the family and deletes the entity.

## Why this matters for the plan as written

The plan's phrasing invites the reader to assume symmetric, benign deferral —
three paths that "may already work". A reader acting on that would conclude a
faced delete from Lua is an unconfirmed-but-plausible capability. It is instead
a likely footgun: a script author who has just learned that `POL-1@draft` names
the draft face on create and update would reasonably expect it to name the draft
face on delete, and get a family-wide delete.

This is not a defect introduced by this ticket — the behaviour predates it. It
becomes *more* reachable because of it: this ticket is what teaches Lua authors
the `@face` vocabulary in the first place.

## Suggested resolution

Split the out-of-scope note in the plan into its two real cases:

1. `update_entity` — face already carried via `getEntityByRef`; confirm with a
test and document, no code change expected.
2. `delete_entity` — deliberately family-scoped per `BUG-HC6I2T`. Per-face
delete from Lua is genuinely unbuilt, not merely unverified. Either file it as
follow-up work or document the family semantics in `docs/lua-scripting.md` so
the asymmetry is stated rather than discovered.

Documenting it is the minimum: `BUG-64MU2Q`'s own `prevention` field says a
capability gap should be "marked as unbuilt rather than closed with a refusal",
and an undocumented asymmetry is the weaker version of the same mistake.
