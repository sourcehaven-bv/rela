---
id: RR-6PGS
type: review-response
title: Validation divergence risk between Lua and Go
finding: The Lua rrule_next and the new metamodel validation will independently implement the same RRULE validation logic (prefix stripping, parse, INTERVAL/DTSTART check). Extract a shared ValidateRrule function to prevent divergence.
severity: significant
resolution: Will extract shared ValidateRrule() function in internal/metamodel/. Both Lua helper and metamodel validator will call it.
status: addressed
---
