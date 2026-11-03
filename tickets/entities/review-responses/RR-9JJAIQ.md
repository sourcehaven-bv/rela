---
id: RR-9JJAIQ
type: review-response
title: 'Transition role check: capability and degraded config are not specified'
finding: 'The plan says "the poster holds one of its roles". It should say call.can(role, CapabilityUpdate), as mayPost does, so the client ceiling applies the same way to post: and to transition roles. Under a degraded snapshot, call.can refuses every entity-property role (lead: from entity.owner). The post is then accepted and the session silently does not move, but the HTTP post answer (201 with the message only) does not show that. Either add the session to the HTTP post answer, as PostResult/MCP get it, or write down that HTTP clients learn it from the head over SSE. The CLI channel post/show renderer (internal/cli/channel_render.go) is also not mentioned: include it or defer it explicitly.'
severity: minor
resolution: 'Plan updated: the role check uses CapabilityUpdate; degraded mode is documented and tested; the CLI header and post show the session.'
status: addressed
---
