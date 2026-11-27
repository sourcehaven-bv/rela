---
id: RR-5BP241I
type: review-response
title: Target lookup runs before the operator check
finding: A store fault on filter[rel][contains]=x gives 500 where the request is a 400. Skip unsupported operators in relationFilterValues.
severity: nit
resolution: 'Operators are filtered before the lookup. Test: TestRelationFilter_BadOperatorBeforeLookup.'
status: addressed
---
