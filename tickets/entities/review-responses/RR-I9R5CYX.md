---
id: RR-I9R5CYX
type: review-response
title: Relation page fill thins on a gate fault
finding: 'ListRelationsPage used the tolerant FilterRelations. A persistent gate fault hid every raw page so the loop scanned the whole key range and returned an empty list with no error: the short list TKT-FVQ4 forbids.'
severity: significant
resolution: The fill uses FilterRelationsStrict and returns its error. TestScriptReader_ListRelationsPageReturnsGateFaults pins it.
status: addressed
---
