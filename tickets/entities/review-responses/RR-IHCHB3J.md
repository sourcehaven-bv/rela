---
id: RR-IHCHB3J
type: review-response
title: 'Design: relation range with no endpoint types is silently empty'
finding: A relation range gates endpoints with the named entity types; with none named it answers count 0 and a client concludes it is in sync.
severity: critical
resolution: A relation range must name at least one entity type and one relation type; otherwise 400. The docs state that a relation is in range only when both endpoints are of named types (in scope).
status: addressed
---
