---
id: RR-YJ0UY3
type: review-response
title: Piles wiring duplicated in NewFromCollaborators
finding: collaboratorPilesAndAliases repeats assembly steps; recorders hide the piles capability.
severity: minor
resolution: One piles helper is shared by assembly and NewFromCollaborators. Recorders expose Unwrap so the postgres piles and state lookups see through a wrapped store.
status: addressed
---
