---
id: RR-OUO80G
type: review-response
title: archplan authz guard lost on entity GET and views gate
finding: handleV1GetEntity replaced readAddressedOr404 (annotated //archplan:guard=authz) with an inline address read, and handleV1Views inlined the PermitsRead gate instead of calling gateReadOrNotFound. Both routes lost their guard annotation, so the architecture plan no longer sees the authz chokepoint on them.
severity: significant
resolution: Added readAddressedOr and gateReadOr, guard-annotated helpers that take the miss answer from the caller. readAddressedOr404 delegates to readAddressedOr; the entity GET uses readAddressedOr with a miss that tries the redirect and otherwise writes the uniform 404. The views route gates through gateReadOr via viewsHandler.gateEntry. The now-unused gateReadOrNotFound was removed and its doc merged into gateReadOr.
status: addressed
---
