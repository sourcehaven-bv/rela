---
id: BUG-OOZBBK
type: bug
title: Relation-history route never parsed the source address, so a faced relation's history 404'd and served empty meta
description: The relation-history route used the {from} path segment verbatim instead of parsing it, so on a faced type the ACL row gate and the live-source lookup both received an ID@face string that matches no row. An ordinary reader got a 404 on pgstore, and a live relation's meta was served empty (properties redacted from everyone). Invisible on the default build because fsstore/memstore key their index on the state reference, so the suffixed string accidentally resolves there.
priority: high
effort: s
why1: On a faced type the SPA sends `ID@face` in the relation-history `{from}` segment, but the handler split the path and used the segment verbatim. The ACL row gate received the suffixed string, which matches no row, so an ordinary reader got a 404 on pgstore.
why2: The same unparsed string reached `serveRelationHistoryVersion`'s live-source lookup, which missed and took the deleted-source branch — serving empty `meta` for a live relation, so its properties were redacted from every caller.
why3: The route was written before faced types could carry relation history, so `{from}` was only ever a bare id. BUG-64MU2Q made faced relation writes reachable, which made faced relation history reachable, and nothing connected the two.
why4: Address parsing is a per-route obligation with no structural enforcement. `parseEntityRef` exists and every faced-aware route calls it, but a route that forgets compiles, passes its tests, and fails only against a faced type — which no relation-history test used.
why5: The address type is a string at the route boundary. A bare id and an `ID@face` address are the same Go type, so the compiler cannot tell a parsed address from an unparsed one, and the mistake is invisible at every call site that matters.
prevention: 'A route taking an entity address must have a test that drives it with a FACED address and asserts the right row was reached, not merely that the route answered (measure faced-route-address-parse-test). Bare-id tests cannot see this class of bug, and the default build hides it: fsstore and memstore key their index on FormatStateRef, so a suffixed id accidentally resolves, while pgstore looks up by (id, face) columns and fails. The deeper fix is a distinct address type at the route boundary — today a bare id and an ID@face address are both `string`, so the compiler cannot tell a parsed address from an unparsed one, which is why this recurred after BUG-64MU2Q. That is the TKT-80EWGM direction (make the mistake unavailable rather than discouraged); the test-per-route is the cheap mitigation until it is worth doing.'
status: done
---

## Symptom

Opening relation history for a relation on a **faced** entity type:

- an ordinary reader gets a 404 on the pgstore backend, indistinguishable from
a missing relation;
- where the read does resolve, the relation's `meta` is served empty, so its
properties appear redacted to everyone.

Restoring such a relation created a SECOND edge on the default tail, leaving the
addressed edge untouched.

## Cause

`internal/dataentry/relation_history_handler.go` split `r.URL.Path` and used the
`{from}` segment verbatim:

```go
from, relType, to := parts[1], parts[2], parts[3]
```

`parts[1]` is an ADDRESS, not a bare id. `selfHref` hands out `ID@face` for a
faced type and the SPA passes it through unchanged
(`frontend/src/api/history.ts` `relPath` → `RelationCards.openRelationHistory`,
whose `entityId` is the faced address). Percent-encoding preserves it: `%40`
decodes back to `@` in `URL.Path`.

Two consumers then received a string that names no row:

1. `authorizeRelationHistoryRead` → `a.reader.getEntity(ctx, from)`.
`getEntity` passes straight to `store.GetEntity`, whose contract is "the bare id
addresses the default state". On pgstore the lookup is by `(id, face)` columns,
so a suffixed string matches nothing → `fromLive=false` → the deleted-relation
branch → global `PermHistoryRead` required → 404.
2. `serveRelationHistoryVersion` → the same lookup for the live source, whose
miss means "deleted source, serve no meta".

On fsstore/memstore the lookup accidentally succeeds, because their index key IS
the state reference — the exact coincidence `entityreader.go` already documents
as a bug source. So the defect was invisible on the default build.

## Fix

Parse the segment with `parseEntityRef`, like every other faced-aware route: the
bare id feeds the ACL gate, the face selects the tail. A malformed address is
the uniform not-found, since a syntactically impossible address cannot name a
row.

Fixed in `5a0800cf` alongside TKT-JAROC3's read-side work, which is what
surfaced it. Pinned by `TestRelationHistory_FacedAddressReadsItsOwnTail` and
`TestRelationHistory_BareAddressReadsTheDefaultTail` — both directions, because
a one-sided test passes against a handler that reads whichever tail sorts first.

## Prevention

See the measure `faced-route-address-parse-test`. This is the SECOND occurrence
of the shape (BUG-64MU2Q was the first, on the write path), which is what makes
it worth a structural answer rather than another one-off fix. The honest
statement of the root cause is that `string` cannot distinguish a parsed address
from an unparsed one, so the compiler helps nobody; the cheap mitigation is a
test per faced-capable route, and the real fix is a distinct address type at the
route boundary.

## Related

- **TKT-JAROC3** — the read-side work that surfaced this.
- **BUG-64MU2Q** — the same unparsed-address shape on the relation WRITE path.
- **TKT-0VJ0HV** — default-tail assumptions on the single-relation routes.
