---
id: RR-UAI3XDB
type: review-response
title: A stray tenants.yaml switches the server mode
finding: Tenant mode turns on because the file exists.
severity: minor
reason: The file's presence is the documented switch in the ticket design. On a non-postgres build the server refuses to start rather than serve wrong data, and the chosen mode is logged at startup (tenant mode enabled).
status: wont-fix
---
