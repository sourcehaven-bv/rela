---
id: RR-TR9PTMP
type: review-response
title: Metamodel copy must not share maps
finding: A struct copy shares Entities and Relations; adding keys mutates the published original. Revalidation rejects the reserved names.
severity: significant
resolution: 'Plan changed: WithFlowRuns clones the maps it writes; validateReservedNames exempts exactly the managed definitions rela built (compared by name and value); the id prefix is checked against operator prefixes.'
status: addressed
---
