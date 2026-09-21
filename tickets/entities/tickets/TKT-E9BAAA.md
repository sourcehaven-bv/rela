---
id: TKT-E9BAAA
type: ticket
title: Lua write bindings cannot name a face, so a faced type is uncreatable from a script
kind: enhancement
priority: high
effort: m
status: done
---

## Problem

`entity.CreateOptions.Face` (`internal/entity/writeapi.go:34`) and
`entity.RelationOptions.FromFace` (`internal/entity/writeapi.go:142`) both exist
and are honoured by the manager. The Lua bindings never set either:

- `internal/lua/runtime.go:1729` — `CreateEntity(ctx, newE, entity.CreateOptions{ID: customID})`
- `internal/lua/runtime.go:1887` — `CreateRelation(ctx, from, relType, to, entity.RelationOptions{})`
- `internal/lua/elevation.go:218` — elevated `admin.create_relation`, same empty options

`Deps.requireCreateFaceFor` (`internal/entitymanager/core.go:321`) enforces a
symmetric rule: a faced type must name a face, a faceless type must not. So on a
type declaring `faces:`, **every Lua create raises `ErrFaceRequired`**
(`internal/entitymanager/errors.go:99`) and there is no argument that can
satisfy it. A `scope: content` relation is likewise unwritable from a script.

The read side drops the face too. `EntityToTable`
(`internal/lua/runtime.go:1328`) emits `id`, `type`, `content`, `mod_time`,
`properties`, `redacted` — no `face`; `relationToTable` has no `from_face`. A
script cannot observe which face a row came from, so it could not derive a
create face from what it read even once writes accept one.

Neither `face` nor `Face` appears anywhere in `internal/lua/*.go`, and no test
pins the current behaviour.

## Scope

Design review changed this materially; `PLAN-WNTU66` carries the full reasoning.

**In scope:**

1. `rela.create_entity` accepts a face.
2. `rela.create_relation` accepts a source face.
3. **`requireRelationFaceFor` in `internal/entitymanager/core.go`** — the
relation-side counterpart of `requireCreateFaceFor`, called before the ACL
subject is built. The relation write path performed **no** scope or declaration
check (RR-9LM7T7); every existing caller computes the tail from the metamodel
instead of accepting it, so Lua would be the first to pass it unvalidated. It
**rejects a wrong face and never demands one** — requiring one broke every
caller that cannot supply one (RR-HQUW7V).
4. `admin.create_relation`, **sequenced behind item 3** (RR-3NNWNK):
`authorizeAndAudit` returns early under `bypassACL` (`manager.go:560-566`), so
no ACL subject is built and the manager check is the only validation remaining
on that path.
5. Read side: `face` on `EntityToTable`, `from_face` on `relationToTable`,
both set unconditionally (RR-QR7BH0).

**Out of scope** — the three id-addressed paths, each now resolved rather than
left unverified:

- `update_entity` already carries the face correctly via `getEntityByRef`
(`manager.go:1033` → `core.go:474-484`). Confirming test plus a doc line.
- `delete_entity` deletes the whole **family** (`anyFaceOf`, `manager.go:1414`,
and the comment at `:1403-1407`). Documented, not changed (RR-AQGG5O).
- `delete_relation` targets the **wrong edge and reports success**
(`manager.go:2022-2024`). Spun out as **BUG-YVU8CP**.
- Worlds (`writeapi.go:29-33` forbids it; `FEAT-CRWFACE` covers the HTTP form)
and the other faceless callers (`TKT-2RQMV4`).

## Approach

A trailing options table on both create bindings:

```lua
rela.create_entity("policy", {title = "P"}, "body", nil, { face = "draft" })
rela.create_relation("POL-1", "cites", "FEAT-1", { face = "draft" })
```

Chosen over a 5th positional because the signature already has four and
`CreateOptions.Prefix`/`.Variant` are equally unreachable; it matches
DEC-IYHLNF's opts-table convention on the read bindings.

