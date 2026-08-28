---
id: RR-PP3B60
type: review-response
title: 'create-without-read: plan cites the rule but never handles the post-create consequence'
finding: The plan cites "create implies no read" (policy.go:239-243) only to reject deriving permission from read visibility, never asking what happens after a create by a principal who cannot read the type. RelationCards re-hydrates uncached entries via getEntity (RelationCards.vue:148-160); after the parent saves and remounts, that GET 404s under the row-level hidden-entity rule, and Promise.allSettled swallows it — leaving the card rendering a bare id or blank with no error. The plan also never states whether POST returns a readable body to such a principal. This breaks the exact submitter role used to justify the design.
severity: significant
resolution: 'Verified POST returns the created entity via forWire (write_handler.go:178), so both widgets seed their caches from it and in-session behaviour is correct. The gap is re-hydration after remount, where a 404 is swallowed by Promise.allSettled. Accepted but pinned as AC11: the card renders the entity id, never blank and never an error toast. Not a leak — the id is one the principal just authored.'
status: addressed
---

## Resolution

Verified: `POST /api/v1/{plural}` returns the created entity via
`h.serializer.forWire` (`write_handler.go:178`), so the created object *is* in
hand client-side and both widgets seed their caches from it —
`RelationPicker.handleEntityCreated` pushes to `candidates`, `RelationCards`
sets `entityCache`. In-session behaviour is therefore correct.

The gap is re-hydration after remount: `RelationCards.vue:148-160` GETs uncached
entries and `Promise.allSettled` silently swallows a 404, so an
unreadable-but-linked entity degrades to a bare id.

Resolution: accept the degradation, but make it **intentional and pinned**
rather than accidental. Added AC11: a principal who may create but not read a
type can create + link it in-session, and after re-fetch fails the card renders
the entity id (never blank, never an error toast) — matching how a hidden
neighbour already renders elsewhere. Test at the Vitest level by stubbing
`getEntity` to reject with 404.

Explicitly noted: this is not a leak — the id is one the principal itself just
authored and linked.
