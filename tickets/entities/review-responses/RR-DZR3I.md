---
id: RR-DZR3I
type: review-response
title: 'POST create response shape changed: relations now echoed'
finding: Flipped entityToV1(created, plural, true, false) — POST responses now include the relations key. Consistent with PATCH. Any external API consumer (MCP, Lua, third-party) that relied on the old shape sees more data. Grep before merging; document in PR body.
severity: minor
resolution: Grepped the codebase for existing callers of the POST response. The frontend's entitiesStore consumes `relations` when present; no other in-tree consumer asserts the old empty-relations shape. Documenting the change in the PR body. Shipping the flip together with the PATCH consistency; any external consumer with a strict schema will see additional keys (a fair v1 evolution).
status: addressed
---
