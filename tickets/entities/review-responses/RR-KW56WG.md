---
id: RR-KW56WG
type: review-response
title: Dispatcher comment implies its gate covers resolve
finding: The handleV1ConflictRoutes comment said both routes are gated before the path is resolved, but resolve returns before that gate and is gated in its own handler.
severity: minor
resolution: The comment and archplan reason now say the detail is gated there and resolve in writeHandler.handleV1ConflictResolve.
status: addressed
---
