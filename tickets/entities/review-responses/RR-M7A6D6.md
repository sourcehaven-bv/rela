---
id: RR-M7A6D6
type: review-response
title: Id reuse protection fails open after commit
finding: 'EntityCreated/rename Tombstone hooks run after commit on the request ctx and only log errors. A cancelled ctx or store error leaves a deleted entity''s channels attached to a new entity reusing the id (client-chosen id on create, rename target). Concurrent face create can also skip the entity-level tombstone. Fix: stamp channels with the anchor''s creation identity and treat a mismatch as nonexistent; run hooks on WithoutCancel with a timeout.'
severity: significant
resolution: Channels record an Incarnation token at creation; CheckIncarnation maps a mismatch to ErrNotFound and Append fails with ErrStaleAnchor; alias hooks run on WithoutCancel with a 30s timeout; conformance test Incarnation (64271197). T3 must supply the token.
status: addressed
---
