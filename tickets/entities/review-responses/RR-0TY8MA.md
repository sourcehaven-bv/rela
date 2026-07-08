---
id: RR-0TY8MA
type: review-response
title: Collided-title option shows an arbitrary '(ID)' that is misleading
finding: EntityTargetSelect.vue sortedOptions dedup keeps the FIRST candidate's label for a shared title, so a collapsed option shows e.g. 'Marc (PERS-A)' purely by fetch order, while the filter actually matches BOTH Marcs. Showing one specific ID on an option that intentionally matches multiple IDs is misleading. For collided titles, show the bare title (drop the ID) or append a count.
severity: minor
resolution: 'EntityTargetSelect.vue sortedOptions: a first pass counts candidates per title; a collapsed option shows ''Title (ID)'' only when the title is unique, and the bare title when it''s shared (so no arbitrary id is pinned to an option that matches multiple entities). Tested: ''deduplicates candidates that share a display title and drops the ambiguous id''.'
status: addressed
---
