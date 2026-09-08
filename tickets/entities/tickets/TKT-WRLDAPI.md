---
id: TKT-WRLDAPI
type: ticket
title: 'Worlds on the wire: schema discovery, face provenance, copy endpoint, world-capable detail read'
kind: enhancement
priority: high
effort: l
status: backlog
---

The content-states backend is complete and correct, but **most of it is not
reachable from a client**. Verified live 2026-08-23 against `/tmp/isms-demo`:
world resolution, published-world filtering, language fallback and the
read-only refusal on a published face all work — while world *discovery*,
face *provenance*, and the copy kernel itself have no HTTP surface at all.

`entitymanager.Manager.CopyState` (`copy.go:142`) has **zero non-test callers
repo-wide** — not HTTP, not CLI, not Lua, not MCP. Promote works in Go tests
and nowhere else.

This ticket adds the surface. It blocks TKT-F2D5U5 items 3-8 (the demo's
promote / go-to-draft / provenance affordances); the SPA slice that IS
buildable today proceeds in parallel on the list surface.

## Work, in dependency order

1. **`worlds` on `/api/v1/_schema`** — declared world names plus per-principal
   readability. `PermitsWorld` (`dataentry/readgate.go:53`) already answers
   per name; this needs an enumeration. Today `_schema` returns only
   `{entities, relations}`, and `metamodel.Metamodel.Worlds`
   (`metamodel/types.go:59`) has a yaml tag and **no JSON tag**.
   *Unblocks the world selector.* NOTE: `world.go:144` carries a comment
   claiming the declared worlds "are already served over the API" — that is
   FALSE; fix it.
2. **`face` + `via` on the entity GET.** `worldreader.Resolved`
   (`worldreader.go:75-101`) computes both; `visiblereader.go:114-123`
   discards them. `entity.Entity.Face` already has a JSON tag but
   `entityserializer.go:45-103` never copies it.
   *Unblocks provenance — "published" vs "default, because nothing is
   published yet".*
3. **`faces` on `_schema`'s EntityType** — a client cannot currently learn
   that `policy` has draft/published.
