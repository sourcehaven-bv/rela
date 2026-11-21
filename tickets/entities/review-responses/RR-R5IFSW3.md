---
id: RR-R5IFSW3
type: review-response
title: "Header gap e2e only checks one direction"
finding: "toBeLessThanOrEqual(2) also passes when the table slides under the header."
severity: minor
resolution: "Addressed: the test also asserts the gap is at least -1."
status: addressed
---
