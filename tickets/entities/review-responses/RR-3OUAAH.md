---
id: RR-3OUAAH
type: review-response
title: ActionResult has no JSON tags
finding: Encoding realtime.ActionResult directly gives Message/Warnings keys, unlike the HTTP and MCP wire shapes.
severity: minor
resolution: Plan uses a tagged struct with notice, message and warnings.
status: addressed
---
