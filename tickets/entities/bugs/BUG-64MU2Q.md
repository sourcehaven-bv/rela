---
id: BUG-64MU2Q
type: bug
title: Relation writes cannot name the source face, so a content-scoped relation on a faced type is unwritable
description: 'A content-scoped relation attaches to the SOURCE''s face (Relation.FromFace; targets are faceless by design). Storage supports this, but the write API does not: entity.RelationOptions has no face field, so the data-entry handler refuses any faced relation write — with advice naming an address that no longer exists. The SPA always addresses a face on a faced type, so the relation is unwritable from any client.'
priority: high
effort: m
why1: 'Editing a `scope: content` relation on a faced type shows a 422 telling the user to ''edit it on the bare face'', an address BUG-HC6I2T removed, which now returns 404. The user has no way to write the relation from any client.'
why2: 'The handler refuses because the write API cannot express the source face the edge needs: entity.RelationOptions has no face field, so entitymanager.CreateRelation/UpdateRelation would attach the edge to the zero face and silently misfile it onto the wrong version. Refusing is the right call; being unable to express it is the defect.'
why3: 'The source face stops at the store boundary. store.RelationData.FromFace and the from_face column exist and are used by the copy kernel, but nothing above the store carries the value: not entity.RelationOptions, not acl.RelationSubject, not the v1 wire types, not the SPA — `from_face` appears nowhere in internal/apiwire or frontend/src.'
why4: The feature was built store-first and the client half deferred. TKT-2RQMV4 records the identical gap for entity creates ('no client can send a face yet'). The relation half had no such marker, so it presented as a settled refusal rather than unbuilt work.
why5: 'A capability gap was closed with a refusal that gave remediation advice instead of declaring itself unbuilt. That made it look finished, and the advice — being prose in a handler, not a value derived from the addressing model — then rotted silently when BUG-HC6I2T removed the address it named. Nothing failed: the guard''s test asserts the 422 fires, never that its advice works.'
prevention: 'When a capability is built store-first with the client half deferred, mark the gap as unbuilt rather than closing it with a refusal that offers remediation. A refusal reads as a settled contract, so the deferred work is only discovered when a user meets the dead end — and its advice, being prose in a handler rather than a value derived from the addressing model, is free to rot when the model moves under it. TKT-2RQMV4 carries exactly this marker for the entity-create half of the same feature and is why that gap is known rather than surprising. Secondary: test the escape a refusal names, not just that the refusal fires (measure refusal-remediation-is-reachable-test) — the guard''s test asserted the 422 and never its advice, so BUG-HC6I2T could delete the address the message recommends with every test still passing.'
status: in-progress
---

## Symptom

Editing a relation in the data-entry SPA on an entity of a **faced** type shows
an error toast:

> relation "schrijft_voor" is `scope: content`; edit it on the bare face, or
> move it with a copy definition

Neither escape works. A faced ID has no faceless row to edit (BUG-HC6I2T removed
`bare_face`), and a copy definition moves edges between existing faces rather
than authoring one. The relation cannot be written from any client.

## The model (confirmed, and correctly implemented in storage)

- A relation's **source has a face**; its **target is faceless**. A draft
version of a document may link to different things than the published version
does.
- `entity.Relation` matches this exactly: `FromFace` on the source, and
"deliberately NO `ToFace`" — which is what keeps cross-world dangling references
inexpressible (`internal/entity/entity.go:257-266`).
- `scope: content` vs `scope: identity` is the per-relation-type declaration
of whether an edge belongs to one version or to the entity as such.

None of that is in question. The storage layer implements it: a `from_face`
column in pgstore and sqlitestore, carried through relation versioning and
tombstones, and used by the copy kernel.

## The defect: the source face stops at the store boundary

Nothing above the store can carry the value:

