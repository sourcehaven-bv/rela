---
id: TKT-OPD8WQ
type: ticket
title: Entity references to faced types never resolve as mentions
kind: enhancement
priority: medium
effort: s
started: "2026-10-11"
completed: "2026-10-11"
status: done
---

## Problem

An entity reference in markdown content (an inline code span whose whole text is
an entity ID, e.g. `` `PROC-1` ``) is rendered by the SPA as a titled link when
the server lists it in the response's `mentions ` map. For every type that
declares faces the map never contains the ID, so references to faced entities
(in Atlas: beleid, procedure, document) stay bare code, also when the entity has
the face the reader's world serves.

Cause (origin/develop a6b9eb69b):

- `internal/dataentry/mentions.go:47-55 `: `collectMentions ` loads candidates
with `loadDefaultFaceHeaders `.
- `internal/dataentry/visiblereader.go:267-283 `: `loadDefaultFaceHeaders ` keeps
only rows with `h.Face.IsImplicit() `.
- `internal/entity/face.go:41-43 `: a faced type has no row at the implicit face.

Reproduced read-only on atlas (rela 26.10.7) via `GET /api/v1/_views/... `:
every procedure/policy ID in `PROCEDURE-6l21@concept ` and
`POLICY-001@{concept,vastgesteld} ` is missing from `mentions `, including
ones with a `vastgesteld ` face; unfaced types resolve.

## Goal

Resolve mentions in the request's world, the same way a bare id is resolved
everywhere else on the read path: per id, the face the world serves among the
faces the principal may read (`visibility.Resolver.ResolveIDs ` via
`visibleReader.servedIDs ` / the `idResolver ` pattern in
`viewsHandler.visibleTitles `). The link keeps its bare-id href
(`entity/<type>/<id> `): the entity page resolves a bare id in the same world,
so no `@face ` is needed.

## Scope

In scope:

- `collectMentions ` resolves candidates through the request world (both
callers: `api_v1.go ` entity GET and `views_handler.go ` view GET).
- ACL: an id whose faces the principal may not read gets no mention (no link,
no title); a redacted display title falls back to the id as today.

Out of scope:

- `ID@face ` code spans as explicit references.
- `entityReader.defaultWorldHeaders ` (neighbor titles), which documents that it
serves only the default world.
- The Lua `rela.md.entity_refs `/`resolve_refs `, which already resolve in a
world (`lua.Deps.World `).

## Acceptance criteria

1. A reference to a faced entity whose world-served face the reader may read
is in `mentions ` with that face's display title; the SPA renders it as a link
to `entity/<type>/<id> `.
2. A faced entity with only a non-default face (e.g. only `concept ` in a world
`select: [vastgesteld, concept] `) resolves to that face.
3. When the world's first choice is denied to the reader but a later face is
readable, the mention resolves to the readable face; when no face is readable,
there is no mention.
4. Unfaced references behave as before (existing mentions tests unchanged).
5. A store failure still drops all mentions without failing the response.
6. On a page bound to a world, a mention link in the read view is
`entity/<type>/<id>?world=<w> `; without a bound world it has no query.

## Scope change (2026-10-11)

Agreed with the requester after planning: when the page is bound to a world, the
rendered mention link carries that world as `?world= `, so a reader in a
non-default world opens the target in the world the mention was resolved in.
Applies to the read view (`frontend/src/utils/markdown.ts `); the editor's
entity-ref link nodes stay out of scope.
