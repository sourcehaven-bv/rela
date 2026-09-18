---
id: RR-RTNTSR
type: review-response
title: budgetEpics collapses the parent level below n=5
finding: budgetEpics(n) = (n+4)/5 yields 1 for n<5 and 0 for n=0, which would give the program a single epic or none - collapsing the parent level the nested pin exists to measure. readsFor only drives {10, 50} so this is fine today, but a future third size below 5 would silently stop measuring the parent dimension while still passing.
severity: minor
resolution: 'Documented on budgetEpics: the doc now states the {10, 50} sizes give 2 and 10 epics, why that matters, and that a third size must stay above budgetTicketsPerEpic or the parent dimension stops being measured.'
status: addressed
---