**Argument handling is specified exhaustively** (RR-NWBAR5), because `GetTop()`
counts explicit trailing nils — verified against gopher-lua:

| Form | Behaviour |
| --- | --- |
| absent | no face |
| explicit `nil` | same as absent |
| a table | parse; reject unknown keys and wrong-typed values |
| **any other type** (e.g. `..., nil, "draft"`) | **raise** |

That last row is the one that matters: a bare string in the opts slot is the
mistake a real author makes, and silently ignoring it is BUG-HC6I2T's shape
occurring in the very argument this ticket adds.

Face strings go through `entity.ParseFace` (`face.go:70`), never an
`entity.Face(s)` conversion. Key presence is tested via `RawGetString` returning
non-`LNil`, **not** by emptiness — a deliberate divergence from
`write_handler.go:277`, which treats `""` as absent.

## Acceptance criteria

1. Faced type + `{face = "draft"}` creates on `draft`; the returned table's
`face` is `"draft"`.
2. The same call with no opts table still raises `face_required`.
3. Faceless type + a named face is rejected (`ErrFaceNotDeclared`); no opts
table behaves exactly as today.
4. `{face = "nope"}` on a faced type raises, naming the face.
5. `{face = 42}`, `{fce = "draft"}` and a **non-table opts argument** all
raise.
6. `create_relation` on a `scope: content` type with `{face = "draft"}` lands
the edge on `draft`; the published face does not see it.
7. A face on an `identity`-scoped relation is **refused** by
`requireRelationFaceFor`, from both the ordinary and elevated bindings. A
relation create that names NO face keeps working on every type — unlike an
entity create, a zero tail addresses a real edge rather than a missing row.
8. Existing 3-argument calls are unchanged. **`create_relation`'s 4th argument
is a declared break** (RR-Z7BI5E): previously accepted and ignored, now it must
be a table. Zero in-tree callers pass it and it was never in the documented
signature.

## Test plan

Two harnesses, because they cannot be one (see plan):

- **Mock-level** — ACs 1, 5, 6, 8. Assert the `Face`/`FromFace` the manager
received.
- **Integration** (real manager + faced `schema.yaml`) — ACs 2, 3, 4, 7. These
rules live in the manager and `mockManager` has no metamodel, so they are
unreachable through it.

**Prerequisite:** `mockManager.CreateEntity` (`runtime_test.go:165`) and
`.CreateRelation` (`:238`) currently **drop** `opts.Face`/`opts.FromFace`, so
any test written against today's double passes vacuously. Fix the mock first.

Assert the stored value, never just the absence of an error, and have every
negative assert that no write occurred.

## Risks

- **Silent misfiling** — a face parsed and dropped. BUG-HC6I2T's shape.
Mitigated by asserting at the manager boundary and by the exhaustive argument
rules above.
- **Unvalidated relation face** — certain without item 3.
- **Elevated path unchecked** — certain if item 4 lands before item 3.
- **Docs** — `docs/` is generated; see below.

## Docs

**`docs/` is a build output.** `just docs` regenerates it from `docs-project/`,
and `docs-check` (part of `just ci`) fails on any diff (RR-NP9T1J). Edit:

- `docs-project/entities/guides/GUIDE-lua-scripting.md` (:348, :351, :431)
- `docs-project/entities/guides/GUIDE-content-states.md`

then run `just docs` and commit the regenerated `docs/`. The guide must also
state the addressing asymmetry: a faced id selects the face on `update_entity`,
but `delete_entity` removes the whole family and `delete_relation` targets the
default face.

## Related

- `TKT-2RQMV4` — the same gap for SPA, sync, MCP and CLI. Item 3 hands that
ticket a solved problem instead of four copies of an unsolved one.
- `BUG-YVU8CP` — spun out of this ticket's design review.
- `BUG-64MU2Q` — threaded `FromFace` to the store, which is what makes the
`create_relation` half small.
- `BUG-HC6I2T` — made a face mandatory on a faced type, creating this gap.
- `FEAT-CRWFACE` — the world-based answer for the HTTP create form.
