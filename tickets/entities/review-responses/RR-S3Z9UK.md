---
id: RR-S3Z9UK
type: review-response
title: Guide overstates the deleted-row rule
finding: The Lua guide said a deleted row needs read access to the type and face but only the face restriction is checked as in the HTTP history endpoint. Version 0 was not documented.
severity: minor
resolution: Guide now states the history:read permission plus the face rule and why conditions cannot be checked. Version 0 is documented.
status: addressed
---
