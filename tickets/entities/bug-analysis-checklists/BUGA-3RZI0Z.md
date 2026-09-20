---
id: BUGA-3RZI0Z
type: bug-analysis-checklist
title: 'Analysis: Relation writes cannot name the source face'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

Reproduced as a Go test against the existing faced fixture (`facedApp` /
`seedPolicyFaces` in `internal/dataentry/entityref_test.go`); the worlds
prototype ships no seeded entities, so a live server is a worse reproduction.

```
faced PATCH  -> 422 face_relations_unsupported
                "relation \"cites\" is `scope: content`; edit it on the bare
                 face, or move it with a copy definition"
bare PATCH   -> 404 not_found
```

Conditions: any schema declaring `faces:` on a type that is the source of a
`scope: content` relation. Faceless schemas (`tickets/`, `docs-project/`) are
unaffected.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

Recorded in `why1`-`why5` on BUG-64MU2Q.

The model is: a relation's **source has a face, its target is faceless** — a
draft version may link to different things than the published version.
`entity.Relation` implements exactly that (`FromFace`, and "deliberately NO
`ToFace`"), as do both database backends.

The defect is that the source face **stops at the store boundary**.
`entity.RelationOptions` has no face field, `acl.RelationSubject` has none, and
`from_face` appears nowhere in `internal/apiwire/` or `frontend/src/`. So the
data-entry handler refuses rather than misfile the edge onto the wrong version.
The refusal is correct; the inability to express the face is the bug.

Two dead ends were explored and discarded before reaching this. Recorded so they
are not retried:

1. *"The message is merely stale; let the faced write through."* Half right —
the message is stale — but it treated the missing write-API face as incidental
rather than as the defect.
2. *"Relations should be faceless."* Wrong. Targets are faceless; sources are
not, and `scope: content` is the intended per-type declaration of which.

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

Approach: thread the source face from address to store — see the "Fix" section
on BUG-64MU2Q for the five steps. The value already exists at both ends; only
the middle is missing.

The ACL step is load-bearing, not incidental: once a relation write names a
face, the write grant must be face-shaped for content-scoped types, or a caller
granted the draft could write edges belonging to the published version.
`internal/acl/authz_write.go:102` already anticipates this.

Regression test: measure `refusal-remediation-is-reachable-test`, which becomes
the positive case once the fix lands — a faced relation write succeeds and the
edge lands on the addressed face, not the zero face.

Related areas checked:

- **Delete path.** `DeleteRelation` takes no face either, so without the same
treatment an edge would become creatable but not removable per face.
`store.DeleteRelationState` already accepts one.
- **Legacy per-relation endpoints** (`handleV1CreateRelation` /
`handleV1DeleteRelation`) do not parse an entity ref at all, so on a faced type
both a bare and a faced address 404. The SPA's relations panel does not use
them.
- **Entity creates on a faced type** — TKT-2RQMV4, the same store-first gap on
an adjacent surface. Worth doing together.
- **The copy kernel** already performs the faced write correctly
(`internal/entitymanager/copy_apply.go:64`) and is the reference implementation
for the human-intent path.
- **Reads do not expose the edge's face.** `from_face` never reaches the wire,
so the UI cannot show that a draft and a published version have different links.
Not the reported bug, but it belongs in the same change.