- `entity.RelationOptions` (`internal/entity/writeapi.go:122`) has no face
field, so `entitymanager.CreateRelation`/`UpdateRelation` can only attach an
edge to the zero face.
- `acl.RelationSubject` carries no face either —
`internal/acl/authz_write.go:102` says so in a comment and calls the zero face
there "the accurate statement, not a placeholder", pending exactly this work.
- The v1 wire types and the SPA never mention it: `from_face` appears nowhere
in `internal/apiwire/` or `frontend/src/`.

So the data-entry handler refuses rather than misfile the edge onto the wrong
version (`contentScopedRelationOn`, `internal/dataentry/entityref.go:106`).
**The refusal is correct.** Being unable to express the face is the bug.

Two further problems compound it:

1. **The SPA cannot avoid triggering it.** `selfHref`
(`internal/dataentry/entityserializer.go:211`) always suffixes `@face` for a
faced type — there is no faceless row to point at — and the SPA PATCHes the last
segment of `_self` verbatim (`frontend/src/utils/entityRef.ts:22` →
`frontend/src/api/entities.ts:191`, relations panel via `DynamicForm.vue:1984`).
The ordinary "open from list, edit a relation" flow always hits the guard, with
no face anywhere in the user-visible URL.

2. **The advice is stale.** It shipped in TKT-SLFURL when the bare face was a
real address; BUG-HC6I2T removed it and nothing pointed at the prose that
depended on it. The manual states the correct model — "a face by name, or a copy
that declares which face it writes"
(`prototypes/worlds/manual/worlds-manual.md:233`) — while the message says "the
bare face", which no longer exists.

## Reproduction

Against the faced fixture in `internal/dataentry/entityref_test.go` (`policy`
declares `draft`/`published`; `cites` is `scope: content`):

1. `PATCH /api/v1/policys/POL-1@published` with
`{"relations":{"cites":{"data":[{"type":"feature","id":"FEAT-1"}]}}}` → `422
face_relations_unsupported`
2. Follow the advice — `PATCH /api/v1/policys/POL-1` → `404 not_found`

## Fix

Thread the source face from the address through to the store, which is the one
link missing. The value already exists at both ends.

1. **`entity.RelationOptions`** gains the source face, so
`entitymanager.CreateRelation`/`UpdateRelation` can attach an edge to the
version that owns it. `store.RelationData.FromFace` is already the destination.
2. **`acl.RelationSubject`** gains it too, and the write grant becomes
face-shaped for content-scoped types — matching the entity write grants
(`policy@draft` / `policy@published`). Without this, a caller granted the draft
could write edges belonging to the published version. `authz_write.go:102` is
the site that anticipates this.
3. **The data-entry handler** passes `ref.Face` through instead of refusing,
at which point `contentScopedRelationOn` and its 422 can go.
4. **Delete path**: `DeleteRelation` needs the same treatment, or an edge
becomes creatable but not removable per face. `store.DeleteRelationState`
already takes the face.
5. **Wire + SPA**: the relations panel already addresses a face (it PATCHes
`_self`), so the SPA may need no change beyond the refusal disappearing. Worth
confirming whether reads should expose which face an edge belongs to — today
`from_face` never reaches the wire, so the UI cannot show that a draft and
published version have different links.

Identity-scoped relations keep today's behaviour: entity-level, same edge from
every face, no face on the write.

## Related

- **TKT-2RQMV4** is the same gap for entity *creates* on a faced type ("no
client can send a face yet"). Same cause, adjacent surface; worth doing
together.
- The **copy kernel** already does the faced write correctly
(`internal/entitymanager/copy_apply.go:64`) and is the reference for what the
human-intent path should do.

## Regression test

Measure `refusal-remediation-is-reachable-test`: the existing test asserts only
that the 422 fires, never that its advice works — which is why the advice could
rot when the address it named was deleted. Once the fix lands, the test becomes
the positive case: a faced relation write succeeds and the edge lands on the
addressed face, not the zero face.
