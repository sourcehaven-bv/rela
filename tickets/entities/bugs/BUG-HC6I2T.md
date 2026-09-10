---
id: BUG-HC6I2T
type: bug
title: Writes resolve their face differently from reads, so a create always lands on the bare row
description: 'Reads resolve a face from explicit @face, else ?world=, else default_world. Creates ignore all three and write the zero coordinate: createCoreOpts carries no coordinate and buildCandidateEntity calls entity.New, leaving Entity.Face zero. There is no way to create an entity into a named face, ACL is applied to the redirected face rather than the intended one, and an unknown `face` key in a create body is silently dropped with a 201.'
priority: high
effort: l
why1: createCoreOpts (internal/entitymanager/core.go:19) carries no face coordinate, so buildCandidateEntity's entity.New leaves Entity.Face at its zero value and every create writes the bare row.
why2: 'Manager.CreateEntity DOES authorize against a face -- acl.EntitySubject{..., Face: e.Face} at manager.go:736 -- but then builds its createCoreOpts literal without carrying e.Face into it. The authorization input and the write target were populated from different places, and only one of them was ever set.'
why3: 'e.Face is structurally always the zero value on the create path because no caller can set it: entity.CreateOptions has ID/Prefix/Variant/SkipAutomation and no Face, and the HTTP create handler decodes into an anonymous struct with no face field. The ACL check reads a field that is correctly wired and permanently empty, so it always returns a decision about the bare face and never disagrees with the write in any exercised case.'
why4: The guard that exists for this exact defect class (TestEveryEntitySubjectNamesItsFace, BUG-Y0GNSB) checks that an acl.EntitySubject literal SETS Face, not that the value it sets is the one the write uses. manager.go:736 satisfies the guard while being the bug, so the measure that was supposed to catch a forgotten face reported clean.
why5: Face resolution was implemented per operation rather than once. Reads grew a real resolver (explicit @face, then ?world=, then default_world) while writes kept an implicit constant, and nothing named the resolution ORDER as a shared contract that both sides must satisfy. A per-site invariant (does this literal set a face?) cannot detect two sites that each look locally correct and disagree with each other.
prevention: 'The condition now names the rule directly: CreateEntity authorizes and writes the SAME opts.Face value, so the two cannot be populated from different places. requireCreateFaceFor is called from all four create paths (CreateEntity, ValidateCreate, cascadeHost, ApplyEntity) so they cannot drift. The new measure pins the resolution ORDER rather than each input, because the asks are separately satisfiable and per-input tests would pass on a partial fix. Beyond this bug: the same authorize-here/read-there shape was found and fixed in seven more places (UpdateEntity, ApplyEntity, relation endpoints, the world-absent view, DeleteEntity, RenameEntity and the store type probe), which suggests the class is worth a lint rather than a test -- a write that authorizes one coordinate and reads another is detectable structurally. RenameEntity was the sharpest case: the missed read took the not-found branch, which skips ACL by design, so a faced rename succeeded under a deny-all policy.'
status: review
---

## Summary

A write always lands on the zero coordinate. Reads resolve a face properly
(explicit `@face`, else the world chain), but a create ignores every one of
those signals and writes the bare row. There is no way to create an entity into
a named face.

Found while adopting worlds/faces in atlas (ISMS), the first production usage of
faces and the first workload where the bare face is not the draft.

## The rule

A face should be resolved the same way for every operation, by exactly three
inputs:

1. an explicit face in the address (`ID@face`), else
2. an explicit world (`?world=`), else
3. the default world (`default_world`).

Reads already work this way. Writes should too. Nothing else gets a vote, and in
particular no face is privileged for being the one stored unsuffixed.

`bare_face` is that privilege. It is not needed for addressing: `default_world`
already decides what an unqualified request means, and decides it better,
because a chain can express "adopted where it exists, else concept", which a
single named default cannot.

## Reproduction

atlas `metamodel.yaml`, where the adopted face is the unsuffixed one:

```yaml
entities:
  beleid:
    bare_face: vastgesteld
    faces:
      vastgesteld: { label: Vastgesteld }
      concept:     { label: Concept }
worlds:
  actueel:
    select: [vastgesteld, concept]
    otherwise: default
```

ACL grants the CISO `update` on `beleid@concept` but not on the bare face, so
adopted text can only be written by the guarded `vaststellen` copy. Against a
running server as that CISO:

| Request | Result |
|---|---|
| `POST /api/v1/beleids` | 201, lands on the **bare** face |
| `POST /api/v1/beleids/POL-X@concept` | **405** Method not allowed |
| `POST /api/v1/beleids` with `"face":"concept"` in the body | 201, key **silently ignored**, still bare |
| `POST /api/v1/beleids?world=actueel` | 422 |

