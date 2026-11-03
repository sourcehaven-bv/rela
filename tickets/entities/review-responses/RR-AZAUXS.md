---
id: RR-AZAUXS
type: review-response
title: listPage primed the whole population without a view condition
finding: listPage called primeVerdicts over every matched row before applyViewCondition, even when no condition would judge them. A large list paid a population-sized prime per request.
severity: significant
resolution: The prime runs only when cond != nil; the served page is primed later as before. Goldens updated; only read counts changed.
status: addressed
---
