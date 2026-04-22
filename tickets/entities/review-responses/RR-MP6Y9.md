---
id: RR-MP6Y9
type: review-response
title: Reserved return_to key scope undocumented for foldPrefixed
finding: internal/lua/urls.go:412-444 mergeParamsTable rejects return_to as reserved. foldPrefixed does not reject return_to in properties=/relations= sub-tables (they'd become prop.return_to / rel.return_to, different keys). Probably fine but worth a line in the doc comment so the scope of the reservation is explicit.
severity: nit
resolution: Added an explanatory paragraph to mergeParamsTable's doc comment noting that foldPrefixed intentionally doesn't reject return_to (under relations=/properties= it becomes rel.return_to / prop.return_to, different keys that don't collide with the reserved top-level). Scope of the reservation is now explicit.
status: addressed
---