All three ways of naming a face are refused, ignored, or rejected. The created
entity comes back:

```json
{"id":"POLICY-001","_self":"/api/v1/beleids/POLICY-001",
 "_actions":{"update":false,"delete":false,"rename":false},"_faces":[]}
```

The author cannot edit or delete what they just created, and the policy is
adopted the moment it exists, with no `vaststellen` event. The ISMS control
inverts.

Reads, for contrast, resolve correctly:

| Request | `_world` |
|---|---|
| `GET /beleids/POLICY-TOEG` | `{name: actueel, face: vastgesteld, via: chain, chain_position: 0}` |
| `GET /beleids/POLICY-TOEG?world=default` | `{name: default, face: vastgesteld, via: unscoped}` |

The bare read is served by the chain, not by `bare_face`.

## Cause

The create path has no face parameter to lose. `createCoreOpts`
(`internal/entitymanager/core.go:19`) carries ID, prefix, template variant,
properties and content, and no coordinate. `buildCandidateEntity` calls
`entity.New(entityID, entityType)` at `core.go:159` (the only such call in the
package), whose `Entity.Face` is left at its zero value, and `createCore` hands
that to `Store.CreateEntity` at `core.go:121`. Its own doc calls it "the bare
write path".

Reads and copies resolve coordinates through `StoredFace` / `DeclaredFace`
(`internal/metamodel/copies.go:329`, `:358`). Create is the one write path that
never asks.

rela's own metamodel doc already states the rule this violates
(`internal/metamodel/types.go:322`):

> It names a row, it does not create one. [...] All this says is which declared
> NAME refers to it.

and warns against exactly the reading the create path implements:

> [it] read as a statement about precedence — "the default face" sounds like the
> important one, when in an ISMS it is the DRAFT while the published face is the
> one in force.

## Design decisions taken during analysis

These revise the report. Recorded here because the reasoning matters more than
the conclusion.

### The headless-state invariant is not a reason to keep the bare face

The store refuses to create a non-bare row when no bare row exists
(`pgstore/entity.go:324` and the other three backends,
`storeutil.HeadlessStateError`: "a state row cannot exist headless"). That looks
like a structural argument for a privileged bare row. It is not:

- **The store already tolerates headless families.** Four sites say so
explicitly ("a headless family (tolerated from disk, design doc §6)" at
`fsstore/entity.go:395` and `:758`, `sqlitestore/rename.go:28` and `:160`).
Rename, delete and notify all carry defensive scans that work without a bare
row. It is refused only at the create door, so it is a write-path policy and not
an invariant of the data.
- **Its stated justification is circular.** `pgstore/entity.go:594` says "a
family with no default row has no defined meaning and world fallback resolves
against it." World fallback resolves against the zero row only because
`bare_face` made it the fallback target. Removing `bare_face` removes the
premise.

### There is no "anchor row" requirement

Three sites predicate on `face = ''` and look like they need a designated
identity row. Two of them do not:

| Site | Predicate | What it actually needs |
| --- | --- | --- |
| `HighestID` (`pgstore/entity.go:217`) | `face = ''` | count each family once -- `DISTINCT id` |
| Identity-anchor CTE (`pgstore/graphquery.go:545`) | `e0.face = ''` | one seed per family -- `DISTINCT id` |
| `unique:` index (`pgstore/derivedschema.go:488`) | `face = ''` | one row per family in the natural key |

Identity lives in the `id` column: the primary key is already `(id, face)` and
case-identity is `(lower(id), face)` (migration `0011_content_states.sql`). No
row carries identity on behalf of the others.

### `unique:` is the one real question, and it is deferred

`unique: true` compiled to a partial index over bare rows, silently meaning
"unique among whichever face `bare_face` points at". With no privileged face it
must be answered. It is a PARTIAL INDEX predicate, which structurally cannot
reference a runtime parameter, so the reading must be decidable at DDL time.

This bug implements **per-face**: two entities may not share the value within
one face; two faces of one entity never collide. Chosen because it is the
reading that cannot silently DROP a constraint an operator already relies on --
every collision the old bare-row index rejected is still rejected when both rows
share a face.

The per-entity reading is equally legitimate and is deferred to **TKT-HXT2P9**,
which adds `unique: per-face | per-entity`.

### Two operations are hiding under one verb

