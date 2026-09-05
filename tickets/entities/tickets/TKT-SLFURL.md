---
id: TKT-SLFURL
type: ticket
title: '`_self` on a non-bare face 404s under a configured default_world'
kind: enhancement
priority: high
effort: l
status: done
---

A faced response advertises `_self: "/api/v1/policys/POL-1@published"`. That URL
did not resolve when the project set `app.default_world`, and the SPA locked
every world-bound page read-only on top of it — for every type, including types
without faces. Atlas's adoption of worlds (`default_world: actueel`) made the
whole app read-only. Findings: `atlas-world/docs/rela-worlds-issues.md` (issues
1-5, 7, 9, 10; a minimal form of 8); executable acceptance manual in
`atlas-world/verify/`.

## Rule

```
view[entity@face]  --Edit-->  form[entity@face]  --Save-->  PATCH entity@face
```

The face is part of the address, exactly as the id is. What you look at is what
you edit is what you save.

## Server

- `entityRef` parses `ID` / `ID@face` once at the HTTP boundary
(`internal/dataentry/entityref.go`); the row gate keys on the bare id, the face
gate on the face. The bare face by its declared name (`POL-1@draft`) is the bare
row, explicit.
- An explicit address is served literally under every world on GET, PATCH,
DELETE and `_views`; `_world.via` is `unscoped` for it.
- `DELETE ID@face` removes that face only (`Manager.DeleteEntityFace`);
`ID@<bare>` deletes the entity.
- A PATCH to a non-bare face refuses `scope: content` relations
(`422 face_relations_unsupported`).
- `_faces[].ref` carries each face's address; `_self` on view rows.

## SPA

- Every write goes to `entityRef(row)` (`_self`); affordances come from
`_actions` alone. `isWorldBound` no longer gates anything.
- The edit form fetches its address in the default world and explains a
read-only non-bare face instead of blaming permissions.
- Face switcher links by address; copy toast speaks the copy's label and
lands on the written face; world note only on lists/boards of faced types.

## Left out (follow-up)

Issue 6: operator-configurable world chrome text and redirect targets.
