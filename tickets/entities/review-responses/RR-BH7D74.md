---
id: RR-BH7D74
type: review-response
title: getAllEntityRelations does not encode the id
finding: The id goes into the path raw.
severity: nit
resolution: Not changed; see reason.
reason: Every entity-path helper in api/entities.ts builds the path the same way; encoding this one alone would make it the odd one out. An address contains only id characters and '@'; both are path-safe.
status: wont-fix
---