"Create" currently means both "bring this entity into existence" and, once faces
exist, "add a state to it". They differ in whether the id is generated
(`HighestID` filters `face = ''` because states share their family's number) and
in which face is legal. The fix must not conflate them.

## What the zero coordinate becomes

The report proposed keeping the zero row as a "family anchor". Analysis
disproved the need: identity lives in the `id` column (the primary key is
already `(id, face)`), and the three sites that predicated on `face = ''` each
wanted something weaker.

| Site | Wanted | Now |
| --- | --- | --- |
| `HighestID` | count each family once | `DISTINCT id`, every face |
| Identity-anchor CTE | one seed per family | unchanged, see below |
| `unique:` index | one row per family in the key | per-face, `face` as a key column |

So the zero coordinate keeps exactly one job: it is where a type declaring NO
faces stores its single state. A type declaring faces stores nothing there.

## What removing `bare_face` fixes beyond this bug

Three problems in atlas trace to one face being privileged by storage:

- **Writes silently redirect.** This bug.
- **ACL grants are position-dependent.** A grant names the face as stored, so
`update: [beleid]` means "the bare face", and which face that is moves when
`bare_face` moves. rela's docs flag this with a **Warning**
(`docs/content-states.md:323`).
- **Changing it is a data migration**, not a relabelling
(`metamodel/shapecompare.go:212`), because it rewrites what every existing
entity's unsuffixed id refers to.

## Asks, as resolved

1. **Writes name their face.** Done, with one deviation: `?world=` is NOT a
face input for writes. `internal/dataentry/world.go:493` refuses it on every
write deliberately, because a world resolves through a chain with a *fallback* —
`PATCH ?world=published` on an entity with no published face would silently edit
the draft. A create takes an explicit `face` in the body, else nothing, and a
faced type has no default to fall back to.
2. **ACL applies to the face actually written.** Done. `CreateEntity`
authorizes and writes the same `opts.Face`.
3. **`bare_face` removed.** Done, including the wire field, the SPA field, the
`{bare_face}` chrome placeholder, the loader validation and the shape-migration
findings.
4. **An unknown key in a create body is rejected.** Done via
`DisallowUnknownFields`, so a misspelled key is a 400 rather than silently
dropped.

## `create_world` is not the fix

It looks like it and is not. Its doc comment
(`internal/dataentryconfig/config.go:591`) describes this exact scenario:

> An ISMS list rendered in `published` creates a DRAFT: nobody authors straight
> into the published face. Without this the form would inherit the list's world,
> write the published face, and publish by the act of creating.

The validation comment (`internal/dataentryconfig/validate.go:867`) states the
real behaviour:

> a create writes the DEFAULT face regardless (§9.4)

`create_world` only picks the world the form *opens in*, keeping the button
visible; the write still goes to the zero row. The two comments contradict each
other, and whichever way this is resolved one of them must change.

That second comment is not merely stale. It is the stated *reason*
`validateCreateWorld` skips checking whether the target world resolves a face
for the type: "there is no combination to reject". Fixing ask 1 turns a
currently-unreachable misconfiguration into a reachable one, so
`validateCreateWorld` must grow the check it currently argues it does not need.
This is the one place the fix creates new work rather than removing it.

## Notes for implementation

Verified against the tree at the time of filing:

- `GrantsVerbOnState` (`internal/acl/worldgrant.go:255`) already takes an
`entity.Face` and matches it exactly, with `OpCreate` reading `role.Create`. Ask
2 is therefore mostly plumbing: the face-granular check exists, the create path
just always hands it the zero face.
- `handleV1CreateEntity` (`internal/dataentry/write_handler.go:177`) decodes into
an anonymous struct with no `face` field, and `encoding/json` drops unknown keys
silently. That is the mechanism behind ask 4's "201 that looks like it worked".
- `bare_face` has a live consumer beyond addressing:
`internal/aclaudit/tier_b.go:153` uses it to recognise that `update:
[policy@draft]` and `update: [policy]` name the same row under `bare_face:
draft`. Removing the key removes that equivalence, an ACL-audit behaviour change
riding along with the migration.
- `metamodel.StoredFace` returns "" for the `bare_face` name and
`DeclaredFace` inverts it. Both keep working for job B; ask 3 removes only the
`def.BareFace` branch.

## Workaround atlas is using

Create, then **Herzien** (copy to `@concept`), edit, then **Vaststellen**.
Verified: `herzien-beleid` is offered with `allowed: true` on a freshly created
policy. It leaves an empty adopted version standing from creation until the
first adoption, which is the wrong posture for a policy register but is not
blocking.

## Verified against

`rela-acl-fix2` (develop plus the copy-guard fix BUG-J2E6VJ), atlas
`feature/worlds-faces` at `af61447`, Postgres store, principal resolved to a
persoon holding the CISO role.
