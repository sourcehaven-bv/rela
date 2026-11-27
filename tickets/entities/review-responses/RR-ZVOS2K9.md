---
id: RR-ZVOS2K9
type: review-response
title: Repeated relation filter values are dropped silently
finding: filter[rel]=A&filter[rel]=B keeps only B on both paths. With ids it reads as an OR that is not honoured; a 400 would be honest.
severity: nit
resolution: 'Two different values for one key return a 400; the same value twice is one value. Test: TestRelationFilter_RepeatedValues.'
status: addressed
---
