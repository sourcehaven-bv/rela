---
id: RR-RDPC48
type: review-response
title: Create affordance is computed with a zero face, so it over-offers for a faced type under a non-default world
finding: 'computeCollectionActions hardcodes the zero face (affordances.go: translateVerb(v, entityType, "", "")), and translateVerb''s own doc says face is REQUIRED rather than optional because "an affordance must answer the same question the write will" (BUG-Y0GNSB), with the zero value correct only for an unfaced entity or a per-collection verb. Create was a per-collection verb when that was written; worlds.<name>.create makes it face-targeted, and GrantsVerbOnState matches faces exactly, so create: ["*"] grants only the default state. Consequence: for a faced type viewed in a non-default world, a principal holding create on the default face gets the button and the POST is refused 403 by the manager. The write side is correct — createFaceForWorld refuses an undeclared world, refuses the literal default world for a faced type, refuses a declared world with no create:, and returns a face named in schema.yaml rather than a client string, so no wrong-face write is possible. Only the affordance over-offers.'
severity: minor
reason: 'Deferred rather than fixed: it is a pre-existing shortcut in computeCollectionActions, not introduced here, and it fails in the safe direction (button appears, write is refused 403). Fixing it properly means making the collection-verb translation face-aware — resolving the world''s create face the way the write path does — which changes a shared affordance helper used by the list handler and every other create surface, well beyond this ticket''s scope. This ticket is what first puts a create affordance on _views, the one world-capable surface, so the shortcut is newly reachable with a world in play; that is worth its own ticket with the BUG-Y0GNSB precedent cited, rather than a widened change here. No security consequence: the ACL ceiling''s fail-toward-less-access property is intact.'
status: deferred
---

## Finding

`computeCollectionActions` hardcodes the zero face:

```go
out[v] = svc.acl().AuthorizeWrite(ctx, translateVerb(v, entityType, "", "")).Allow
```

`translateVerb`'s own doc comment says face is "REQUIRED rather than optional:
an affordance must answer the same question the write will (BUG-Y0GNSB)", with
the zero value "correct only for a genuinely unfaced entity or a per-collection
verb."

Create *was* a per-collection verb when that was written. It no longer is:
`worlds.<name>.create` makes a create face-targeted, and `GrantsVerbOnState`
(`internal/acl/worldgrant.go`) matches faces **exactly**, so `create: ["*"]`
grants only the default state.

So for a faced type viewed in a non-default world, a principal holding create on
the default face gets the button, and the POST is refused 403 by the manager.

**The write side is correct.** `createFaceForWorld` refuses an undeclared world,
refuses the literal `default` world for a faced type, refuses a declared world
with no `create:`, and returns a face named in `schema.yaml` — never a client
string. No wrong-face write is possible. Only the affordance over-offers.

## Deferred, and why

Not introduced here: it is a pre-existing shortcut in a shared affordance
helper. It fails in the **safe** direction — the button appears, the write is
refused — so the ACL ceiling's fail-toward-less-access property is intact and
there is no security consequence.

Fixing it properly means making the collection-verb translation face-aware,
resolving the world's create face the way the write path does. That touches a
helper the list handler and every other create surface depend on, which is well
beyond this ticket.

Worth recording because this ticket is what first puts a *create* affordance on
`_views`, the one world-capable surface, so the shortcut is newly reachable with
a world in play. That earns its own ticket citing the BUG-Y0GNSB precedent, not
a widened change here.
