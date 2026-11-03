---
id: RR-K6ER8S
type: review-response
title: Holder lookup streams full entities, bodies included, up to 20 001 per pair
finding: 'internal/visibility/holders.go:222 (pageHolders) calls gq.GraphQuery with Limit = MaxWaitingAnchors+1 = 20 001 (internal/realtime/inbox.go:243). GraphQuery returns whole entities with their markdown content, but the lookup needs only id, face and one property. A job identity that owns a few thousand entities makes every inbox call, and every list_inbox watch replay (each in-process channel wake, each store event, every 15 s), pull megabytes of bodies per pair. The budget test counts calls, not bytes, so it cannot catch this. Fix: use store.GraphQueryHeaders (content-free; the same package already has RedactHeader for headers) and test namesIdentity on the redacted header''s Properties. Apply the same change to the unrestricted path at holders.go:176.'
severity: significant
resolution: The holder lookup reads headers only (GraphQueryHeaders) and redacts each row with RedactHeaderRow; TestHolderLookup_ReadsNoBodies pins it
status: addressed
---
