---
id: RR-WJHM8I
type: review-response
title: System writes keep growing `realtime.Service`
finding: '`systemPost`, `systemTransition`, `applyTransition` and `recordType` add to a type that T4 also extends. A small unexported system-writer type over the store, Local and record would keep Service focused.'
severity: nit
resolution: System writes live in an unexported systemWriter type.
status: addressed
---
