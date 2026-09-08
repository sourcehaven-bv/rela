---
id: TKT-SLFURL
type: ticket
title: '`_self` on a non-bare face 404s under a configured default_world'
kind: enhancement
priority: medium
effort: s
status: backlog
---

A faced response advertises `_self: "/api/v1/policys/POL-1@published"`. That URL
does not resolve when the project sets `app.default_world`, which the shipped
worlds prototype and the documented ISMS setup both do.

## Reproduction

Verified 2026-09-04 against `prototypes/worlds/project` (which sets
`default_world: published`) through the docs harness on the postgres backend:

```lua
api{ path = "/api/v1/policys/POL-1@published", as = "editor", status = 404 }
```

That assertion PASSES — a 404 on the URL the server itself just handed out. The
same request with an explicit `?world=default` returns 200.

## Mechanism

`selfHref` (entityserializer.go:207) appends the declared face name, which is
correct and deliberate: `_self` must address the ROW the response describes, or
a client's GET-`_self`/PATCH-`_self` loop silently edits a different face. That
reasoning stands.

The break is on the read side. There is no `@` parser in the HTTP layer
(`ParseStateRef` is never called from `internal/dataentry`), so `POL-1@published`
is treated as a literal id. Under a configured `default_world` the world chain
then looks for face `published` on an id that has no such row, and 404s. On
fsstore without a default world it appears to work by accident:
`GetEntity("POL-1@published")` builds a key that happens to equal the published
face's index key. On pgstore it never works.

So the field is backend-dependent and config-dependent, which is worse than
either working or failing consistently.

## Options

1. Parse `id@face` at the route on reads and writes, so `_self` round-trips.
2. Emit `_self` as `/api/v1/policys/POL-1?world=<world>` when serving a world.
3. Keep the bare id and rely on the 422 write refusal as the only guard —
   rejected once already, for the reason in the `selfHref` doc comment.

Option 1 is the one that makes the existing `world_read_only` error hint true;
it still says "address the face directly by id (`ID` or `ID@face`)".

## Note

The `@`-URL examples were already removed from the docs rather than document a
backend-dependent accident, so this is not currently a documented promise —
but `_self` is in every faced response body, which is a promise of its own.
