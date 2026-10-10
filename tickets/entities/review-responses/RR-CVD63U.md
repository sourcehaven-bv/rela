---
id: RR-CVD63U
type: review-response
title: Removing a choice is a deletion path
finding: enum_values_removed is drift; Persist would ledger it and GC deletes values after grace; a named type needs one map_values per property using it; relation properties have no map_values.
severity: significant
resolution: 'Plan changed: the save never persists drift; a removed option expands to a map_values step for every entity property of that type; removal is refused when a relation property uses the type.'
status: addressed
---
