---
id: RR-4LHM6
type: review-response
title: 'Title field name: title vs _title'
finding: 'The plan says we''ll display each result''s title and fall back to ID. SearchView.vue:155 reads entity.properties.title. The Explore-agent survey says V1Entity JSON uses _title. These cannot both be right. If the plan picks the wrong field, every result row falls back to the ID for entities whose title is in fact populated. Required: read frontend/src/types/ Entity definition and a sample API response to confirm the canonical field. If both can occur, document an explicit fallback chain (entity._title ?? entity.properties.title ?? entity.id). Update the Approach section''s ''Render results'' bullet with the verified field name.'
severity: significant
resolution: 'Verified frontend/src/types/entity.ts:4 declares `_title?: string` as the server-populated display title (metamodel-aware). EntityList.vue:469 uses the precedence `_title || properties.title || id`, which is the canonical fallback chain in the codebase. Plan updated: render uses `entity._title || entity.properties?.title || entity.id`.'
status: addressed
---