4. **World-capable detail read, INCLUDING relations (RULING 12).** Jeroen
   settled the shape 2026-08-23: relations resolve through the SAME world as
   the entry, per neighbour, independently. An ISMS published view links to
   published faces; a preview world with chain `[draft, published]` links to
   drafts where drafts exist; a Spanish page links to Spanish where Spanish
   exists and English where it does not. The fallback chain does this
   automatically — it is not configured twice.

   This is **not a design fork**; the previously-offered options (scope the
   entry only / omit neighbours / move to the thin endpoint) all mis-framed
   it as "how much of the neighbour question do we avoid". The answer is
   none of it.

   The machinery already exists: `worldreader.RelationReader.Neighbors` does
   the per-relation-type scope dispatch (identity types query a nil tail,
   content types the prime's face, merged) and its godoc states it exists
   so the dispatch is "UNREPRESENTABLE to omit". What is missing is that the
   detail handler does not call it: `views.go:21,159` read via
   `store.GetEntity`, which takes **no scope parameter** (`store.go:238`).

   So: scope the detail read through the world and resolve its links via
   `Neighbors`. *Unblocks every Ruling-9 affordance having a page to live
   on.*
4b. **World-enable `_views/` — and carry provenance on its collections
   (RULING 14).** This is the item that makes the SPA's DETAIL PAGE work:
   `EntityDetail.vue` reads `fetchView()` -> `GET /_views/{type}/{id}`,
   which is `_`-prefixed and blanket-refused by `worldCapablePath`. Item 4
   fixes the entity GET and list rows; the page the user actually opens
   still cannot switch worlds until this lands.

   **Provenance belongs HERE, not on the entity-GET relations map.**
   `viewResult.Collections` is `map[string][]*entity.Entity` (`views.go:16`)
   — the neighbours are already whole entities, so attaching `_world` to
   each is additive, unlike the relations map where a neighbour is a bare
   ID string (Ruling 13). This delivers "this link is a real Dutch page vs
   the English fallback" with **no extra requests**, which is what the
   Ruling 13 caveat identified as the real cost of deferring.

   **Design questions — DECIDED (Jeroen, 2026-08-24).** A traverse rule's
   `where:` evaluates against the **resolved face** (RULING 16): filtering on
   draft values while rendering published content produces a collection that
   contradicts its own page. The raw-entity behaviour documented at
   `views.go:59-64` is NOT precedent — it describes a world-less system and
   was falsified by this epic (RULING 15 corollary). Blast radius measured:
   **zero** `where:` clauses across every in-tree `data-entry.yaml`; the only
   examples are in the generated docs, so the docs update IS the migration
   (edit `docs-project/entities/guides/GUIDE-data-entry.md`, never `docs/`).

   **Scope constraint (architect, 2026-08-24):** `executeView` is a SHARED
   ENGINE with three callers — `_views`, `_sidepanel` (`sections.go:378`) and
   the command runner (`commands.go:451`), the last of which marshals view
   output to JSON and pipes it to an operator shell script
   (`exec.CommandContext(ctx, "sh", "-c", cmd.Script)`). World-resolving
   INSIDE `executeView` would silently change what an external process
   receives. The world must therefore arrive as an explicit PARAMETER, with
   the two unscoped callers naming the default world at the call site, and a
   guard test asserting they do. Default-deny, joined by an explicit
   reviewable act — `worldCapablePath`'s discipline one layer in.

4c. **Entry relations on a world-bound view.** Admitting `_views/{type}/{id}`
   to `worldCapablePath` made `handleV1Views` world-capable while
   `views_handler.go:568` still read the ungated default-world reader for the
   entry's relations — a resolved entry wrapped in DRAFT edges, the mixed-face
   response item 4 exists to prevent. Item 4b guards the call, so a
   world-bound view currently carries **no entry relations**: the honest
   placeholder, and the same posture the entity GET held between items 2 and 4.

   Closing it means resolving them through the item-4 seam
   (`worldOutgoingForEntity`) inside `handleV1Views`. Deliberately NOT bundled
   into 4b: that PR already threads a world through a shared engine with three
   callers, and the view response's relation shape differs from the entity
   GET's, so it needs its own tests rather than inheriting item 4's. A visible
   absence is preferable to a subtly-wrong second surface.

   **Guard-enumeration follow-on.** `TestWorldCapableRoutesDoNotUseUngatedReader`
   missed this because its scanned set is a manual literal and `handleV1Views`
   was not in it — the route became world-capable without the handler joining
   the guard. That is now TWICE in this epic. A manually-enumerated guard is a
   check whose failure mode is SILENCE (RULING 15): it passes while checking
   nothing. The guard should derive its set from the route table, or fail
   closed on an unlisted world-capable handler.

5. **Copies: list-by-source + invoke-by-name, and `label:` on `CopyDef`.**
   A request may only invoke a definition BY NAME (transforms-registry
   precedent); the endpoint must **re-authorize the guard server-side**
   (§9.3 rule 1 — the UI affordance is a hint, never the boundary).
   `CopyDef` (`metamodel/types.go:599-644`) has no label field, so Ruling 9's
   "operator-configurable text" has nothing to read. **This is the big one
   and needs a real design pass — it is a write endpoint.**
   *Unblocks promote / go-to-draft / create-a-face.*
6. **All-states query for the "other faces" indicator.**
   `store.EntityQuery.AllStates` exists (`store.go:294`) but is unused by
   `internal/dataentry`. Must respect the omit-unreadable-faces rule
   (Ruling 9 item 8): a face the viewer may not read is OMITTED, never shown
   as existing-but-locked.
7. **Face-aware `computeActions` (RULING 11).** Verified live: a published
   face carries `_actions:{update:true}` while `PATCH ?world=published`
   returns 422 — the affordance map SAYS writable, the write path REFUSES.
   `computeActions` (`affordances.go:127-133`) authorizes on entity id with
   no face, so it cannot express the face-granular authority TKT-C1XUA8
   PR-A gave the write path. Usability defect, not a security hole (the
   server refuses correctly), but an affordance map that lies is a trap for
   every future consumer.

## Constraints

- Items 1-3 are small and additive. Item 4 is a design decision. Item 5
  should not have its shape chosen unilaterally by an agent.
- Under a non-default world the backend currently refuses `?include=` (422)
  and emits **no relations** (`api_v1.go:794-797`, `639-643`). Per RULING 12
  both are **GAPS TO CLOSE, not decisions to preserve** — neighbour titles a
  client asks for must come from the world-resolved faces.
- **Do not add a face-enumeration path that lets a client probe which faces
  exist.** That reconstructs the existence oracle `errWorldDenied` was
  designed to close.
