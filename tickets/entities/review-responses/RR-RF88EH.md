---
id: RR-RF88EH
type: review-response
title: feedProvider.Get doesn't verify loaded entity's type matches the UID
finding: 'feed_provider.go:153 calls d.src.getEntity(ctx, entityType, id), but production getVisible (visiblereader.go:65) looks up by ID ALONE (store.GetEntity(ctx, id)); entityType is used only for the ACL gate, not the lookup. Get then maps the returned entity without checking e.Type == entityType. The entity handlers (api_v1.go:654, :752) all guard entity.Type != typeName precisely because a by-id lookup can return a different type. If two types share an id, a CalDAV Get maps the wrong entity and applies the wrong source''s filter/mapping. Latent (Get unrouted) but the same-shape bug the codebase defends against elsewhere. Fix: add the type check after getEntity.'
severity: significant
resolution: 'Get now rejects a type mismatch: after getEntity, `if !found || e.Type != entityType` returns not-found, so a by-id lookup that resolves a different type can''t be mapped under the wrong source''s rules. Pinned by TestDeclarativeFeed_GetRejectsTypeMismatch using a byIDSource that mimics the production by-id reader.'
status: addressed
---
