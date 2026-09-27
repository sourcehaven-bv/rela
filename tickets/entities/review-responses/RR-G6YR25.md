---
id: RR-G6YR25
type: review-response
title: '[security] Face-scoped delete grant can delete the whole family'
finding: '[security] DeleteEntity authorizes one face and deletes all faces; now reachable via MCP delete_entity.'
severity: minor
reason: Outside this diff and already reachable via web DELETE and Lua; filed as BUG-6SCBG3.
status: deferred
---
