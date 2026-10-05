---
id: RES-Y6JA37
type: research
title: Should faces be intrinsic to the data model (every type has a face, bare-id reads removed)?
summary: rela always has faces and worlds; omitted config is generated as the operator would have written it, explicit config replaces generation. Faceless types get an implicit face at ""; explicit entity/face levels; typed entity.Ref; one gated resolver; no zero-value EntityQuery; generated default world (all faces, declaration order) only when no worlds are declared; unqualified write grant on a faced type is a load error.
status: done
---

## Problem

Faces were added to a data model built on one record per id. The APIs still
treat "the record with no face" as the normal case, so every surface must opt in
to face handling. Surfaces that do not opt in fail quietly: a 404, an empty
list, or an access check made against the wrong face.

This keeps producing bugs of one shape: BUG-64MU2Q, BUG-OOZBBK, BUG-VFHUWO,
BUG-R1PQY9 and BUG-CTUW2N. The face-awareness inventory
(`.ignored/face-awareness-inventory.md`) rated 157 features: 41 ignore faces, 32
handle them partly, and none has e2e coverage with faces. Fixing these one route
at a time repeats the pattern that caused them.

The question: should faces become intrinsic to the data model, so that every
type has at least one face and no API can address "the record with no face"?

## Context

### How the model got here

