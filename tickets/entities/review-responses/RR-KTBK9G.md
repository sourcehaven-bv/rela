---
id: RR-KTBK9G
type: review-response
title: Incoming-edge source resolution could under-redact on mid-request peer delete
finding: 'For incoming edges, relationSourceEntity falls back to the wrong-type path (TO-side) entity when the peer can''t be fetched. If a peer is deleted between the neighbor-visibility pass and the meta read, the fallback path entity''s type likely has no visible: block for that relation → meta emitted un-redacted. Astronomically narrow (delete landing mid-request) but a genuine under-redaction; a redaction consumer should fail closed, unlike the write path where the fallback is benign.'
severity: minor
resolution: Added visibleRelationMetaIncoming which resolves the incoming source via getEntity(peerID) and FAILS CLOSED (returns empty meta) when the peer is gone, only when the resolver can redact at all. Both incoming serialization sites route through it. Pinned by TestVisibleRelationMetaIncoming_SourceGone_FailsClosed.
status: addressed
---
