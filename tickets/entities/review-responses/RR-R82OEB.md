---
id: RR-R82OEB
type: review-response
title: A resolved reference title outlived a revoked read grant
finding: 'resolveAttrs treated ''no mentions map at all'' and ''map present but this ID absent'' identically, both keeping whatever title the node already held. The second case includes a reference that resolved on one load and whose read access was then revoked: a reload whose map omits the ID left the old title rendered. makeRefResolver already distinguishes the two (undefined vs a resolver returning null); the plugin just did not use it.'
severity: significant
resolution: Added a view-only `resolvedFromServer` attribute. No map at all keeps what the node holds; a map that declines an ID clears a server-resolved title to the bare ID, while still preserving a picker-supplied title on a just-inserted reference that the server has not yet weighed in on.
status: addressed
---

## Finding

`entityRefResolution.ts` returned `keepExisting(node)` for both "the resolver is
undefined" and "the resolver returned null". Those mean different things, and
`entityRefResolver.ts:19-26` documents the distinction: undefined means the
response carried no mention data, null means the server considered the ID and
declined to resolve it.

Conflating them meant a title could outlive the grant that produced it. A
reference resolves on one load; the entity's ACL changes; a reload produces a
map without that ID; the node keeps the old title and goes on displaying it.

## Severity

Significant rather than critical. The title being retained is one the server had
already sent this principal, so nothing undisclosed leaks. But "never derive a
title from the graph" quietly became "never derive, but cache indefinitely", and
the editor showed a title the server had stopped vouching for.

## Resolution

A view-only `resolvedFromServer` attribute records whether a title came from a
mentions map:

- **no map** → keep what the node holds (nothing contradicts it)
- **map, ID absent, `resolvedFromServer` true** → clear to the bare ID
- **map, ID absent, `resolvedFromServer` false** → keep, because this is the
just-inserted reference the picker supplied and the server has not yet had a
chance to weigh in

Two tests pin it: one revokes a grant after resolution and asserts the title
clears, one passes no map and asserts it does not.
