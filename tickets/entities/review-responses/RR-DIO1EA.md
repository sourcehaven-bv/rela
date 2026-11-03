---
id: RR-DIO1EA
type: review-response
title: Adapter repeats property lookup
finding: GetPropertyType, IsPropertyRequired and GetPropertyDefault each repeat the entity and property lookup.
severity: nit
resolution: Extracted migrationSchema.property (zero PropertyDef when absent); each method is now one line.
status: addressed
---
