---
id: RR-Q919WY
type: review-response
title: bareref guard misses elided composite literals
finding: '[]entity.Ref{{ID: id}} and map values with elided types were not caught.'
severity: minor
resolution: bareRefs checks elided elements of slice/array/map literals of Ref; test cases added.
status: addressed
---
