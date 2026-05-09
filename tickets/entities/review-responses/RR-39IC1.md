---
id: RR-39IC1
type: review-response
title: Empty types={} returns empty map silently — undocumented
finding: parseEntityRefsOpts treats opts.types={} as 'use this empty list' (useAllTypes=false), so result is empty map. Different from omitting types (which means 'all types'). Could be intentional but is undocumented and surprising.
severity: minor
resolution: 'Documented in docs/lua-scripting.md: ''an *empty* list ({}) returns an empty map; omit `types` to mean "all types".'' Added test ''RR-39IC1 empty types list returns empty map'' verifying the behavior.'
status: addressed
---

# Finding

`parseEntityRefsOpts` sets `useAllTypes=false` whenever `opts.types` is present,
regardless of length. So `entity_refs({types={}})` returns an empty map, while
`entity_refs()` returns all types.

# Resolution

Document explicitly: "An empty `types` list returns an empty map. Omit `types`
(or pass `nil`) to mean 'all types'."

Add a test pinning the behavior.
