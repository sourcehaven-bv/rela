---
id: BUG-G2BASF
type: bug
title: Commands and actions are unreachable on entity types that declare faces
description: 'On a type declaring faces:, the SPA suppresses every command whenever a non-bare face is served (EntityDetail.vue:199), and a faced type has no bare face, so a context: entity command has no URL at which its button renders. The backend already resolves ID@face for entity-context commands (commands.go:427-437); the suppression is purely render-time and was written for a context: view problem. Actions have no entity-detail surface at all: they bind only to a list''s actions: or a navigation entry, and the NextActionOffer.action path is filtered out client-side (NextActionOffers.vue:32). Together this makes "regenerate this document''s draft face" inexpressible for exactly the content type faces exist to model.'
priority: high
effort: m
why1: 'On a faced type no command button ever renders: frontend/src/components/entity/EntityDetail.vue:199 returns [] whenever servedFace is set, and a faced type is always served at a face (BUG-HC6I2T removed the bare face), so the else branch is unreachable at every address.'
why2: 'The gate''s stated rationale was false for EVERY command context. It claimed a command receives the bare face''s content while the reader looks at another face, citing defaultViewWorld(). But viewworld.go:150 short-circuits on `if w.isDefault() || entry.Explicit` — an explicit ID@face is served literally under every world — and the entity branch parses the address the same way (commands.go:427-437). The mismatch the comment described existed only because this component discarded the face before sending it (bareEntityId at EntityDetail.vue:2448), so the gate was justified by a defect in its own caller. NOTE (RR-114UN5): that is a statement about address RESOLUTION only. The two contexts are NOT equivalent on AUTHORIZATION — the view branch row-gates, face-gates and redacts; the entity branch did none of those, and the suppression was incidentally containing that. Both halves had to be fixed together.'
why3: The gate was not authored against the face model; it was mechanically re-keyed onto it. It entered as readOnly (isWorldBound && !worldAbsent) in e0187047 — a WORLD predicate, true only while a world was pinning the view — and TKT-SLFURL (f9b0052f) rewrote it to servedFace, a FACE predicate, in the same commit that made the face part of the address. A world is optional, so the old predicate left a reachable else branch; a face on a faced type is not, so the new one does not. The substitution silently converted a partial restriction into a total one.
why4: Nothing could observe the change in kind. The gate encodes an assumption about the addressing model — that a bare address exists to fall back to — which is stated nowhere it can be checked, and the test suite asserts what the gate suppresses rather than what stays reachable. A suppression-only assertion passes identically whether the fallback address exists or not, so the commit that removed the fallback broke no test.
why5: 'Faces were added as an addressing dimension without the affordance surfaces being re-derived against it, and the deferral was recorded as a comment rather than as work. The comment at :185-198 says the meaning of a face-bound command is "deliberately another ticket" — but no ticket exists, so the stop-gap is indistinguishable from a decision and a total outage is indistinguishable from a deliberate restriction. This is the same failure mode as BUG-64MU2Q, whose why5 names it exactly: a capability gap closed with prose that looks finished. There, the prose was a refusal with stale advice; here it is a silent suppression, which is worse — a refusal at least reaches the user.'
prevention: |-
    When a new addressing dimension (a face, a world, a tenant) is introduced, enumerate the affordance surfaces keyed on the OLD addressing assumption and re-derive each against the new one, rather than re-keying the predicate to whatever new variable is nearest in scope. The re-key at f9b0052f was a one-token change (readOnly -> servedFace) that altered the predicate's kind from optional-and-partial to mandatory-and-total; only enumeration catches that, because the diff looks like a rename.

    Second, and learned during implementation (RR-114UN5): when REMOVING a gate, audit what the gate was incidentally containing, not just whether its stated rationale holds. The rationale here was about address RESOLUTION and was genuinely obsolete; the suppression was separately the only thing keeping a faced address away from an ungated, unredacted store read. Checking only the half that supports the change is how a latent hole becomes a reachable one. The asymmetry was documented one screen above the edited code (buildViewInput's comment names the view payload as row-gated and field-redacted) and was read past. Ask: what reaches a new code path that could not reach it before?

    Third, the measure carried here: test affordances for REACHABILITY on the new dimension, not just for suppression on the old one (faced-type-affordance-parity-test). Fourth: a stop-gap comment that names a deferral must cite a ticket ID, so 'not yet built' stays distinguishable from 'decided'; the comment at EntityDetail.vue:185-198 said the real meaning was 'deliberately another ticket' and named none, which is exactly how BUG-64MU2Q's gap stayed invisible until a user hit the dead end.
status: done
---

## Symptom

On an entity type that declares `faces:`, a `context: entity` command's button
**never renders, under any world**, and an action cannot be bound to an entity
detail page at all.

## Reproduction

1. Declare an entity type with `faces: {concept, vastgesteld}` and a world
`actueel: {select: [vastgesteld, concept], create: concept}`.
2. Add `commands: { my-cmd: { context: entity, available_on: {entity_types: [document]} } }`.
3. Open any entity of that type in the web app.

The button never appears. `GET
/api/v1/_commands?page_type=entity&entity_type=document` returns the command and
the ACL authorizes it, so the server is not the gate.

Environment: rela HEAD (2026-09-18, `6f02d2d2f`), PostgreSQL backend,
`default_world: actueel`. Not environment-specific — the gate is client-side, so
every backend and every world is affected. The only condition that matters is
that the type declares `faces:`.

## Cause 1: the SPA hides every command whenever a face is served

`frontend/src/components/entity/EntityDetail.vue:199`

```ts
const commands = computed<Command[]>(() => (servedFace.value ? [] : loadedCommands.value))
```

For a **faced** type this is total, not partial. A faced type has no bare face
to fall back to (BUG-HC6I2T removed `bare_face`), so there is no address at
which the buttons appear.

### The stated rationale was false for EVERY context

The comment at `EntityDetail.vue:185-198` justifies the gate by saying the
script receives the bare face's content while the reader looks at another face,
citing `defaultViewWorld()`.

The initial report read this as a `context: view` truth over-applied to
`context: entity`. Checking it during implementation showed it is not true of
`view` either. The **entity** branch parses the address
(`internal/dataentry/commands.go:427-437`):

```go
// An ADDRESS, as everywhere an id is accepted: `ID` or `ID@face`.
id, face, perr := entity.ParseStateRef(entityID)
entityDomain, err := svc.Store.GetEntityState(r.Context(), id, face)
```

and the **view** branch short-circuits on an explicit address before the world
is ever consulted (`internal/dataentry/viewworld.go:150`):

```go
if w.isDefault() || entry.Explicit {
```

An explicit `ID@face` is served literally under every world — the caller named
the row — and the resolved face is then ACL-checked by `faceReadable`, so a
principal granted only `policy@published` cannot reach the draft through it.

So the mismatch the comment describes existed **only because this component
discarded the face before sending it** (`bareEntityId`, `EntityDetail.vue:2448`,
computed at `:156`). The gate was justified by a defect in its own caller: the
page sent the wrong row, then hid the button on the grounds that the wrong row
would be sent.

### How the predicate changed kind

`git log -L 199,199` shows the gate was never authored against the face model.
It entered as a **world** predicate in `e0187047` (FEAT-9CD2MX):

```ts
const readOnly = computed(() => isWorldBound.value && !worldAbsent.value)
```

and TKT-SLFURL (`f9b0052f`) re-keyed it to `servedFace` — a **face** predicate —
in the commit whose own subject is "the face is part of the address". A world is
optional, so the old predicate left a reachable `else`; a face on a faced type
is mandatory, so the new one does not. A one-token substitution turned a partial
restriction into a total outage, and the diff reads as a rename.

## Cause 2: actions have no entity-detail surface

Actions can only be referenced from a list's `actions:` (bulk, keyboard-driven)
or a navigation entry (`config.go:705`, `config.go:1217`). There is no binding
on `entity_views`, forms, or detail pages, and `EntityDetail.vue` never imports
`runAction` (the only importers are `useListActions.ts`, `relaBridge.ts` and
`Sidebar.vue`).

`NextActionOffer` declares `action:` and `set:`
(`internal/dataentryconfig/nextaction.go:399,403`), and those offers are
per-entity, but `frontend/src/components/NextActionOffers.vue:32` filters them
out:

```ts
.filter(({ offer }) => offer.navigate || offer.acknowledge || offer.pick_one)
```

A configured `action:` on a next-action source therefore validates server-side
and renders nothing. That reads as an unimplemented path, not an intentional one
— `Confirm` at `nextaction.go:405-407` documents itself as "only meaningful with
Action or Set", so the config half was written expecting a renderer.

## Why this matters

The use case is a **generated controlled document**: a rendered-from-the-graph
document kept as a single entity, refreshed periodically, and adopted via a
guarded copy. Faces model this exactly, and `vaststellen` as a copy works well.

What cannot be expressed is the other half: a button that regenerates the draft.
The generator must write the `concept` face, and the natural trigger is a button
on the document. Today:

- A **command** can do the work (`rela update` accepts the fused address) but
can never be shown.
- An **action** can be shown on a list but cannot reuse the render logic: the
Lua sandbox runs with `SkipOpenLibs` (`internal/lua/runtime.go:415`), so there
is no `require` and no `read_file` binding.

The remaining option is to drive it from outside the app, which defeats having
the document in rela.

## Fix plan

Three independent changes. (1) is the outage and stands alone; (2) and (3) are
the capability gaps behind the use case.

### 0. Gate the entity-context command read (found in review, RR-114UN5)

Deleting the suppression made a pre-existing hole REACHABLE, so it is fixed
here. `handleCommandExec`'s `entity` branch read `svc.Store` directly with no
row gate, no face gate and no field redaction — while the `view` branch has all
three. The suppression was the only thing keeping a faced address away from it.

`commandHandler.entityReadable` now applies the world block, the face-blind row
gate and `faceReadable`, in `getVisibleRef`'s order, and the payload goes
through `redactEntity` before `buildEntityInput`. Every refusal is the uniform
not-found. See RR-114UN5 for why the original justification checked only the
half that supported the change.

### 1. Delete the gate, and send the address (the outage)

The plan was to NARROW the gate to `context: view`. Checking that assumption
first — as the plan required — showed `view` resolves an explicit address too
(`viewworld.go:150`, above), so there is nothing left to suppress. The gate is
**deleted**:

```ts
const commands = computed<Command[]>(() => loadedCommands.value)
```

Then stop discarding the face: pass the **served address** (`servedRef`) to
`CommandModal` rather than `bareEntityId`. `commands.go:427-437` already parses
`ID@face`, so the read needs no backend change. Sending the address is what
makes rendering the button honest — it is the half that removes the mismatch the
old comment described.

For the script contract, add `RELA_ENTITY_FACE` and `RELA_ENTITY_REF` beside
`RELA_ENTITY_ID` (`commands.go:880-881`), with `RELA_ENTITY_ID` staying **bare**
so a face-unaware script is untouched. The stdin JSON needs nothing:
`buildEntityInput` marshals `*entity.Entity` whole and `entity.Entity.Face`
already exists (`entity.go:63`).

### 2. An entity-detail surface for actions

Implement the `NextActionOffer.action` branch in `NextActionOffers.vue:32` — the
smaller of the two options, and the one the config already anticipates.
`runAction` is already the right shape (`api/actions.ts:20`), and the offer is
per-entity, so the entity id is in hand. The entity **type** is deliberately not
sent: the server reads it off the stored row and ignores a caller-supplied one,
because a claimed type is forgeable (BUG-ZWTDH9).

`confirm: true` is honoured — an offer that mutates the graph on one click with
no undo is not the default that should ship.

`set:` is **not** implemented. Unlike `action` it has no endpoint behind it, so
a button would have nothing to call; faking it client-side out of a PATCH would
put the interpolation rules in the wrong layer. Left to the ticket that gives it
a server path.

An `actions:` field on `entity_views` is the larger, more general option. Worth
a separate ticket if the next-action surface proves too narrow a place to hang a
"regenerate" button; not needed to close this bug.

### 3. Expose the face to Lua

Add `face` to `EntityToTable` (`runtime.go:1325-1368`), beside `id` and `type`.
Empty string for a faceless type, so no script breaks. Without it a
face-triggered action cannot tell which face invoked it, which makes (2) useless
for the actual use case.

### Regression test

`faced-type-affordance-parity-test` (linked via `adds-measure`): on a **faced**
fixture, assert each detail-page affordance is reachable and that the command
receives the served address. Asserting that the guard fires is exactly what
failed to catch this.

## Out of scope

- **Creating into a face from Lua.** `luaCreateEntity` hardcodes
`entity.CreateOptions{ID: customID}` (`runtime.go:1730`) with no `Face`, so
scripts cannot bring a faced entity into being. That is the Lua instance of
**TKT-2RQMV4**'s client-half gap, not a separate defect. Referenced, not
duplicated.

## Related

- **TKT-2RQMV4** — the create-into-a-face client gap, above.
- **BUG-64MU2Q** — the same shape on the relation write path: store-side
support complete, client half deferred, surfaced to the user as a dead end. Its
why5 names this failure mode exactly.
- **TKT-K3W8VD** — CLI corpus commands skip non-bare faces; same assumption.

## Documentation note

`docs/content-states.md:866` reads:

> Only the HTTP API can create into a face. The web app's create forms, `rela
> create`, the MCP `create_entity` tool and the Lua `create` binding all name
> no face, so they reach faceless types only.

Correct for **create**, but the paragraph leaves the impression that updates are
equally restricted. They are not: `rela.update_entity("POL-1@concept", …)`
works, because `PatchEntity` parses the fused address
(`internal/entitymanager/manager.go:1024-1033`, tested at
`internal/entitymanager/facedwrite_test.go:388`). Worth splitting into separate
create and update sentences — the asymmetry is real and currently reads as
accidental.
