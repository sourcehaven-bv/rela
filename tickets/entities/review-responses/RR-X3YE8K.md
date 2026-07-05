---
id: RR-X3YE8K
type: review-response
title: export relation-target titles would echo ID for titleless entities
finding: 'Wiring DisplayTitle into export.go relation targets changed the JSON/YAML semantics: RelationTarget.Title is `omitempty` and was empty for titleless entities; DisplayTitle falls back to the entity ID, so absent titles would serialize as title==id, breaking the ''no title'' signal for machine consumers.'
severity: significant
resolution: Reverted export.go to use node.Title() (raw property). Export is a data-interchange format; presentation-layer display_property resolution belongs only in human-facing output. Narrowed the ticket's AC4 to graph (visualization) and documented export-stays-raw as a deliberate exclusion. Added an explaining comment at both export call sites.
status: addressed
---
