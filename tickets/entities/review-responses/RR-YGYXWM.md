---
id: RR-YGYXWM
type: review-response
title: search_entities ranking moves on hidden writes
finding: 'Security and architecture design review: relevance scoring uses index-wide term statistics that include hidden documents, so a hidden write can reorder visible hits, change the digest and end a wait. That breaks the plan''s claim that hidden writes never end a wait.'
severity: significant
resolution: search_entities is left out of the target allowlist; agents watch list_entities with a filter. The exclusion and its reason are in the plan and go in docs/acl-security.md.
status: addressed
---
