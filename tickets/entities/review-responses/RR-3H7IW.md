---
id: RR-3H7IW
type: review-response
title: no updates specified guard would reject delete-only calls if helper collapses to nil
finding: 'The plan says introduce extractPropertiesAllowNil and switch handleUpdateEntity to it. But internal/mcp/tools_helpers.go:88-90 returns nil when the filtered map is empty. If the new variant copies that pattern and a client sends only {"foo": null}, the new map would have one entry (the nil) so this is fine — BUT the plan does not call out this trap explicitly, and a slightly different implementation (e.g., ''collapse to nil if no NON-NIL entries'') would silently kill the delete-only path because tools_entity.go:208 checks len(properties) == 0 and returns ''no updates specified''. The plan must explicitly say: extractPropertiesAllowNil keeps nil entries in the returned map AND counts them toward len() — so delete-only calls survive the ''no updates specified'' guard. Add a test for the delete-only call shape: {"properties": {"foo": null}} on an entity with foo=bar → entity has foo removed (this is AC 1 but as a delete-only call, not mixed with other set ops).'
severity: critical
resolution: 'Plan updated: extractPropertiesAllowNil keeps nil entries in the returned map and counts them toward len(); the ''no updates specified'' guard at tools_entity.go:208 therefore allows delete-only calls. Added explicit test TestHandleUpdateEntity_DeleteOnlyCallSurvivesGuard for `{"foo": null}` as the sole property argument.'
status: addressed
---
