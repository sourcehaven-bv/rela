---
id: RR-L0WVWGV
type: review-response
title: 'Design: compile cost grows with type names per range'
finding: Each range compiles ReadQueryFor and SearchFieldScope per named type; 32 ranges naming every type multiplies that.
severity: significant
resolution: The total of type names over all ranges is capped per request (256); together with the slot-time bound this caps compile work.
status: addressed
---