- RES-NH3P12 (the original faces research) designed faces as an overlay on a bare "live" entity; type-wide grants covered that bare row. The bare-id-first APIs date from that design.
- BUG-HC6I2T (#1557) removed the bare row for faced types, so a faced type now stores rows only under face names. The APIs kept their shape.
- DEC-0VGTF3 fixes `entity.Face` as an opaque canonical token stored in one column or key. Re-keying stored faces is a job for the data-migration system.

### The current code

| Area | Current state |
|---|---|
| Addresses | `entity.Face` is a string, and `""` means the default state (`entity/face.go:33`). No typed (id, face) address exists; addresses cross boundaries as `string`, and `ParseStateRef`/`FormatStateRef` convert them. `default` is not a reserved face name. |
| Store | `GetEntity(id)` means `GetEntityState(id, "")`. It has 161 non-test call sites, 67 of them in storetest helpers (about 94 production). `dataentry.entityReader.getEntity` adds 15 more. `GetRelation`, `RenameEntity` and the four attachment methods have no face-aware sibling. `DeleteEntity(id)` deletes the whole family. Four backends plus two tx views implement `store.Store`. |
| Storage | pg and sqlite use primary key `(id, face)`; fs and mem key rows by `id@face`. The pg read indexes are partial (`WHERE face = ''`, migration 0014), so faced rows never use them. |
| Relations | The tail carries a face (`FromFace`); the head is entity-level (no `ToFace`). Identity-scoped edges are forced to the zero tail (`requireRelationFaceFor`). |
| Versions | Lineage is keyed by `(entity_id, face)`. `ListVersions(id)` and `GetVersion(id)` read the zero face; `StateHistoryReader` is the face-aware API. |
| Attachments | Keyed by `(entity_id, property, file_name)` with no face. On `AttachFile`, fs/mem check existence by key (a bare id misses), while pg/sqlite accept any face. |
| Worlds | The default world is the zero `WorldScope`, which resolves every type to `""`, so a faced type is absent (`store/world.go:152`, `docs/content-states.md:228`). `EntityDef.Faces` is a map, so no primary face exists and declaration order is lost. |
| ACL | A type-wide read grant covers every face. A type-wide write grant and `*` cover only the zero face (`acl/worldgrant.go:240-289`), so on a faced type they authorize nothing. The row gate keys on the bare id; the face gate (`FaceAllowed`/`faceReadable`) must be called after the lookup. |
| Lua, MCP | Reads accept `ID@face` through `ScriptReader`/`UnrestrictedReader`. Delete, rename, relation writes and attachments are bare-id only. |
| Guard tests | `acl/ceilingguard_test.go` (regex plus exemption list) and `dataentry/world_test.go:367` (`go/ast`) are the precedents for mechanically forbidding a call. |

### Prior art elsewhere

- **Drupal** gives every entity a language code. Untranslatable entities get a reserved "not specified" code, so all entity APIs are language-aware and no language-less code path exists. This is the implicit-face idea.
- **Strapi v5**'s Document Service addresses content by `documentId` plus `locale` and `status`. When draft/publish is disabled, status still exists but has a single value. That is two levels: the document, then its variant.
- **Sanity** stored drafts as separate documents under a `drafts.` id prefix. Every query then had to handle two id forms, which is the same bug class as here. It later added "perspectives" (published, previewDrafts, raw) chosen per request; these match rela's worlds and `AllStates`.

## Options

### A. Keep the model; add guard tests and fix each route

Add a guard test forbidding `GetEntity`/`getEntity` outside an allowlist that
shrinks over time. Fix the backlog bugs route by route.

- Pros: smallest change; stops new cases right away.
- Cons: the zero face stays the default in the store, in `EntityQuery{}` and in the default world. Faced types stay invisible in the default world. The write-grant asymmetry stays. Each fix still decides by hand which level it acts on.
- Effort: S for the guard test, then M per backlog bug.

### B. A typed address and one resolver, with faceless types still special

Add `entity.Ref{ID, Face}`. Store reads take a `Ref`. `internal/visibility` gets
one resolver that returns a row that has passed both the row gate and the face
gate. Faceless types keep the zero face as a special case.

- Pros: the compiler finds every caller, and the face gate can no longer be forgotten.
- Cons: "faceless" and "faced" remain two code paths, and tests on faceless types still do not exercise face handling. The default-world and write-grant problems remain.
- Effort: L.

### C. Faces intrinsic to the model (recommended)

Every type has at least one face. A faceless type has one implicit face, stored
at the existing `""` coordinate, so no data moves. Model two levels explicitly:

- **Entity level:** id, type, existence and the row gate, identity-scoped relations and relation heads, stored attachment bytes, rename, and deleting the whole family.
- **Face level:** properties, body, content-scoped relation tails, attachment references (which files the face shows), version lineage, and the face gate.

Rules:

1. A face-level API takes `entity.Ref`. An entity-level API takes the id and states that it covers every face; one that changes several faces authorizes each face it touches.
2. The wire format is unchanged. A faceless entity is addressed as `ID`, shorthand for its implicit face. For a faced type, a bare `ID` is resolved by the world.
3. Reads go through one resolver in `internal/visibility`, whose modes are: an explicit `Ref`, a world, or the whole family.
4. `EntityQuery` must choose its faces: a world, `AllStates`, or explicit faces. The zero value is invalid rather than meaning "no face", so no surface silently loses faced types.
5. A type has no primary face; choosing a face is a world's job. When the operator declares no worlds, rela generates one world, `default`, that selects every face of each type in declaration order; a faceless type resolves to its implicit face. When the operator declares any world, nothing is generated and no `default` world exists. The reader's face grants trim the candidates before the world ranks them (`docs/content-states.md:349`), so a `published`-only reader still sees the published face when `draft` is declared first.
6. An unqualified write grant on a type that declares faces is a load error; it must name `type@face`. A type-wide read grant keeps covering every face.

- Pros:
  - Removes the bug class; a zero-face read has no API to go through.
  - Tests on faceless types exercise the same code path as faced types.
  - Faced types become visible in the default world.
  - The write-grant asymmetry becomes a load error instead of a silent no-op.
  - Adding faces to a type becomes renaming its implicit face, which largely replaces `migrate_face`.
- Cons:
  - It touches the store interface (four backends), about 110 call sites, the ACL grant meaning and the default-world behaviour.
  - Two behaviour changes need an operator decision (see "Open decisions").
- Effort: XL overall; staged below.

### D. A literal `default` face name, with stored data re-keyed

Like C, but faceless rows are re-keyed from `""` to a reserved name such as
`default`.

- Cons:
  - Rewrites every row in four backends and every relation tail.
  - Changes addresses on the wire, and clashes with operator face names unless reserved.
  - DEC-0VGTF3 makes the stored token opaque, so the internal name gains nothing.
- Rejected.

## Recommendation

Adopt **C**, staged so that each stage ships on its own and stops a class of
bugs.

**Stage 0 (now, independent of this decision):**
- Fix BUG-1YN750 (family delete authorizes one face) and BUG-8J3LSB (documents skip the face gate).
- Add a `go/ast` guard test forbidding `store.GetEntity`, `entityReader.getEntity` and `bareEntityID` outside an allowlist that pins the current count and can only shrink.
- Make the faced fixture (`facedApp`/`seedDeclaredFaceTicket`) the Go default.
- Build the e2e fixture with faces (TKT-WCMW47).

**Stage 1: address and resolver.**
- Add `entity.Ref` and the resolver in `internal/visibility`, with explicit, world and family modes, returning a row gated by both the row gate and the face gate.
- Migrate `dataentry`, `mcp` and `lua` onto it.
- Fix BUG-CTUW2N, BUG-BZQQDP, BUG-4SYAA6 and BUG-FYEEVX on the resolver instead of per route.

**Stage 2: store API.**
- Replace `GetEntity(id)` with a read taking `Ref`. Give `GetRelation`, `RenameEntity` and attachments explicit entity-level signatures.
- Make the zero `EntityQuery` invalid.
- Align the attachment existence check across backends.
- Drop the `face = ''` partial pg indexes.
- Compile errors list the remaining callers; fix BUG-J3PBFN and BUG-95W7MV here.

**Stage 3: semantics.**
- Record face declaration order at load (`FaceOrder`, following the `PropertyOrder` precedent at `metamodel/types.go:305`), because `EntityDef.Faces` is a map.
- Generate the `default` world from that order only when no worlds are declared, and generate a missing `default_world` as the first declared world; fix BUG-7MB1D5 and BUG-4NQ2JF here.
- Reject an unqualified write grant on a faced type at load.
- Update `docs/content-states.md` and `docs/acl-security.md`.

Tradeoffs accepted: a broad mechanical migration of the store API, and two
behaviour changes that existing deployments will notice, in exchange for
removing the class rather than its instances.

### Decided (2026-09-28)

0. **Governing principle: rela always has faces and worlds.** An operator may omit their configuration for simple projects. rela then generates exactly the configuration the operator would otherwise have written, and uses it. Once the operator writes that configuration, it is used as written and nothing is generated or mixed in.

1. **No primary face.** Choosing a face is a world concept; a type declares faces, not a preference among them.
2. **An unqualified write grant on a faced type is a load error.** It must name `type@face`. This fails toward less access, and today such a grant authorizes nothing anyway, so no working policy loses access.
3. **The default world is generated, and only when no worlds are declared.** It selects every face of each type in declaration order. When worlds are declared, `default` does not exist, and a missing `default_world` key is generated as the first declared world. Behaviour change: faced types become visible in the default world, at the first declared face the reader may read. Face-scoped read grants still gate every row.
