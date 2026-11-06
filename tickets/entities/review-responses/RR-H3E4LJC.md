---
id: RR-H3E4LJC
type: review-response
title: Reserved prefix misses namespaced local names
finding: isms._run passed the reserved-name check.
severity: minor
resolution: 'IsReservedName also checks the local part after a namespace. Test: TestIsReservedName.'
status: addressed
---
