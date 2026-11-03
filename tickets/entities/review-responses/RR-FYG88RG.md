---
id: RR-FYG88RG
type: review-response
title: e2e position check too weak
finding: The e2e test only checked that the first view section is above the panel and the last below it, which also passes for wrong positions between sections.
severity: minor
resolution: 'The test now asserts the panel''s neighbours: the section before it holds the entity body and the section after it holds relation rows.'
status: addressed
---
