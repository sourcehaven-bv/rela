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
2. **`pointer` + `via` on the entity GET.** `worldreader.Resolved`
   (`worldreader.go:75-101`) computes both; `visiblereader.go:114-123`
   discards them. `entity.Entity.Pointer` already has a JSON tag but
   `entityserializer.go:45-103` never copies it.
   *Unblocks provenance — "published" vs "default, because nothing is
   published yet".*
3. **`pointers` on `_schema`'s EntityType** — a client cannot currently learn
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
   content types the prime's pointer, merged) and its godoc states it exists
   so the dispatch is "UNREPRESENTABLE to omit". What is missing is that the
   detail handler does not call it: `views.go:21,159` read via
   `store.GetEntity`, which takes **no scope parameter** (`store.go:238`).

   So: scope the detail read through the world and resolve its links via
   `Neighbors`. *Unblocks every Ruling-9 affordance having a page to live
   on.*
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
   no pointer, so it cannot express the face-granular authority TKT-C1XUA8
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
