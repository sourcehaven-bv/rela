---
id: BUG-Y0GNSB
type: bug
title: Faced-id write authorizes against the default face — published face writable with a bare grant
description: |-
    `ID@face` is the boundary serialization of a state reference, and the stores key their index on exactly that string (stateKey = entity.FormatStateRef). Because FormatStateRef(id, "") returns the id verbatim, GetEntity("POL-1@published") resolves to the SAME row as GetEntityState("POL-1", "published") and returns the published face with .Face populated. The write path then dropped that face: every acl.EntitySubject literal in entitymanager was built {Type, ID} with no Face, and a zero Face means "the default state". So the ACL was asked "may this principal update the DEFAULT face?" while the store wrote the PUBLISHED one.

    Reproduced end-to-end through the production router: `PATCH /api/v1/tickets/TKT-001@published` with a role holding only `update: [ticket]` returned 200 and overwrote the published face. `update: ["*"]` — the grant every admin policy holds — escalates identically, which is precisely the case GrantsVerbOnState's own doc says must not "silently acquire authority over every face". Defeats the ISMS invariant the worlds epic exists to provide (a published policy changes only by promoting a draft through an audited copy). Also reachable via MCP update_entity, CLI `rela update` and Lua rela.update_entity, which all pass a raw id string.

    Precondition: a principal on the AllowAll read branch (holds a global read grant on the type). Scope is PATCH/UPDATE; DELETE authorized past the ACL but then failed in the store, so it was a failed no-op rather than an erasure. NOT related to worlds: no `?world=` is involved, so attachWorld's `world_read_only` refusal never applies — the face rides in the PATH.
priority: high
why1: A PATCH to `POL-1@published` was authorized against the default face, because the acl.EntitySubject built for the write set only {Type, ID} and left Face at its zero value, which acl.EntitySubject documents as meaning "the default state".
why2: The subject was built from an entity that HAD the correct face on it (the store had already resolved `POL-1@published` to the published row and populated e.Face) — the field was in scope one line away and simply not copied.
why3: acl.EntitySubject.Face was added later, for the copy kernel (TKT-C1XUA8), and only the copy path was updated to populate it; the seven pre-existing construction sites were never revisited, because adding a field with a meaningful zero value is a source-compatible change that produces no compile error at any existing site.
why4: The zero value was chosen to mean "the default face" specifically so the field could be added without touching existing callers — a deliberate compatibility decision that made the omission silent rather than loud, and turned "forgot to set it" into "asserted the wrong face" instead of "failed to build".
why5: 'Systemic: an authorization subject is assembled field-by-field at each call site by convention, so a new security-relevant dimension (face) can be added to the subject type without any mechanism forcing existing sites to answer for it. The ACL unit tests set Face as an explicit struct field, so they exercised the matcher correctly and passed — the defect was never in the grant logic but in the subject the production code failed to build, which no test drove. Same class as BUG-K6FEVB: authorization enforced by convention at each call site rather than by construction at a mandatory chokepoint.'
prevention: 'P1 (implemented): every acl.EntitySubject literal in entitymanager now names its Face, sourced from the entity actually being written. P2 (implemented): TestEveryEntitySubjectNamesItsFace source-scans the package and fails on any subject literal omitting Face, with a per-literal `facesubject:no-face` opt-out requiring a reason — a per-file exemption was rejected as too coarse, since manager.go holds six literals of which only rename is legitimately faceless. P3 (implemented): dataentry.translateVerb now takes the face as a required parameter, so the compiler forces every affordance/write call site to answer; this also fixed _actions reporting update:true on a face the principal could not write. P4 (implemented): ApplyEntity rejects a body face differing from the stored face (ErrFaceImmutable), mirroring the existing ErrTypeImmutable guard from BUG-ZWTDH9 — the sync body can set Face via its JSON tag. P5 (not done, follow-up): make Face a required constructor argument on acl.EntitySubject so the invariant is enforced by the type rather than by a source scan; wider change, crosses packages.'
status: backlog
---

Found while working up the write-world asymmetry for FEAT-9CD2MX (PR #1452),
but **independent of worlds** — it needs no world, no `?world=` and no SPA.

## Reproduction

`internal/dataentry/acl_facedid_write_test.go`. With the fix reverted, both
subtests fail on the status AND on the on-disk bytes:

```
PATCH POL-1@published = 200; a grant that names no face must not reach the published face
published face title = TAMPERED, want PUBLISHED-ORIGINAL (the denied write reached the store)
```

The test asserts the stored bytes, not just the status code — a denial that
still wrote would be the worst outcome, and is exactly what the pre-fix code
did while returning 200.

## Why the existing suite missed it

`internal/acl`'s face tests (`facesubject_test.go`, `worldgrant_test.go`) set
`Face` as an explicit **struct field**, so they exercise `GrantsVerbOnState`
correctly and pass — the grant matcher was never wrong.
`internal/dataentry/facegrant_test.go` is entirely read-side. Nothing drove a
WRITE addressed to a faced id.

## Not covered by attachWorld

`attachWorld` refuses an explicit `?world=` on a non-GET with
`world_read_only`. This attack carries no world at all, so that refusal never
fires. The two are independent — which is why the world guard did not blunt
this.

## Second-order finding (separate, not fixed here)

`internal/acl` never calls `metamodel.StoredFace` and structurally cannot
(arch-lint forbids `acl → metamodel`). So under `bare_face: draft` the stored
coordinate is the zero face while the grant says `"draft"`, and
`update: [policy@draft]` — the form `docs/acl-security.md:163` and
`docs/acl-overview.md:174` both recommend — matches nothing and denies the
draft face it is supposed to grant. Verified by execution. It **fails safe**
(denies rather than over-permits), so it is not part of this fix. `aclaudit`'s
B11 does not catch it: `HasFace` is a declared-name lookup with no
`StoredFace` mapping. Worth its own bug.
