---
id: RR-NZMJEY
type: review-response
title: MCP says entity-not-found for a visible entity whose type the channel does not attach to
finding: 'With EntityType "" (section 3.2), channelType passes and resolveAnchor returns ErrAnchorNotFound when view.Type is not in AttachTo (authz.go resolveAnchor). Section 3.5 then maps that to show_entity''s "entity not found" for an entity show_entity serves. The section 5 row "outside attach_to → unknown_channel_type" is wrong for MCP and CLI. Fix: after a served anchor read, answer unknown_channel_type naming the entity type; this leaks nothing because the read was gated.'
severity: significant
resolution: After a served (gated) anchor read, a type outside attach_to answers unknown_channel_type naming the entity type. The section 5 table is corrected.
status: addressed
---
